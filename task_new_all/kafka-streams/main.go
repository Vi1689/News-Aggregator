package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

type NewsEvent struct {
	EventID   string                 `json:"eventId"`
	EventType string                 `json:"eventType"`
	EntityID  string                 `json:"entityId"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Version   int                    `json:"version"`
	Payload   map[string]interface{} `json:"payload"`
}

type TagCount struct {
	Tag   string `json:"tag"`
	Hour  int64  `json:"hour"`
	Count int    `json:"count"`
}

func main() {
	// Конфигурация из переменных окружения
	brokers := []string{getEnv("KAFKA_BROKERS", "localhost:9092")}
	sourceTopic := getEnv("SOURCE_TOPIC", "news.events")
	targetTopic := getEnv("TARGET_TOPIC", "news.analytics")
	dlqTopic := getEnv("DLQ_TOPIC", "news.dlq")

	log.Printf("Starting Kafka Streams...")
	log.Printf("Brokers: %v, Source: %s, Target: %s", brokers, sourceTopic, targetTopic)

	// Reader для источника
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          sourceTopic,
		GroupID:        "kafka-streams-group",
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
	})
	defer reader.Close()

	// Writer для результатов
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  brokers,
		Topic:    targetTopic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	// Writer для DLQ
	dlqWriter := kafka.NewWriter(kafka.WriterConfig{
		Brokers: brokers,
		Topic:   dlqTopic,
	})
	defer dlqWriter.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Оконная агрегация: популярность тегов за час
	// tag -> hour_start -> count
	tagCounts := make(map[string]map[int64]int)

	// Канал для периодической отправки агрегатов
	aggregateChan := make(chan TagCount, 100)

	// Горутина: читаем сообщения из Kafka
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := reader.FetchMessage(ctx)
				if err != nil {
					log.Printf("Error fetching message: %v", err)
					continue
				}

				// Обрабатываем сообщение
				if err := processMessage(msg, tagCounts, aggregateChan); err != nil {
					log.Printf("Error processing message: %v", err)
					sendToDLQ(ctx, dlqWriter, msg, err)
				}

				// Commit offset (manual commit)
				if err := reader.CommitMessages(ctx, msg); err != nil {
					log.Printf("Error committing message: %v", err)
				}
			}
		}
	}()

	// Горутина: отправляем агрегаты каждые 5 минут
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				flushAggregates(ctx, writer, tagCounts)
			case agg := <-aggregateChan:
				sendAggregate(ctx, writer, agg)
			}
		}
	}()

	log.Println("Kafka Streams is running...")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Kafka Streams...")
	cancel()
	ticker.Stop()
	flushAggregates(context.Background(), writer, tagCounts)
	log.Println("Kafka Streams stopped")
}

func processMessage(msg kafka.Message, tagCounts map[string]map[int64]int, aggChan chan<- TagCount) error {
	var event NewsEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return err
	}

	// Только для событий публикации новости
	if event.EventType == "NewsPublished" {
		// Извлекаем теги
		tags, ok := event.Payload["tags"].([]interface{})
		if !ok {
			return nil
		}

		// Окно = час (усекаем до начала часа)
		hour := event.Timestamp.Truncate(time.Hour).Unix()

		for _, tag := range tags {
			tagStr, ok := tag.(string)
			if !ok {
				continue
			}

			if _, exists := tagCounts[tagStr]; !exists {
				tagCounts[tagStr] = make(map[int64]int)
			}
			tagCounts[tagStr][hour]++

			// Отправляем в канал для немедленной обработки
			aggChan <- TagCount{
				Tag:   tagStr,
				Hour:  hour,
				Count: tagCounts[tagStr][hour],
			}
		}
	}

	return nil
}

func sendAggregate(ctx context.Context, writer *kafka.Writer, agg TagCount) {
	aggMsg := map[string]interface{}{
		"aggregation_type": "tag_popularity",
		"tag":              agg.Tag,
		"hour":             agg.Hour,
		"count":            agg.Count,
		"timestamp":        time.Now(),
		"window_size":      "1h",
	}

	data, err := json.Marshal(aggMsg)
	if err != nil {
		log.Printf("Error marshaling aggregate: %v", err)
		return
	}

	if err := writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(agg.Tag),
		Value: data,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "aggregation_type", Value: []byte("tag_popularity")},
			{Key: "window", Value: []byte("1h")},
		},
	}); err != nil {
		log.Printf("Error sending aggregate: %v", err)
	}
}

func flushAggregates(ctx context.Context, writer *kafka.Writer, tagCounts map[string]map[int64]int) {
	for tag, hours := range tagCounts {
		for hour, count := range hours {
			aggMsg := map[string]interface{}{
				"aggregation_type": "tag_popularity",
				"tag":              tag,
				"hour":             hour,
				"count":            count,
				"timestamp":        time.Now(),
				"window_size":      "1h",
			}

			data, err := json.Marshal(aggMsg)
			if err != nil {
				log.Printf("Error marshaling aggregate: %v", err)
				continue
			}

			if err := writer.WriteMessages(ctx, kafka.Message{
				Key:   []byte(tag),
				Value: data,
				Time:  time.Now(),
				Headers: []kafka.Header{
					{Key: "aggregation_type", Value: []byte("tag_popularity")},
					{Key: "window", Value: []byte("1h")},
				},
			}); err != nil {
				log.Printf("Error sending aggregate: %v", err)
			}
		}
	}
	log.Println("Flushed aggregates to Kafka")
}

func sendToDLQ(ctx context.Context, writer *kafka.Writer, msg kafka.Message, err error) {
	dlqMsg := map[string]interface{}{
		"original_message": string(msg.Value),
		"error":            err.Error(),
		"timestamp":        time.Now(),
		"component":        "kafka-streams",
		"topic":            msg.Topic,
		"partition":        msg.Partition,
		"offset":           msg.Offset,
	}

	data, _ := json.Marshal(dlqMsg)

	if sendErr := writer.WriteMessages(ctx, kafka.Message{
		Key:   msg.Key,
		Value: data,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "original_topic", Value: []byte(msg.Topic)},
			{Key: "component", Value: []byte("kafka-streams")},
			{Key: "error", Value: []byte(err.Error())},
		},
	}); sendErr != nil {
		log.Printf("Failed to send to DLQ: %v", sendErr)
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
