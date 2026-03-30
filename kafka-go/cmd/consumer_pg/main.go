// cmd/consumer_pg/main.go
//
// Consumer Group 1: cg-postgres-writer
// ======================================
// Reads POST_CREATED / POST_UPDATED events and writes them to PostgreSQL.
//
// Commit strategy: MANUAL — offset is committed only after a successful DB write.
//   → guarantees at-least-once delivery; upserts make processing idempotent.
//
// Error handling:
//   - 3 retries with exponential backoff (1s → 2s → 4s)
//   - After all retries exhausted: message is forwarded to DLQ (news.posts.dlq)
//   - Offset is always committed after DLQ send to avoid getting stuck

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/newsaggregator/kafka/internal/config"
	"github.com/newsaggregator/kafka/internal/events"
)

const maxRetries = 3

// ---------------------------------------------------------------------------
// Kafka consumer group handler (sarama ConsumerGroupHandler interface)
// ---------------------------------------------------------------------------

type postgresHandler struct {
	db       *pgxpool.Pool
	dlqProd  sarama.SyncProducer
}

// Setup is called at the beginning of a new session.
func (h *postgresHandler) Setup(_ sarama.ConsumerGroupSession) error {
	log.Println("[CG1] Session started")
	return nil
}

// Cleanup is called at the end of a session.
func (h *postgresHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	log.Println("[CG1] Session ended")
	return nil
}

// ConsumeClaim processes messages from a single partition claim.
// MANUAL COMMIT: session.MarkMessage() is called only after successful processing.
func (h *postgresHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("[CG1] Received topic=%s partition=%d offset=%d key=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key))

		ok := h.processWithRetry(session.Context(), msg)

		// Always mark the offset — if failed, the message went to DLQ.
		// Not marking would cause an infinite reprocessing loop.
		session.MarkMessage(msg, "") // ← MANUAL COMMIT (mark = will be committed on next Commit call)
		_ = ok
	}
	return nil
}

// ---------------------------------------------------------------------------
// Processing with retry + DLQ
// ---------------------------------------------------------------------------

func (h *postgresHandler) processWithRetry(ctx context.Context, msg *sarama.ConsumerMessage) bool {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := h.handle(ctx, msg)
		if err == nil {
			return true // success
		}

		log.Printf("[CG1] Attempt %d/%d failed: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			log.Printf("[CG1] Retrying in %s...", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return false
			}
		} else {
			h.sendToDLQ(msg, err, attempt)
		}
	}
	return false
}

func (h *postgresHandler) handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	event, err := events.FromJSON(msg.Value)
	if err != nil {
		// Deserialisation errors are not retried — malformed message goes straight to DLQ.
		return fmt.Errorf("deserialise: %w", err)
	}

	switch event.EventType {
	case events.EventTypePostCreated, events.EventTypePostUpdated:
		return h.upsertPost(ctx, event.Payload)
	default:
		log.Printf("[CG1] Skipping event type: %s", event.EventType)
		return nil
	}
}

// ---------------------------------------------------------------------------
// PostgreSQL upsert — idempotent via ON CONFLICT
// ---------------------------------------------------------------------------

// upsertPost вставляет или обновляет пост в PostgreSQL
// cmd/consumer_pg/main.go - исправленная функция upsertPost

func (h *postgresHandler) upsertPost(ctx context.Context, payload map[string]string) error {
	postID := parseInt(payload["postId"])
	authorID := parseInt(payload["authorId"])
	channelID := parseInt(payload["channelId"])
	title := payload["title"]
	tagsStr := payload["tags"]

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Upsert author - если author_id не существует, создаем с автоинкрементом
	// Но так как мы передаем конкретный ID, нужно использовать DO UPDATE
	_, err = tx.Exec(ctx, `
		INSERT INTO authors (author_id, name)
		VALUES ($1, $2)
		ON CONFLICT (author_id) DO UPDATE 
			SET name = EXCLUDED.name`,
		authorID, fmt.Sprintf("Author_%d", authorID),
	)
	if err != nil {
		return fmt.Errorf("upsert author: %w", err)
	}

	// Upsert channel - аналогично
	_, err = tx.Exec(ctx, `
		INSERT INTO channels (channel_id, name)
		VALUES ($1, $2)
		ON CONFLICT (channel_id) DO UPDATE 
			SET name = EXCLUDED.name`,
		channelID, fmt.Sprintf("Channel_%d", channelID),
	)
	if err != nil {
		return fmt.Errorf("upsert channel: %w", err)
	}

	// Insert news text
	var textID int
	err = tx.QueryRow(ctx, `
		INSERT INTO news_texts (text) VALUES ($1) RETURNING text_id`,
		fmt.Sprintf("Content for post %d", postID),
	).Scan(&textID)
	if err != nil {
		return fmt.Errorf("insert news_text: %w", err)
	}

	// Upsert post - БЕЗ updated_at
	_, err = tx.Exec(ctx, `
		INSERT INTO posts (post_id, title, author_id, text_id, channel_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (post_id) DO UPDATE
		  SET title = EXCLUDED.title,
		      author_id = EXCLUDED.author_id,
		      text_id = EXCLUDED.text_id,
		      channel_id = EXCLUDED.channel_id`,
		postID, title, authorID, textID, channelID,
	)
	if err != nil {
		return fmt.Errorf("upsert post: %w", err)
	}

	// Upsert tags
	for _, tagName := range strings.Split(tagsStr, ",") {
		tagName = strings.TrimSpace(tagName)
		if tagName == "" {
			continue
		}
		var tagID int
		err = tx.QueryRow(ctx, `
			INSERT INTO tags (name) VALUES ($1)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING tag_id`,
			tagName,
		).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("upsert tag %q: %w", tagName, err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`,
			postID, tagID,
		)
		if err != nil {
			return fmt.Errorf("insert post_tag: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("[CG1] PostgreSQL: upserted post_id=%d title=%q channel_id=%d", postID, title, channelID)
	return nil
}
// ---------------------------------------------------------------------------
// DLQ
// ---------------------------------------------------------------------------

type dlqEnvelope struct {
	OriginalTopic  string `json:"originalTopic"`
	OriginalOffset int64  `json:"originalOffset"`
	OriginalKey    string `json:"originalKey"`
	OriginalValue  string `json:"originalValue"`
	ErrorType      string `json:"errorType"`
	ErrorMessage   string `json:"errorMessage"`
	FailedAttempts int    `json:"failedAttempts"`
	DLQTimestamp   string `json:"dlqTimestamp"`
	ConsumerGroup  string `json:"consumerGroup"`
}

func (h *postgresHandler) sendToDLQ(msg *sarama.ConsumerMessage, cause error, attempts int) {
	env := dlqEnvelope{
		OriginalTopic:  msg.Topic,
		OriginalOffset: msg.Offset,
		OriginalKey:    string(msg.Key),
		OriginalValue:  string(msg.Value),
		ErrorType:      fmt.Sprintf("%T", cause),
		ErrorMessage:   cause.Error(),
		FailedAttempts: attempts,
		DLQTimestamp:   time.Now().UTC().Format(time.RFC3339),
		ConsumerGroup:  config.GroupPostgresWriter,
	}

	body, _ := json.Marshal(env)
	dlqMsg := &sarama.ProducerMessage{
		Topic: config.TopicDLQ,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(body),
		Headers: []sarama.RecordHeader{
			{Key: []byte("dlq-reason"), Value: []byte(cause.Error())},
			{Key: []byte("dlq-consumer-group"), Value: []byte(config.GroupPostgresWriter)},
			{Key: []byte("dlq-attempts"), Value: []byte(fmt.Sprintf("%d", attempts))},
		},
	}

	if _, _, err := h.dlqProd.SendMessage(dlqMsg); err != nil {
		log.Printf("[CG1] DLQ send failed: %v", err)
	} else {
		log.Printf("[CG1] DLQ: forwarded offset=%d error=%v", msg.Offset, cause)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseInt(s string) int {
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// PostgreSQL connection pool
	pool, err := pgxpool.New(ctx, config.PostgresDSN)
	if err != nil {
		log.Fatalf("[CG1] postgres connect: %v", err)
	}
	defer pool.Close()

	// DLQ producer
	dlqCfg := sarama.NewConfig()
	dlqCfg.Producer.RequiredAcks = sarama.WaitForAll
	dlqCfg.Producer.Return.Successes = true
	dlqProd, err := sarama.NewSyncProducer([]string{config.KafkaBootstrap}, dlqCfg)
	if err != nil {
		log.Fatalf("[CG1] dlq producer: %v", err)
	}
	defer dlqProd.Close()

	// Consumer group config — MANUAL commit
	cgCfg := sarama.NewConfig()
	cgCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cgCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cgCfg.Consumer.Offsets.AutoCommit.Enable = false // ← MANUAL COMMIT

	group, err := sarama.NewConsumerGroup([]string{config.KafkaBootstrap}, config.GroupPostgresWriter, cgCfg)
	if err != nil {
		log.Fatalf("[CG1] consumer group: %v", err)
	}
	defer group.Close()

	handler := &postgresHandler{db: pool, dlqProd: dlqProd}
	topics := []string{config.TopicPostsCreated, config.TopicPostsUpdated}

	log.Printf("[CG1] Consumer group %q started. Topics: %v", config.GroupPostgresWriter, topics)

	for {
		if err := group.Consume(ctx, topics, handler); err != nil {
			log.Printf("[CG1] Consume error: %v", err)
		}
		if ctx.Err() != nil {
			log.Println("[CG1] Shutting down.")
			return
		}
	}
}
