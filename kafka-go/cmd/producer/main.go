// cmd/producer/main.go
//
// News Aggregator — Kafka Producer (Go / sarama)
// ================================================
// Publishes three event types:
//   POST_CREATED   → key: channel-{id}
//   COMMENT_ADDED  → key: post-{id}
//   TAG_TRENDING   → key: tag-{name}
//
// Message format: JSON-serialised NewsEvent (see internal/events/event.go)
// Message key:    controls partitioning — same key → same partition → ordered delivery
// Headers:        eventType, source, version, schemaName (readable without deserialising body)

package main

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/newsaggregator/kafka/internal/config"
	"github.com/newsaggregator/kafka/internal/events"
)

// ---------------------------------------------------------------------------
// Producer setup
// ---------------------------------------------------------------------------

func newSyncProducer() sarama.SyncProducer {
	cfg := sarama.NewConfig()

	// Reliability: wait for all in-sync replicas to ack
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 5
	cfg.Producer.Retry.Backoff = 500 * time.Millisecond
	cfg.Producer.Return.Successes = true

	// Idempotent producer — exactly-once delivery at producer level
	cfg.Producer.Idempotent = true
	cfg.Net.MaxOpenRequests = 1 // required for idempotent mode

	// Message format
	cfg.Producer.Compression = sarama.CompressionSnappy

	p, err := sarama.NewSyncProducer([]string{config.KafkaBootstrap}, cfg)
	if err != nil {
		log.Fatalf("[PRODUCER] failed to create producer: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// publish sends a NewsEvent to Kafka with the correct key and headers.
// ---------------------------------------------------------------------------

func publish(p sarama.SyncProducer, topic string, event events.NewsEvent) {
	value, err := event.ToJSON()
	if err != nil {
		log.Printf("[PRODUCER] serialisation error: %v", err)
		return
	}

	key := events.MessageKey(event.EventType, event.EntityID)

	// Build Kafka record headers
	hdrs := events.KafkaHeaders(event)
	recordHeaders := make([]sarama.RecordHeader, 0, len(hdrs))
	for k, v := range hdrs {
		recordHeaders = append(recordHeaders, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	msg := &sarama.ProducerMessage{
		Topic:   topic,
		Key:     sarama.StringEncoder(key),   // ← message key
		Value:   sarama.ByteEncoder(value),
		Headers: recordHeaders,               // ← metadata in headers
	}

	partition, offset, err := p.SendMessage(msg)
	if err != nil {
		log.Printf("[PRODUCER] send error topic=%s: %v", topic, err)
		return
	}
	log.Printf("[PRODUCER] → %s | key=%s | partition=%d | offset=%d | eventId=%s",
		topic, key, partition, offset, event.EventID)
}

// ---------------------------------------------------------------------------
// Event builders — one function per event type
// ---------------------------------------------------------------------------

// publishPostCreated sends a POST_CREATED event.
// Key = channel-{channelID} so all posts from one channel go to one partition.
func publishPostCreated(p sarama.SyncProducer, postID, authorID, channelID int, title string, tags []string) {
	event := events.BuildEvent(
		events.EventTypePostCreated,
		strconv.Itoa(postID), // entityID = postID
		"go-producer",
		map[string]string{
			"postId":      strconv.Itoa(postID),
			"title":       title,
			"authorId":    strconv.Itoa(authorID),
			"channelId":   strconv.Itoa(channelID),
			"tags":        strings.Join(tags, ","),
			"contentHash": strconv.Itoa(int(hashString(title))),
		},
		events.WithChannelID(channelID),
	)
	publish(p, config.TopicPostsCreated, event)
}

// publishCommentAdded sends a COMMENT_ADDED event.
// Key = post-{postID} so all comments on one post go to one partition.
func publishCommentAdded(p sarama.SyncProducer, commentID, postID int, nickname, text string) {
	preview := text
	if len(preview) > 200 {
		preview = preview[:200]
	}
	event := events.BuildEvent(
		events.EventTypeCommentAdded,
		strconv.Itoa(commentID),
		"go-producer",
		map[string]string{
			"commentId":   strconv.Itoa(commentID),
			"postId":      strconv.Itoa(postID),
			"nickname":    nickname,
			"textPreview": preview,
		},
		events.WithUserID(nickname),
	)
	publish(p, config.TopicCommentsAdded, event)
}

// publishTagTrending sends a TAG_TRENDING event.
// Key = tag-{tagName} so trend updates for one tag stay ordered.
func publishTagTrending(p sarama.SyncProducer, tagName string, postCount int, trendScore float64) {
	event := events.BuildEvent(
		events.EventTypeTagTrending,
		tagName,
		"go-scheduler",
		map[string]string{
			"tagName":    tagName,
			"postCount":  strconv.Itoa(postCount),
			"trendScore": fmt.Sprintf("%.4f", trendScore),
		},
	)
	publish(p, config.TopicTagsTrending, event)
}

// ---------------------------------------------------------------------------
// Demo data
// ---------------------------------------------------------------------------

var samplePosts = []struct {
	title string
	tags  []string
}{
	{"Россия победила в хоккее", []string{"спорт", "хоккей", "новости"}},
	{"ИИ превзошёл человека в Go", []string{"технологии", "ии", "наука"}},
	{"Выборы: предварительные итоги", []string{"политика", "новости"}},
	{"Биткоин пробил $100k", []string{"финансы", "крипто"}},
	{"Новый iPhone 17 анонсирован", []string{"технологии", "apple"}},
}

var sampleComments = []string{
	"Отличная новость!",
	"Не верю этому источнику.",
	"Давно ждал.",
	"Интересно, что будет дальше?",
}

var sampleTags = []string{"технологии", "спорт", "политика", "финансы", "ии"}

func hashString(s string) uint32 {
	var h uint32 = 2166136261
	for _, c := range []byte(s) {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	log.Println("[PRODUCER] Starting news aggregator producer...")

	producer := newSyncProducer()
	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("[PRODUCER] close error: %v", err)
		}
	}()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Publish 5 posts
	for i, post := range samplePosts {
		postID := i + 1
		publishPostCreated(producer, postID, rng.Intn(5)+1, rng.Intn(3)+1, post.title, post.tags)
		time.Sleep(300 * time.Millisecond)
	}

	// Publish 5 comments
	for i := 1; i <= 5; i++ {
		publishCommentAdded(
			producer, i, rng.Intn(5)+1,
			fmt.Sprintf("user_%d", rng.Intn(100)+1),
			sampleComments[rng.Intn(len(sampleComments))],
		)
		time.Sleep(200 * time.Millisecond)
	}

	// Publish trending tags
	for _, tag := range sampleTags {
		publishTagTrending(producer, tag, rng.Intn(46)+5, rng.Float64())
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("[PRODUCER] All demo events published ✓")
}
