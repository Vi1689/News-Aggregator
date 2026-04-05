package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"news-aggregator/internal/database"
	"news-aggregator/internal/models"

	"github.com/segmentio/kafka-go"
)

type ClickHouseConsumer struct {
	Reader           *kafka.Reader
	ClickHouseClient *database.ClickHouseClient
	DLQProducer      *Producer
}

func NewClickHouseConsumer(brokers []string, topic, groupID string, clickhouseClient *database.ClickHouseClient, dlqProducer *Producer) *ClickHouseConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		MaxWait:        time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	return &ClickHouseConsumer{
		Reader:           reader,
		ClickHouseClient: clickhouseClient,
		DLQProducer:      dlqProducer,
	}
}

func (c *ClickHouseConsumer) Close() error {
	return c.Reader.Close()
}

// Запуск consumer'а
func (c *ClickHouseConsumer) Start(ctx context.Context) {
	log.Println("Starting ClickHouse Kafka consumer...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping ClickHouse Kafka consumer...")
			return
		default:
			msg, err := c.Reader.FetchMessage(ctx)
			if err != nil {
				log.Printf("Error fetching message: %v", err)
				continue
			}

			// Обрабатываем сообщение
			err = c.processMessage(ctx, msg)
			if err != nil {
				log.Printf("Error processing message for ClickHouse: %v", err)
				c.sendToDLQ(ctx, msg, err)
			}

			// Commit offset
			if err := c.Reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("Error committing message: %v", err)
			}
		}
	}
}

func (c *ClickHouseConsumer) processMessage(ctx context.Context, msg kafka.Message) error {
	// Определяем тип события
	var eventType string
	for _, header := range msg.Headers {
		if header.Key == "event_type" {
			eventType = string(header.Value)
			break
		}
	}

	// Преобразуем в универсальный контейнер
	var rawEvent map[string]interface{}
	if err := json.Unmarshal(msg.Value, &rawEvent); err != nil {
		return err
	}

	eventWrapper := models.EventWrapper{
		BaseEvent: models.BaseEvent{
			EventID:   getString(rawEvent, "eventId"),
			EventType: eventType,
			EntityID:  getString(rawEvent, "entityId"),
			Source:    getString(rawEvent, "source"),
			Version:   getInt(rawEvent, "version"),
		},
		Payload: make(map[string]interface{}),
	}

	// Парсим timestamp
	if ts, ok := rawEvent["timestamp"]; ok {
		if tsStr, ok := ts.(string); ok {
			eventWrapper.Timestamp, _ = time.Parse(time.RFC3339, tsStr)
		}
	}

	// Извлекаем payload
	if payload, ok := rawEvent["payload"].(map[string]interface{}); ok {
		eventWrapper.Payload = payload
	}

	// Вставляем в ClickHouse
	return c.ClickHouseClient.InsertEvent(ctx, eventWrapper)
}

func (c *ClickHouseConsumer) sendToDLQ(ctx context.Context, msg kafka.Message, err error) {
	if c.DLQProducer == nil {
		return
	}

	dlqMsg := map[string]interface{}{
		"original_message": string(msg.Value),
		"error":            err.Error(),
		"timestamp":        time.Now(),
		"target":           "clickhouse",
		"topic":            msg.Topic,
		"partition":        msg.Partition,
		"offset":           msg.Offset,
	}

	data, _ := json.Marshal(dlqMsg)

	dlqKafkaMsg := kafka.Message{
		Key:   msg.Key,
		Value: data,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "original_topic", Value: []byte(msg.Topic)},
			{Key: "target", Value: []byte("clickhouse")},
			{Key: "error", Value: []byte(err.Error())},
		},
	}

	if sendErr := c.DLQProducer.Writer.WriteMessages(ctx, dlqKafkaMsg); sendErr != nil {
		log.Printf("Failed to send to DLQ: %v", sendErr)
	}
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if num, ok := val.(float64); ok {
			return int(num)
		}
	}
	return 0
}
