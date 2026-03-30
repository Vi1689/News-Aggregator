// cmd/streams/main.go
//
// Kafka Streams — News Aggregator (Go)
// ======================================
// Go does not have a native Kafka Streams library, so we implement
// the same topology using sarama consumer + producer with in-memory state.
//
// Three processing stages run as separate goroutines:
//
//   1. TRANSFORM  news.posts.created → news.posts.normalized
//      Normalises the post title (lowercase, clean punctuation),
//      infers a simple sentiment label, and adds a word-count field.
//
//   2. AGGREGATE  news.posts.created → news.tags.aggregated
//      Maintains a running per-tag post count (KTable equivalent).
//      Publishes an update event every time a tag count changes.
//
//   3. WINDOW     news.tags.trending → news.tags.trending.hourly
//      Tumbling window: 1-hour buckets keyed by "YYYY-MM-DD:HH".
//      Every 5 received events the top-10 tags of the current window
//      are published to the hourly output topic.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/IBM/sarama"
	"github.com/newsaggregator/kafka/internal/config"
	"github.com/newsaggregator/kafka/internal/events"
)

// ---------------------------------------------------------------------------
// Shared Kafka producer
// ---------------------------------------------------------------------------

func newProducer() sarama.SyncProducer {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Return.Successes = true
	cfg.Producer.Retry.Max = 3
	p, err := sarama.NewSyncProducer([]string{config.KafkaBootstrap}, cfg)
	if err != nil {
		log.Fatalf("[STREAMS] producer: %v", err)
	}
	return p
}

func sendJSON(p sarama.SyncProducer, topic, key string, payload any) {
	body, _ := json.Marshal(payload)
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(body),
	}
	if _, _, err := p.SendMessage(msg); err != nil {
		log.Printf("[STREAMS] send to %s failed: %v", topic, err)
	}
}

// ---------------------------------------------------------------------------
// Shared consumer factory
// ---------------------------------------------------------------------------

func newConsumer(group, topic string) sarama.ConsumerGroup {
	cfg := sarama.NewConfig()
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Consumer.Offsets.AutoCommit.Enable = true
	cfg.Consumer.Offsets.AutoCommit.Interval = 3 * time.Second
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	g, err := sarama.NewConsumerGroup([]string{config.KafkaBootstrap}, group, cfg)
	if err != nil {
		log.Fatalf("[STREAMS] consumer group %s: %v", group, err)
	}
	return g
}

// ---------------------------------------------------------------------------
// 1. TRANSFORM: title normalisation + sentiment
// ---------------------------------------------------------------------------

var nonWordRe = regexp.MustCompile(`[^\p{L}\p{N}\s\-.,!?]`)
var multiSpaceRe = regexp.MustCompile(`\s+`)

func normalizeTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	title = nonWordRe.ReplaceAllString(title, "")
	title = multiSpaceRe.ReplaceAllString(title, " ")
	if len([]rune(title)) > 200 {
		runes := []rune(title)
		title = string(runes[:200])
	}
	return title
}

var positiveWords = map[string]bool{"победил": true, "рекорд": true, "рост": true, "прорыв": true, "успех": true, "лучший": true}
var negativeWords = map[string]bool{"трагедия": true, "катастрофа": true, "упал": true, "кризис": true, "скандал": true, "провал": true}

func detectSentiment(title string) string {
	for _, w := range strings.FieldsFunc(title, unicode.IsSpace) {
		if positiveWords[w] {
			return "positive"
		}
		if negativeWords[w] {
			return "negative"
		}
	}
	return "neutral"
}

func wordCount(s string) int {
	return len(strings.Fields(s))
}

type transformHandler struct{ prod sarama.SyncProducer }

func (h *transformHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *transformHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *transformHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		event, err := events.FromJSON(msg.Value)
		if err != nil {
			log.Printf("[TRANSFORM] deserialise: %v", err)
			session.MarkMessage(msg, "")
			continue
		}

		title := event.Payload["title"]
		normalized := normalizeTitle(title)
		sentiment := detectSentiment(normalized)

		// Build enriched payload
		enrichedPayload := make(map[string]string, len(event.Payload)+3)
		for k, v := range event.Payload {
			enrichedPayload[k] = v
		}
		enrichedPayload["titleNormalized"] = normalized
		enrichedPayload["sentiment"] = sentiment
		enrichedPayload["wordCount"] = strconv.Itoa(wordCount(title))

		outEvent := events.NewsEvent{
			EventID:   event.EventID,
			EventType: "POST_NORMALIZED",
			EntityID:  event.EntityID,
			Timestamp: time.Now().UnixMilli(),
			Source:    "go-streams/transform",
			Version:   "1.0",
			Metadata:  event.Metadata,
			Payload:   enrichedPayload,
		}

		sendJSON(h.prod, config.TopicPostsNormalized, string(msg.Key), outEvent)
		log.Printf("[TRANSFORM] post_id=%s sentiment=%s title=%q",
			event.Payload["postId"], sentiment, normalized[:min(50, len(normalized))])

		session.MarkMessage(msg, "")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 2. AGGREGATE: running tag post count (KTable equivalent)
// ---------------------------------------------------------------------------

type aggregateHandler struct {
	prod    sarama.SyncProducer
	mu      sync.Mutex
	tagCount map[string]int // in-memory state store
}

func (h *aggregateHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *aggregateHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *aggregateHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		event, err := events.FromJSON(msg.Value)
		if err != nil {
			session.MarkMessage(msg, "")
			continue
		}

		tagsStr := event.Payload["tags"]
		if tagsStr == "" {
			session.MarkMessage(msg, "")
			continue
		}

		for _, tag := range strings.Split(tagsStr, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}

			h.mu.Lock()
			h.tagCount[tag]++
			count := h.tagCount[tag]
			h.mu.Unlock()

			out := map[string]any{
				"eventType":  "TAG_COUNT_UPDATED",
				"tagName":    tag,
				"totalPosts": count,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
				"source":     "go-streams/aggregate",
			}
			sendJSON(h.prod, config.TopicTagsAggregated, "tag-"+tag, out)
			log.Printf("[AGGREGATE] tag=%q total_posts=%d", tag, count)
		}

		session.MarkMessage(msg, "")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 3. WINDOW: tumbling 1-hour window for trending tags
// ---------------------------------------------------------------------------

type tagWindowEntry struct {
	Score   float64
	Posts   int
	Updates int
}

type windowHandler struct {
	prod         sarama.SyncProducer
	mu           sync.Mutex
	windowState  map[string]map[string]*tagWindowEntry // windowKey → tag → entry
	eventCounter int
}

func currentWindowKey() string {
	now := time.Now().UTC()
	return fmt.Sprintf("%s:%02d", now.Format("2006-01-02"), now.Hour())
}

type tagScore struct {
	Tag     string          `json:"tag"`
	Score   float64         `json:"score"`
	Posts   int             `json:"posts"`
	Updates int             `json:"updates"`
}

func (h *windowHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *windowHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *windowHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		event, err := events.FromJSON(msg.Value)
		if err != nil {
			session.MarkMessage(msg, "")
			continue
		}

		tagName := event.Payload["tagName"]
		score, _ := strconv.ParseFloat(event.Payload["trendScore"], 64)
		posts, _ := strconv.Atoi(event.Payload["postCount"])

		if tagName == "" {
			session.MarkMessage(msg, "")
			continue
		}

		wk := currentWindowKey()

		h.mu.Lock()
		if h.windowState[wk] == nil {
			h.windowState[wk] = make(map[string]*tagWindowEntry)
		}
		if h.windowState[wk][tagName] == nil {
			h.windowState[wk][tagName] = &tagWindowEntry{}
		}
		entry := h.windowState[wk][tagName]
		entry.Score += score
		entry.Posts += posts
		entry.Updates++
		h.eventCounter++
		counter := h.eventCounter

		// Snapshot for publishing (avoid holding lock during I/O)
		snapshot := make([]tagScore, 0, len(h.windowState[wk]))
		for t, e := range h.windowState[wk] {
			snapshot = append(snapshot, tagScore{Tag: t, Score: e.Score, Posts: e.Posts, Updates: e.Updates})
		}
		h.mu.Unlock()

		log.Printf("[WINDOW %s] tag=%q window_score=%.4f", wk, tagName, entry.Score)

		// Publish top-10 every 5 events
		if counter%5 == 0 {
			sort.Slice(snapshot, func(i, j int) bool {
				return snapshot[i].Score > snapshot[j].Score
			})
			top10 := snapshot
			if len(top10) > 10 {
				top10 = top10[:10]
			}

			out := map[string]any{
				"eventType":   "HOURLY_TOP_TAGS",
				"windowKey":   wk,
				"windowStart": wk + ":00:00Z",
				"windowEnd":   wk + ":59:59Z",
				"topTags":     top10,
				"totalEvents": counter,
				"computedAt":  time.Now().UTC().Format(time.RFC3339),
				"source":      "go-streams/window",
			}
			sendJSON(h.prod, config.TopicTagsTrendingHourly, wk, out)
			log.Printf("[WINDOW] Published top-%d tags for window=%s", len(top10), wk)
		}

		session.MarkMessage(msg, "")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runGroup(ctx context.Context, wg *sync.WaitGroup, group sarama.ConsumerGroup, topic string, handler sarama.ConsumerGroupHandler, label string) {
	defer wg.Done()
	for {
		if err := group.Consume(ctx, []string{topic}, handler); err != nil {
			log.Printf("[%s] Consume error: %v", label, err)
		}
		if ctx.Err() != nil {
			log.Printf("[%s] Shutting down.", label)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// main — starts all three stream processors concurrently
// ---------------------------------------------------------------------------

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	prod := newProducer()
	defer prod.Close()

	var wg sync.WaitGroup

	// 1. Transform processor
	wg.Add(1)
	go func() {
		g := newConsumer("streams-transform", config.TopicPostsCreated)
		defer g.Close()
		runGroup(ctx, &wg, g, config.TopicPostsCreated, &transformHandler{prod: prod}, "TRANSFORM")
	}()

	// 2. Aggregate processor
	wg.Add(1)
	go func() {
		g := newConsumer("streams-aggregate", config.TopicPostsCreated)
		defer g.Close()
		runGroup(ctx, &wg, g, config.TopicPostsCreated, &aggregateHandler{
			prod:     prod,
			tagCount: make(map[string]int),
		}, "AGGREGATE")
	}()

	// 3. Windowed processor
	wg.Add(1)
	go func() {
		g := newConsumer("streams-window", config.TopicTagsTrending)
		defer g.Close()
		runGroup(ctx, &wg, g, config.TopicTagsTrending, &windowHandler{
			prod:        prod,
			windowState: make(map[string]map[string]*tagWindowEntry),
		}, "WINDOW")
	}()

	_ = math.Pi // suppress unused import
	log.Println("[STREAMS] All processors started. Ctrl+C to stop.")
	wg.Wait()
}
