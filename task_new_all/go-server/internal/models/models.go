package models

import "time"

// ============================================
// NEO4J МОДЕЛИ (ГРАФ)
// ============================================

type News struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	PublishedAt time.Time `json:"published_at"`
	SourceURL   string    `json:"source_url"`
	Language    string    `json:"language"`
}

type Category struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Author struct {
	Name           string `json:"name"`
	Type           string `json:"type"` // journalist, agency, blogger
	FollowersCount int    `json:"followers_count"`
}

type Tag struct {
	Name string `json:"name"`
}

type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	RegisteredAt time.Time `json:"registered_at"`
}

// ============================================
// KAFKA СОБЫТИЯ
// ============================================

type BaseEvent struct {
	EventID   string    `json:"eventId"`
	EventType string    `json:"eventType"`
	EntityID  string    `json:"entityId"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	Version   int       `json:"version"`
}

// Событие 1: Публикация новости
type NewsPublishedEvent struct {
	BaseEvent
	Payload struct {
		NewsID      string    `json:"newsId"`
		Title       string    `json:"title"`
		Summary     string    `json:"summary"`
		FullText    string    `json:"fullText"`
		Author      string    `json:"author"`
		AuthorType  string    `json:"authorType"`
		Categories  []string  `json:"categories"`
		Tags        []string  `json:"tags"`
		PublishedAt time.Time `json:"publishedAt"`
		SourceURL   string    `json:"sourceUrl"`
		Language    string    `json:"language"`
	} `json:"payload"`
}

// Событие 2: Просмотр новости
type NewsViewedEvent struct {
	BaseEvent
	Payload struct {
		NewsID          string  `json:"newsId"`
		UserID          string  `json:"userId"`
		ReadDurationSec int     `json:"readDurationSec"`
		ScrollDepth     float64 `json:"scrollDepth"`
		DeviceType      string  `json:"deviceType"`
		SessionID       string  `json:"sessionId"`
	} `json:"payload"`
}

// Событие 3: Лайк новости
type NewsLikedEvent struct {
	BaseEvent
	Payload struct {
		NewsID    string `json:"newsId"`
		UserID    string `json:"userId"`
		LikeValue int    `json:"likeValue"` // 1 или -1 (дизлайк)
	} `json:"payload"`
}

// Событие 4: Шер новости
type NewsSharedEvent struct {
	BaseEvent
	Payload struct {
		NewsID         string `json:"newsId"`
		UserID         string `json:"userId"`
		Platform       string `json:"platform"` // telegram, twitter, facebook
		SharedToUserID string `json:"sharedToUserId"`
	} `json:"payload"`
}

// Событие 5: Комментарий
type CommentAddedEvent struct {
	BaseEvent
	Payload struct {
		NewsID          string `json:"newsId"`
		UserID          string `json:"userId"`
		CommentText     string `json:"commentText"`
		ParentCommentID string `json:"parentCommentId"`
	} `json:"payload"`
}

// Универсальный контейнер для событий
type EventWrapper struct {
	BaseEvent
	Payload map[string]interface{} `json:"payload"`
}

// ============================================
// HTTP ЗАПРОСЫ/ОТВЕТЫ
// ============================================

type GenerateRequest struct {
	Nodes      int      `json:"nodes"`       // количество узлов (50)
	Relations  int      `json:"relations"`   // количество связей (120)
	Events     int      `json:"events"`      // количество событий (100000)
	Types      []string `json:"types"`       // типы событий для генерации
	CleanFirst bool     `json:"clean_first"` // очистить перед генерацией
}

type GenerateResponse struct {
	Message      string `json:"message"`
	NodesCreated int    `json:"nodes_created"`
	RelsCreated  int    `json:"rels_created"`
	EventsSent   int    `json:"events_sent"`
	Duration     string `json:"duration"`
}

type RecommendRequest struct {
	NewsID    string  `json:"news_id"`
	Limit     int     `json:"limit"`
	MinWeight float64 `json:"min_weight"`
}

type RecommendResponse struct {
	NewsID           string   `json:"news_id"`
	Title            string   `json:"title"`
	Similarity       float64  `json:"similarity"`
	CommonTags       []string `json:"common_tags"`
	CommonCategories []string `json:"common_categories"`
}

type TopNewsResponse struct {
	NewsID   string  `json:"news_id"`
	Title    string  `json:"title"`
	Views    int     `json:"views"`
	Likes    int     `json:"likes"`
	LikeRate float64 `json:"like_rate"`
}
