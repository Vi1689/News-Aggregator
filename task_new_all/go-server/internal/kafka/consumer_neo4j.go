package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"news-aggregator/internal/database"
	"news-aggregator/internal/models"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/segmentio/kafka-go"
)

type Neo4jConsumer struct {
	Reader      *kafka.Reader
	Neo4jClient *database.Neo4jClient
	DLQProducer *Producer
}

func NewNeo4jConsumer(brokers []string, topic, groupID string, neo4jClient *database.Neo4jClient, dlqProducer *Producer) *Neo4jConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		MaxWait:        time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	return &Neo4jConsumer{
		Reader:      reader,
		Neo4jClient: neo4jClient,
		DLQProducer: dlqProducer,
	}
}

func (c *Neo4jConsumer) Close() error {
	return c.Reader.Close()
}

// Запуск consumer'а
func (c *Neo4jConsumer) Start(ctx context.Context) {
	log.Println("Starting Neo4j Kafka consumer...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Neo4j Kafka consumer...")
			return
		default:
			// Установите таймаут для FetchMessage
			fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			msg, err := c.Reader.FetchMessage(fetchCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					continue
				}
				log.Printf("Error fetching message: %v", err)
				time.Sleep(time.Second) // Избегаем busy loop
				continue
			}

			// Обрабатываем сообщение
			err = c.processMessage(ctx, msg)
			if err != nil {
				log.Printf("Error processing message: %v", err)
				// Отправляем в DLQ
				c.sendToDLQ(ctx, msg, err)
			}

			// Commit offset (manual commit)
			if err := c.Reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("Error committing message: %v", err)
			}
		}
	}
}

func (c *Neo4jConsumer) processMessage(ctx context.Context, msg kafka.Message) error {
	// Определяем тип события по заголовку
	var eventType string
	for _, header := range msg.Headers {
		if header.Key == "event_type" {
			eventType = string(header.Value)
			break
		}
	}

	switch eventType {
	case "NewsPublished":
		var event models.NewsPublishedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		return c.handleNewsPublished(ctx, event)

	case "NewsViewed":
		var event models.NewsViewedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		return c.handleNewsViewed(ctx, event)

	case "NewsLiked":
		var event models.NewsLikedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		return c.handleNewsLiked(ctx, event)

	case "NewsShared":
		var event models.NewsSharedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		return c.handleNewsShared(ctx, event)

	case "NewsMentions":
		var event models.NewsMentionsEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		return c.handleNewsMentions(ctx, event)
	}

	return nil
}

func (c *Neo4jConsumer) handleNewsPublished(ctx context.Context, event models.NewsPublishedEvent) error {
	// Создаём узел новости
	news := &models.News{
		ID:          event.Payload.NewsID,
		Title:       event.Payload.Title,
		Summary:     event.Payload.Summary,
		PublishedAt: event.Payload.PublishedAt,
		SourceURL:   event.Payload.SourceURL,
		Language:    event.Payload.Language,
	}

	if err := c.Neo4jClient.CreateNews(ctx, news); err != nil {
		return err
	}

	// Создаём связи с категориями
	for _, category := range event.Payload.Categories {
		if err := c.Neo4jClient.CreateBelongsToRelation(ctx, event.Payload.NewsID, category); err != nil {
			log.Printf("Error creating category relation: %v", err)
		}
	}

	// Создаём связь с автором
	if err := c.Neo4jClient.CreateWrittenByRelation(ctx, event.Payload.NewsID, event.Payload.Author, event.Payload.AuthorType); err != nil {
		return err
	}

	// Создаём связи с тегами
	for _, tag := range event.Payload.Tags {
		weight := 1.0 // можно вычислять на основе частоты
		if err := c.Neo4jClient.CreateTaggedWithRelation(ctx, event.Payload.NewsID, tag, weight); err != nil {
			log.Printf("Error creating tag relation: %v", err)
		}
	}

	log.Printf("Processed NewsPublished: %s", event.Payload.NewsID)
	return nil
}

func (c *Neo4jConsumer) handleNewsViewed(ctx context.Context, event models.NewsViewedEvent) error {
	// Создаём пользователя если его нет
	user := &models.User{
		ID:           event.Payload.UserID,
		Name:         "user_" + event.Payload.UserID,
		RegisteredAt: time.Now(),
	}

	// Создаём связь просмотра
	if err := c.Neo4jClient.CreateViewedRelation(ctx, user.ID, event.Payload.NewsID, event.Payload.ReadDurationSec, event.Timestamp); err != nil {
		return err
	}

	log.Printf("Processed NewsViewed: user=%s news=%s", event.Payload.UserID, event.Payload.NewsID)
	return nil
}

func (c *Neo4jConsumer) handleNewsLiked(ctx context.Context, event models.NewsLikedEvent) error {
	// Создаём связь лайка
	if err := c.Neo4jClient.CreateLikedRelation(ctx, event.Payload.UserID, event.Payload.NewsID, event.Timestamp); err != nil {
		return err
	}

	log.Printf("Processed NewsLiked: user=%s news=%s value=%d", event.Payload.UserID, event.Payload.NewsID, event.Payload.LikeValue)
	return nil
}

func (c *Neo4jConsumer) handleNewsShared(ctx context.Context, event models.NewsSharedEvent) error {
	// Создаём связи между пользователями (шер)
	// Сначала убеждаемся что оба пользователя существуют
	// fromUser := &models.User{ID: event.Payload.UserID, Name: "user_" + event.Payload.UserID, RegisteredAt: time.Now()}
	// toUser := &models.User{ID: event.Payload.SharedToUserID, Name: "user_" + event.Payload.SharedToUserID, RegisteredAt: time.Now()}

	// Создаём связь SHARED_TO
	session := c.Neo4jClient.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.Neo4jClient.Database})
	defer session.Close(ctx)

	query := `
        MATCH (from:User {id: $from_id})
        MATCH (to:User {id: $to_id})
        MERGE (from)-[:SHARED_TO {
            platform: $platform,
            timestamp: $timestamp,
            news_id: $news_id
        }]->(to)
    `

	_, err := session.Run(ctx, query, map[string]interface{}{
		"from_id":   event.Payload.UserID,
		"to_id":     event.Payload.SharedToUserID,
		"platform":  event.Payload.Platform,
		"timestamp": event.Timestamp,
		"news_id":   event.Payload.NewsID,
	})

	if err != nil {
		return err
	}

	log.Printf("Processed NewsShared: %s -> %s", event.Payload.UserID, event.Payload.SharedToUserID)
	return nil
}

func (c *Neo4jConsumer) sendToDLQ(ctx context.Context, msg kafka.Message, err error) {
	if c.DLQProducer == nil {
		return
	}

	dlqMsg := map[string]interface{}{
		"original_message": string(msg.Value),
		"error":            err.Error(),
		"timestamp":        time.Now(),
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
			{Key: "error", Value: []byte(err.Error())},
		},
	}

	if sendErr := c.DLQProducer.Writer.WriteMessages(ctx, dlqKafkaMsg); sendErr != nil {
		log.Printf("Failed to send to DLQ: %v", sendErr)
	}
}

func (c *Neo4jConsumer) handleNewsMentions(ctx context.Context, event models.NewsMentionsEvent) error {
	session := c.Neo4jClient.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Neo4jClient.Database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (from:News {id: $from_id})
		MATCH (to:News {id: $to_id})
		MERGE (from)-[:MENTIONS {
			strength: $strength,
			context: $context,
			timestamp: $timestamp
		}]->(to)
	`

	_, err := session.Run(ctx, query, map[string]interface{}{
		"from_id":   event.Payload.FromNewsID,
		"to_id":     event.Payload.ToNewsID,
		"strength":  event.Payload.Strength,
		"context":   event.Payload.Context,
		"timestamp": event.Timestamp,
	})

	if err != nil {
		log.Printf("Failed to create MENTIONS relation: %v", err)
		return err
	}

	log.Printf("Created MENTIONS relation: %s -> %s (strength=%d)",
		event.Payload.FromNewsID, event.Payload.ToNewsID, event.Payload.Strength)
	return nil
}
