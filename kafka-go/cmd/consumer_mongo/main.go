// cmd/consumer_mongo/main.go
//
// Consumer Group 2: cg-mongo-enricher
// =====================================
// Enriches MongoDB with post content, interaction records, and tag stats.
// Handles all four event types: POST_CREATED, POST_UPDATED, COMMENT_ADDED, TAG_TRENDING.
//
// Commit strategy: AUTO — sarama commits offsets on a fixed interval (5 s).
//   → higher throughput; may reprocess a message after a crash (idempotent upserts handle it).
//
// Error handling:
//   - 2 retries with 500ms * attempt backoff
//   - After retries: forward to DLQ, continue (do not block the partition)

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/newsaggregator/kafka/internal/config"
	"github.com/newsaggregator/kafka/internal/events"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const maxRetries = 2

// ---------------------------------------------------------------------------
// MongoDB collections helper
// ---------------------------------------------------------------------------

type mongoDB struct {
	posts        *mongo.Collection
	interactions *mongo.Collection
	topPostsView *mongo.Collection
}

func newMongoDB(ctx context.Context) (*mongo.Client, mongoDB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return nil, mongoDB{}, err
	}
	db := client.Database("news_aggregator")
	return client, mongoDB{
		posts:        db.Collection("posts"),
		interactions: db.Collection("user_interactions"),
		topPostsView: db.Collection("top_posts_view"),
	}, nil
}

// ---------------------------------------------------------------------------
// ConsumerGroupHandler
// ---------------------------------------------------------------------------

type mongoHandler struct {
	mdb     mongoDB
	dlqProd sarama.SyncProducer
}

func (h *mongoHandler) Setup(_ sarama.ConsumerGroupSession) error {
	log.Println("[CG2] Session started")
	return nil
}

func (h *mongoHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	log.Println("[CG2] Session ended")
	return nil
}

// ConsumeClaim processes messages from a single partition.
// AUTO COMMIT: sarama commits offsets automatically every 5 seconds —
// session.MarkMessage is still required to advance the commit position.
func (h *mongoHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("[CG2] Received topic=%s partition=%d offset=%d key=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key))

		h.processWithRetry(session.Context(), msg)

		// Mark for the auto-committer — without this the offset never advances.
		session.MarkMessage(msg, "")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Retry loop
// ---------------------------------------------------------------------------

func (h *mongoHandler) processWithRetry(ctx context.Context, msg *sarama.ConsumerMessage) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if lastErr = h.dispatch(ctx, msg); lastErr == nil {
			return
		}
		log.Printf("[CG2] Attempt %d/%d failed: %v", attempt, maxRetries, lastErr)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	h.sendToDLQ(msg, lastErr)
}

// ---------------------------------------------------------------------------
// Dispatch by event type
// ---------------------------------------------------------------------------

func (h *mongoHandler) dispatch(ctx context.Context, msg *sarama.ConsumerMessage) error {
	event, err := events.FromJSON(msg.Value)
	if err != nil {
		return fmt.Errorf("deserialise: %w", err)
	}

	p := event.Payload

	switch event.EventType {
	case events.EventTypePostCreated, events.EventTypePostUpdated:
		return h.upsertPost(ctx, p, event)

	case events.EventTypeCommentAdded:
		if err := h.upsertPost(ctx, p, event); err != nil {
			return err
		}
		if err := h.recordInteraction(ctx, p); err != nil {
			return err
		}
		return h.incrementPostStat(ctx, p["postId"], "stats.comments")

	case events.EventTypeTagTrending:
		return h.updateTagStats(ctx, p)

	default:
		log.Printf("[CG2] Unknown event type %q — skipping", event.EventType)
		return nil
	}
}

// ---------------------------------------------------------------------------
// MongoDB operations — all use upsert for idempotency
// ---------------------------------------------------------------------------

func (h *mongoHandler) upsertPost(ctx context.Context, p map[string]string, event events.NewsEvent) error {
	postID, _ := strconv.Atoi(p["postId"])
	authorID, _ := strconv.Atoi(p["authorId"])
	channelID, _ := strconv.Atoi(p["channelId"])

	var tagList []string
	if raw := p["tags"]; raw != "" {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tagList = append(tagList, t)
			}
		}
	}

	filter := bson.M{"post_id": postID}
	update := bson.M{
		"$set": bson.M{
			"post_id":      postID,
			"title":        p["title"],
			"content":      fmt.Sprintf("Full content for post %d", postID),
			"content_hash": p["contentHash"],
			"tags":         tagList,
			"author_id":    authorID,
			"channel_id":   channelID,
			"updated_at":   time.Now().UTC(),
			"_kafka_meta": bson.M{
				"eventId": event.EventID,
				"source":  event.Source,
				"version": event.Version,
			},
		},
		"$setOnInsert": bson.M{
			"stats":      bson.M{"views": 0, "likes": 0, "comments": 0},
			"created_at": time.Now().UTC(),
		},
	}

	opts := options.Update().SetUpsert(true)
	res, err := h.mdb.posts.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("mongo upsertPost: %w", err)
	}
	log.Printf("[CG2] MongoDB posts: post_id=%d matched=%d modified=%d upserted=%v",
		postID, res.MatchedCount, res.ModifiedCount, res.UpsertedID != nil)
	return nil
}

func (h *mongoHandler) recordInteraction(ctx context.Context, p map[string]string) error {
	postID, _ := strconv.Atoi(p["postId"])
	doc := bson.M{
		"user_id":   p["nickname"],
		"post_id":   postID,
		"action":    "comment",
		"timestamp": time.Now().UTC(),
		"metadata": bson.M{
			"commentId":   p["commentId"],
			"textPreview": truncate(p["textPreview"], 100),
		},
	}
	if _, err := h.mdb.interactions.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("mongo recordInteraction: %w", err)
	}
	log.Printf("[CG2] MongoDB interactions: comment recorded for post_id=%d", postID)
	return nil
}

func (h *mongoHandler) incrementPostStat(ctx context.Context, postIDStr, field string) error {
	postID, _ := strconv.Atoi(postIDStr)
	_, err := h.mdb.posts.UpdateOne(ctx,
		bson.M{"post_id": postID},
		bson.M{"$inc": bson.M{field: 1}},
	)
	return err
}

func (h *mongoHandler) updateTagStats(ctx context.Context, p map[string]string) error {
	tagName := p["tagName"]
	score, _ := strconv.ParseFloat(p["trendScore"], 64)
	postCount, _ := strconv.Atoi(p["postCount"])

	filter := bson.M{"tag_name": tagName}
	update := bson.M{"$set": bson.M{
		"total_score": score,
		"post_count":  postCount,
		"updated_at":  time.Now().UTC(),
	}}
	opts := options.Update().SetUpsert(true)
	if _, err := h.mdb.topPostsView.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("mongo updateTagStats: %w", err)
	}
	log.Printf("[CG2] MongoDB top_posts_view: tag=%q score=%.4f", tagName, score)
	return nil
}

// ---------------------------------------------------------------------------
// DLQ
// ---------------------------------------------------------------------------

func (h *mongoHandler) sendToDLQ(msg *sarama.ConsumerMessage, cause error) {
	env := map[string]any{
		"originalTopic":  msg.Topic,
		"originalOffset": msg.Offset,
		"errorMessage":   cause.Error(),
		"consumerGroup":  config.GroupMongoEnricher,
		"dlqTimestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(env)
	dlqMsg := &sarama.ProducerMessage{
		Topic: config.TopicDLQ,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(body),
		Headers: []sarama.RecordHeader{
			{Key: []byte("dlq-consumer-group"), Value: []byte(config.GroupMongoEnricher)},
		},
	}
	if _, _, err := h.dlqProd.SendMessage(dlqMsg); err != nil {
		log.Printf("[CG2] DLQ send failed: %v", err)
	} else {
		log.Printf("[CG2] DLQ: forwarded offset=%d", msg.Offset)
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mongoClient, mdb, err := newMongoDB(ctx)
	if err != nil {
		log.Fatalf("[CG2] mongo connect: %v", err)
	}
	defer mongoClient.Disconnect(ctx)

	// DLQ producer
	dlqCfg := sarama.NewConfig()
	dlqCfg.Producer.RequiredAcks = sarama.WaitForAll
	dlqCfg.Producer.Return.Successes = true
	dlqProd, err := sarama.NewSyncProducer([]string{config.KafkaBootstrap}, dlqCfg)
	if err != nil {
		log.Fatalf("[CG2] dlq producer: %v", err)
	}
	defer dlqProd.Close()

	// Consumer group config — AUTO commit every 5 s
	cgCfg := sarama.NewConfig()
	cgCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cgCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cgCfg.Consumer.Offsets.AutoCommit.Enable = true              // ← AUTO COMMIT
	cgCfg.Consumer.Offsets.AutoCommit.Interval = 5 * time.Second // every 5 seconds

	group, err := sarama.NewConsumerGroup([]string{config.KafkaBootstrap}, config.GroupMongoEnricher, cgCfg)
	if err != nil {
		log.Fatalf("[CG2] consumer group: %v", err)
	}
	defer group.Close()

	handler := &mongoHandler{mdb: mdb, dlqProd: dlqProd}
	topics := []string{
		config.TopicPostsCreated,
		config.TopicPostsUpdated,
		config.TopicCommentsAdded,
		config.TopicTagsTrending,
	}

	log.Printf("[CG2] Consumer group %q started (AUTO commit 5s). Topics: %v",
		config.GroupMongoEnricher, topics)

	for {
		if err := group.Consume(ctx, topics, handler); err != nil {
			log.Printf("[CG2] Consume error: %v", err)
		}
		if ctx.Err() != nil {
			log.Println("[CG2] Shutting down.")
			return
		}
	}
}
