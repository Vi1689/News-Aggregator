package generator

import (
	"fmt"
	"math/rand"
	"time"

	"news-aggregator/internal/models"

	"github.com/google/uuid"
)

type Generator struct {
	// Данные для генерации
	categories []string
	tags       []string
	authors    []AuthorInfo
	users      []string
	newsTitles []string
	rng        *rand.Rand // Добавляем собственный генератор
}

type AuthorInfo struct {
	Name string
	Type string
}

func NewGenerator() *Generator {
	// Создаем генератор с уникальным seed'ом
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	return &Generator{
		categories: []string{
			"Technology", "Politics", "Sports", "Business", "Science",
			"Health", "Entertainment", "Education", "Environment", "Travel",
		},
		tags: []string{
			"ai", "blockchain", "crypto", "elections", "climate", "covid",
			"startup", "innovation", "cybersecurity", "space", "football",
			"basketball", "economy", "stockmarket", "healthcare",
		},
		authors: []AuthorInfo{
			{Name: "tech_crunch", Type: "agency"},
			{Name: "bbc_news", Type: "agency"},
			{Name: "reuters", Type: "agency"},
			{Name: "john_doe", Type: "journalist"},
			{Name: "jane_smith", Type: "journalist"},
			{Name: "alex_blogger", Type: "blogger"},
			{Name: "sci_weekly", Type: "agency"},
			{Name: "sports_beat", Type: "journalist"},
		},
		users: []string{
			"user_001", "user_002", "user_003", "user_004", "user_005",
			"user_006", "user_007", "user_008", "user_009", "user_010",
		},
		newsTitles: []string{
			"Breaking: Major discovery in quantum computing",
			"Election results: What you need to know",
			"Stock market hits all-time high",
			"New AI model surpasses human performance",
			"Climate summit reaches historic agreement",
			"Startup raises $100M for green tech",
			"Cybersecurity threats on the rise",
			"SpaceX launches new satellite constellation",
			"Football championship goes to overtime",
			"Revolutionary cancer treatment approved",
		},
		rng: rng,
	}
}

// Генерация новости
func (g *Generator) GenerateNews(index int) models.NewsPublishedEvent {
	author := g.authors[g.rng.Intn(len(g.authors))]

	// Выбираем случайные категории (1-3)
	numCategories := g.rng.Intn(3) + 1
	categories := make([]string, numCategories)
	for i := 0; i < numCategories; i++ {
		categories[i] = g.categories[g.rng.Intn(len(g.categories))]
	}

	// Выбираем случайные теги (2-5)
	numTags := g.rng.Intn(4) + 2
	tags := make([]string, numTags)
	for i := 0; i < numTags; i++ {
		tags[i] = g.tags[g.rng.Intn(len(g.tags))]
	}

	// Генерируем дату (последние 30 дней)
	publishedAt := time.Now().Add(-time.Duration(g.rng.Intn(30*24)) * time.Hour)

	event := models.NewsPublishedEvent{
		BaseEvent: models.BaseEvent{
			EventID:   uuid.New().String(),
			EventType: "NewsPublished",
			EntityID:  fmt.Sprintf("news_%d", index),
			Timestamp: time.Now(),
			Source:    "generator",
			Version:   1,
		},
	}

	event.Payload.NewsID = fmt.Sprintf("news_%d", index)
	event.Payload.Title = fmt.Sprintf("%s #%d", g.newsTitles[g.rng.Intn(len(g.newsTitles))], index)
	event.Payload.Summary = "This is a summary of the news article. It contains key information about the event."
	event.Payload.FullText = "Full text of the news article. Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
		"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam."
	event.Payload.Author = author.Name
	event.Payload.AuthorType = author.Type
	event.Payload.Categories = categories
	event.Payload.Tags = tags
	event.Payload.PublishedAt = publishedAt
	event.Payload.SourceURL = fmt.Sprintf("https://news.example.com/article/%d", index)
	event.Payload.Language = "en"

	return event
}

// Генерация просмотра новости
func (g *Generator) GenerateNewsView(newsID string, userID string, timestamp time.Time) models.NewsViewedEvent {
	// Выбираем случайное устройство
	devices := []string{"mobile", "desktop", "tablet"}
	deviceType := devices[g.rng.Intn(len(devices))]

	return models.NewsViewedEvent{
		BaseEvent: models.BaseEvent{
			EventID:   uuid.New().String(),
			EventType: "NewsViewed",
			EntityID:  userID,
			Timestamp: timestamp,
			Source:    "mobile_app",
			Version:   1,
		},
		Payload: struct {
			NewsID          string  `json:"newsId"`
			UserID          string  `json:"userId"`
			ReadDurationSec int     `json:"readDurationSec"`
			ScrollDepth     float64 `json:"scrollDepth"`
			DeviceType      string  `json:"deviceType"`
			SessionID       string  `json:"sessionId"`
		}{
			NewsID:          newsID,
			UserID:          userID,
			ReadDurationSec: g.rng.Intn(300) + 10, // 10-310 секунд
			ScrollDepth:     float64(g.rng.Intn(100)) / 100.0,
			DeviceType:      deviceType,
			SessionID:       uuid.New().String(),
		},
	}
}

// Генерация лайка
func (g *Generator) GenerateNewsLike(newsID string, userID string, timestamp time.Time) models.NewsLikedEvent {
	// 90% лайков, 10% дизлайков
	likeValue := 1
	if g.rng.Intn(100) < 10 {
		likeValue = -1
	}

	return models.NewsLikedEvent{
		BaseEvent: models.BaseEvent{
			EventID:   uuid.New().String(),
			EventType: "NewsLiked",
			EntityID:  userID,
			Timestamp: timestamp,
			Source:    "mobile_app",
			Version:   1,
		},
		Payload: struct {
			NewsID    string `json:"newsId"`
			UserID    string `json:"userId"`
			LikeValue int    `json:"likeValue"`
		}{
			NewsID:    newsID,
			UserID:    userID,
			LikeValue: likeValue,
		},
	}
}

// Генерация шера
func (g *Generator) GenerateNewsShare(newsID string, fromUserID, toUserID string, timestamp time.Time) models.NewsSharedEvent {
	platforms := []string{"telegram", "twitter", "facebook", "whatsapp"}

	return models.NewsSharedEvent{
		BaseEvent: models.BaseEvent{
			EventID:   uuid.New().String(),
			EventType: "NewsShared",
			EntityID:  fromUserID,
			Timestamp: timestamp,
			Source:    "mobile_app",
			Version:   1,
		},
		Payload: struct {
			NewsID         string `json:"newsId"`
			UserID         string `json:"userId"`
			Platform       string `json:"platform"`
			SharedToUserID string `json:"sharedToUserId"`
		}{
			NewsID:         newsID,
			UserID:         fromUserID,
			Platform:       platforms[g.rng.Intn(len(platforms))],
			SharedToUserID: toUserID,
		},
	}
}

// Полная генерация набора данных
func (g *Generator) GenerateFullDataset(newsCount, eventsCount int) ([]models.NewsPublishedEvent, []interface{}) {
	newsEvents := make([]models.NewsPublishedEvent, newsCount)
	allEvents := make([]interface{}, 0, eventsCount)

	// Генерируем новости
	for i := 0; i < newsCount; i++ {
		newsEvents[i] = g.GenerateNews(i)
		allEvents = append(allEvents, newsEvents[i])
	}

	// Генерируем события взаимодействия
	for i := 0; i < eventsCount-newsCount; i++ {
		newsIndex := g.rng.Intn(newsCount)
		newsID := fmt.Sprintf("news_%d", newsIndex)
		userID := g.users[g.rng.Intn(len(g.users))]
		timestamp := time.Now().Add(-time.Duration(g.rng.Intn(7*24)) * time.Hour)

		eventType := g.rng.Intn(100)
		if eventType < 60 { // 60% просмотров
			allEvents = append(allEvents, g.GenerateNewsView(newsID, userID, timestamp))
		} else if eventType < 85 { // 25% лайков
			allEvents = append(allEvents, g.GenerateNewsLike(newsID, userID, timestamp))
		} else { // 15% шеров
			toUserID := g.users[g.rng.Intn(len(g.users))]
			allEvents = append(allEvents, g.GenerateNewsShare(newsID, userID, toUserID, timestamp))
		}
	}

	return newsEvents, allEvents
}
