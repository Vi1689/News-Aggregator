package kafka

import (
	"context"
	"encoding/json"
	"time"

	"news-aggregator/internal/models"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	Writer *kafka.Writer
	Topic  string
}

func NewProducer(brokers []string, topic string) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		BatchSize:    100,
		BatchTimeout: 100 * time.Millisecond,
	}

	return &Producer{
		Writer: writer,
		Topic:  topic,
	}
}

func (p *Producer) Close() error {
	return p.Writer.Close()
}

// Отправка одного события
func (p *Producer) SendEvent(ctx context.Context, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Извлекаем newsId для ключа (если есть)
	var key string
	switch e := event.(type) {
	case models.NewsPublishedEvent:
		key = e.Payload.NewsID
	case models.NewsViewedEvent:
		key = e.Payload.NewsID
	case models.NewsLikedEvent:
		key = e.Payload.NewsID
	case models.NewsSharedEvent:
		key = e.Payload.NewsID
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: data,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(getEventType(event))},
			{Key: "version", Value: []byte("1")},
		},
	}

	return p.Writer.WriteMessages(ctx, msg)
}

// Массовая отправка событий
func (p *Producer) SendBatch(ctx context.Context, events []interface{}) error {
	messages := make([]kafka.Message, len(events))

	for i, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}

		var key string
		switch e := event.(type) {
		case models.NewsPublishedEvent:
			key = e.Payload.NewsID
		case models.NewsViewedEvent:
			key = e.Payload.NewsID
		case models.NewsLikedEvent:
			key = e.Payload.NewsID
		case models.NewsSharedEvent:
			key = e.Payload.NewsID
		}

		messages[i] = kafka.Message{
			Key:   []byte(key),
			Value: data,
			Time:  time.Now(),
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(getEventType(event))},
			},
		}
	}

	return p.Writer.WriteMessages(ctx, messages...)
}

func getEventType(event interface{}) string {
	switch event.(type) {
	case models.NewsPublishedEvent:
		return "NewsPublished"
	case models.NewsViewedEvent:
		return "NewsViewed"
	case models.NewsLikedEvent:
		return "NewsLiked"
	case models.NewsSharedEvent:
		return "NewsShared"
	case models.CommentAddedEvent:
		return "CommentAdded"
	default:
		return "Unknown"
	}
}
