package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/IBM/sarama"
	_ "github.com/lib/pq"
)

// NewsEvent — структура сообщения из топика news-enriched
type NewsEvent struct {
    EventId             string          `json:"eventId"`           // ID события из Kafka
    EventType           string          `json:"eventType"`         // NewsPublished / NewsUpdated / NewsTrending
    Timestamp           string          `json:"timestamp"`         // Время события
    Source              string          `json:"source"`            // reuters, bbc, cnn, aljazeera, tass
    Version             string          `json:"version"`           // Версия формата
    EntryId             string          `json:"entryId"`           // Уникальный ID записи
    Enriched            bool            `json:"enriched"`          // true (добавлено streams.py)
    SourceRegion        string          `json:"sourceRegion"`  	   // Global, Europe, Americas и т.д.
    SourceReliability   string          `json:"sourceReliability"` // HIGH / MEDIUM
    CredibilityScore    float64         `json:"credibilityScore"`  // Оценка достоверности
    ProcessedAt         string          `json:"processedAt"`       // Время обработки
    Payload             json.RawMessage `json:"payload"`           // Вложенные данные
}

// ConsumerGroupHandler реализует интерфейс sarama.ConsumerGroupHandler
type ConsumerGroupHandler struct {
	db *sql.DB
}

func (h *ConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error {
	log.Println("[Go Sink] Consumer setup complete")
	return nil
}

func (h *ConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	log.Println("[Go Sink] Consumer cleanup complete")
	return nil
}

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    for msg := range claim.Messages() {        // Берём каждое сообщение из Kafka
        h.processMessage(msg)                   // Обрабатываем его
        session.MarkMessage(msg, "")            // Подтверждаем, что обработали (коммитим offset)
    }
    return nil
}

func (h *ConsumerGroupHandler) processMessage(msg *sarama.ConsumerMessage) {
	var event NewsEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("[Go Sink] Failed to parse JSON: %v", err)
		return
	}

	_, err := h.db.Exec(`
		INSERT INTO news_enriched (
			eventid, eventtype, timestamp, source, version, entryid,
			enriched, sourceregion, sourcereliability, credibilityscore,
			processedat, payload
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (eventid) DO NOTHING
	`,
		event.EventId, event.EventType, event.Timestamp, event.Source,
		event.Version, event.EntryId, event.Enriched, event.SourceRegion,
		event.SourceReliability, event.CredibilityScore, event.ProcessedAt,
		event.Payload,
	)

	if err != nil {
		log.Printf("[Go Sink] Failed to insert: %v", err)
	} else {
		log.Printf("[Go Sink] ✓ Inserted: %s | %s | %s", event.EventId[:8], event.EventType, event.Source)
	}
}

func main() {
	log.Println("========================================")
	log.Println("Go Kafka Sink Connector - News Aggregator")
	log.Println("========================================")

	// 1. PostgreSQL connection
	psqlInfo := "host=localhost port=5436 user=news_user password=news_password dbname=news_db sslmode=disable"
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal("[Go Sink] Failed to connect to PostgreSQL:", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("[Go Sink] PostgreSQL not reachable:", err)
	}
	log.Println("[Go Sink] Connected to PostgreSQL")

	// 2. Create table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS news_enriched (
			eventid VARCHAR(36) PRIMARY KEY,
			eventtype VARCHAR(50),
			timestamp TIMESTAMPTZ,
			source VARCHAR(100),
			version VARCHAR(10),
			entryid VARCHAR(100),
			enriched BOOLEAN,
			sourceregion VARCHAR(100),
			sourcereliability VARCHAR(20),
			credibilityscore FLOAT,
			processedat TIMESTAMPTZ,
			payload JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatal("[Go Sink] Failed to create table:", err)
	}
	log.Println("[Go Sink] Table ready")

	// 3. Kafka config
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Return.Errors = true

	brokers := []string{"localhost:9092", "localhost:9093", "localhost:9094"}
	groupID := "go-sink-connector"
	topics := []string{"news-enriched"}

	// 4. Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		log.Fatalf("[Go Sink] Failed to create consumer group: %v", err)
	}
	defer consumerGroup.Close()

	// 5. Start consuming
	ctx, cancel := context.WithCancel(context.Background())
	handler := &ConsumerGroupHandler{db: db}

	go func() {
		for {
			if err := consumerGroup.Consume(ctx, topics, handler); err != nil {
				if !strings.Contains(err.Error(), "context canceled") {
					log.Printf("[Go Sink] Consumer error: %v", err)
				}
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	log.Println("[Go Sink] 🚀 CONNECTOR RUNNING")
	log.Println("[Go Sink] Listening to topic: news-enriched")
	log.Println("[Go Sink] Consumer group: go-sink-connector")
	log.Println("[Go Sink] Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, os.Interrupt)
	<-sigterm

	log.Println("[Go Sink] Shutting down...")
	cancel()
	time.Sleep(2 * time.Second)
	log.Println("[Go Sink] Done")
}