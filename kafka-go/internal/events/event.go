// Package events defines the unified event envelope for the news aggregator.
// Every message published to Kafka must be wrapped in a NewsEvent.
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType enumerates all supported business event types.
type EventType string

const (
	EventTypePostCreated      EventType = "POST_CREATED"
	EventTypePostUpdated      EventType = "POST_UPDATED"
	EventTypeCommentAdded     EventType = "COMMENT_ADDED"
	EventTypeTagTrending      EventType = "TAG_TRENDING"
	EventTypePostLiked        EventType = "POST_LIKED"
	EventTypeChannelSubscribed EventType = "CHANNEL_SUBSCRIBED"
)

// Metadata holds cross-cutting fields attached to every event.
type Metadata struct {
	CorrelationID string  `json:"correlationId"`
	UserID        *string `json:"userId"`
	ChannelID     *int    `json:"channelId"`
	IPAddress     *string `json:"ipAddress"`
	UserAgent     *string `json:"userAgent"`
}

// NewsEvent is the unified envelope published to every Kafka topic.
// Schema mirrors news_event.avsc — all producers MUST use this struct.
type NewsEvent struct {
	EventID   string            `json:"eventId"`
	EventType EventType         `json:"eventType"`
	EntityID  string            `json:"entityId"`
	Timestamp int64             `json:"timestamp"` // epoch milliseconds
	Source    string            `json:"source"`
	Version   string            `json:"version"`
	Metadata  Metadata          `json:"metadata"`
	Payload   map[string]string `json:"payload"`
}

// BuildEvent constructs a NewsEvent with generated IDs and current timestamp.
func BuildEvent(
	eventType EventType,
	entityID string,
	source string,
	payload map[string]string,
	opts ...Option,
) NewsEvent {
	e := NewsEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		EntityID:  entityID,
		Timestamp: time.Now().UnixMilli(),
		Source:    source,
		Version:   "1.0",
		Metadata: Metadata{
			CorrelationID: uuid.New().String(),
		},
		Payload: payload,
	}
	for _, o := range opts {
		o(&e)
	}
	return e
}

// Option is a functional option for BuildEvent.
type Option func(*NewsEvent)

func WithUserID(uid string) Option {
	return func(e *NewsEvent) { e.Metadata.UserID = &uid }
}

func WithChannelID(cid int) Option {
	return func(e *NewsEvent) { e.Metadata.ChannelID = &cid }
}

// ToJSON serialises the event to JSON bytes (used as Kafka message value).
func (e NewsEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserialises a NewsEvent from JSON bytes.
func FromJSON(data []byte) (NewsEvent, error) {
	var e NewsEvent
	return e, json.Unmarshal(data, &e)
}

// MessageKey returns the Kafka partition key for a given event.
// Consistent key → consistent partition → ordered delivery per entity.
func MessageKey(eventType EventType, entityID string) string {
	switch eventType {
	case EventTypePostCreated, EventTypePostUpdated:
		return "channel-" + entityID
	case EventTypeCommentAdded:
		return "post-" + entityID
	case EventTypeTagTrending:
		return "tag-" + entityID
	default:
		return entityID
	}
}

// KafkaHeaders returns standard Kafka message headers for an event.
func KafkaHeaders(e NewsEvent) map[string]string {
	return map[string]string{
		"eventType":  string(e.EventType),
		"source":     e.Source,
		"version":    e.Version,
		"schemaName": "NewsEvent",
	}
}
