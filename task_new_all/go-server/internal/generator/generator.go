package generator

import (
	"fmt"
	"math/rand"
	"time"

	"news-aggregator/internal/models"

	"github.com/google/uuid"
)

type Generator struct {
	rng *rand.Rand
}

type AuthorInfo struct {
	Name string
	Type string
}

func NewGenerator() *Generator {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &Generator{rng: rng}
}

// Динамическое создание категорий (количество = newsCount/5, но минимум 5)
func (g *Generator) generateCategories(newsCount int) []string {
	count := newsCount / 5
	if count < 5 {
		count = 5
	}
	categories := make([]string, count)
	for i := 0; i < count; i++ {
		categories[i] = fmt.Sprintf("category_%d", i+1)
	}
	return categories
}

// Динамическое создание тегов (количество = newsCount/2, но минимум 10)
func (g *Generator) generateTags(newsCount int) []string {
	count := newsCount / 2
	if count < 10 {
		count = 10
	}
	tags := make([]string, count)
	for i := 0; i < count; i++ {
		tags[i] = fmt.Sprintf("tag_%d", i+1)
	}
	return tags
}

// Динамическое создание авторов (количество = newsCount/4, но минимум 5)
func (g *Generator) generateAuthors(newsCount int) []AuthorInfo {
	count := newsCount / 4
	if count < 5 {
		count = 5
	}
	types := []string{"agency", "journalist", "blogger"}
	authors := make([]AuthorInfo, count)
	for i := 0; i < count; i++ {
		authors[i] = AuthorInfo{
			Name: fmt.Sprintf("author_%d", i+1),
			Type: types[g.rng.Intn(len(types))],
		}
	}
	return authors
}

// Динамическое создание пользователей (количество = newsCount/2, но минимум 10)
func (g *Generator) generateUsers(newsCount int) []string {
	count := newsCount / 2
	if count < 10 {
		count = 10
	}
	users := make([]string, count)
	for i := 0; i < count; i++ {
		users[i] = fmt.Sprintf("user_%03d", i+1)
	}
	// Добавляем demo_user
	users = append(users, "demo_user")
	return users
}

// Динамическое создание заголовков (количество = newsCount, уникальные)
func (g *Generator) generateNewsTitles(newsCount int) []string {
	titles := make([]string, newsCount)
	templates := []string{
		"Breaking: Major discovery in %s",
		"Revolutionary %s breakthrough",
		"New study reveals %s trends",
		"Experts warn about %s",
		"%s market shows growth",
		"Future of %s revealed",
		"Top %s innovations",
		"How %s is changing world",
		"Complete guide to %s",
		"Latest in %s technology",
	}

	topics := []string{
		"quantum computing", "artificial intelligence", "blockchain",
		"cybersecurity", "renewable energy", "space exploration",
		"biotechnology", "robotics", "virtual reality", "IoT",
		"cloud computing", "big data", "machine learning", "neural networks",
		"autonomous vehicles", "smart homes", "wearable tech", "5G networks",
	}

	for i := 0; i < newsCount; i++ {
		template := templates[g.rng.Intn(len(templates))]
		topic := topics[g.rng.Intn(len(topics))]
		titles[i] = fmt.Sprintf(template, topic)
	}
	return titles
}

// Популярные теги для создания связей (фиксированный набор для семантики)
func (g *Generator) getPopularTags() []string {
	return []string{"ai", "technology", "innovation", "trending", "breakthrough", "future", "science"}
}

// Генерация новости
func (g *Generator) GenerateNews(index int, categories, tags, newsTitles []string, authors []AuthorInfo) models.NewsPublishedEvent {
	author := authors[g.rng.Intn(len(authors))]

	// Категории (1-3 случайные)
	numCategories := g.rng.Intn(3) + 1
	selectedCategories := make([]string, numCategories)
	for i := 0; i < numCategories; i++ {
		selectedCategories[i] = categories[g.rng.Intn(len(categories))]
	}

	// Теги (3-6 штук)
	popularTags := g.getPopularTags()
	numTags := g.rng.Intn(4) + 3
	selectedTags := make([]string, numTags)

	// Первый тег — из популярных (для общих связей)
	selectedTags[0] = popularTags[index%len(popularTags)]
	// Остальные — случайные
	for i := 1; i < numTags; i++ {
		selectedTags[i] = tags[g.rng.Intn(len(tags))]
	}

	// Дата (последние 7 дней)
	daysAgo := g.rng.Intn(7)
	publishedAt := time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour)

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
	event.Payload.Title = newsTitles[g.rng.Intn(len(newsTitles))]
	event.Payload.Summary = "Summary of the news article."
	event.Payload.FullText = "Full text of the news article..."
	event.Payload.Author = author.Name
	event.Payload.AuthorType = author.Type
	event.Payload.Categories = selectedCategories
	event.Payload.Tags = selectedTags
	event.Payload.PublishedAt = publishedAt
	event.Payload.SourceURL = fmt.Sprintf("https://news.example.com/article/%d", index)
	event.Payload.Language = "en"

	return event
}

// Генерация просмотра
func (g *Generator) GenerateNewsView(newsID string, userID string, timestamp time.Time) models.NewsViewedEvent {
	devices := []string{"mobile", "desktop", "tablet"}
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
			ReadDurationSec: g.rng.Intn(300) + 10,
			ScrollDepth:     float64(g.rng.Intn(100)) / 100.0,
			DeviceType:      devices[g.rng.Intn(len(devices))],
			SessionID:       uuid.New().String(),
		},
	}
}

// Генерация лайка
func (g *Generator) GenerateNewsLike(newsID string, userID string, timestamp time.Time) models.NewsLikedEvent {
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

// Генерация цитирования (MENTIONS)
func (g *Generator) GenerateMentions(fromNewsID, toNewsID string) models.NewsMentionsEvent {
	return models.NewsMentionsEvent{
		BaseEvent: models.BaseEvent{
			EventID:   uuid.New().String(),
			EventType: "NewsMentions",
			EntityID:  fromNewsID,
			Timestamp: time.Now(),
			Source:    "generator",
			Version:   1,
		},
		Payload: models.MentionsPayload{
			FromNewsID: fromNewsID,
			ToNewsID:   toNewsID,
			Strength:   3,
			Context:    "related",
		},
	}
}

// Полная генерация набора данных (ВСЁ ДИНАМИЧЕСКИ)
func (g *Generator) GenerateFullDataset(newsCount, eventsCount int) ([]models.NewsPublishedEvent, []interface{}) {
	// ВСЕ списки генерируются на основе newsCount
	categories := g.generateCategories(newsCount)
	tags := g.generateTags(newsCount)
	authors := g.generateAuthors(newsCount)
	users := g.generateUsers(newsCount)
	newsTitles := g.generateNewsTitles(newsCount)

	newsEvents := make([]models.NewsPublishedEvent, newsCount)
	allEvents := make([]interface{}, 0, eventsCount+newsCount)

	// 1. Новости
	for i := 0; i < newsCount; i++ {
		newsEvents[i] = g.GenerateNews(i, categories, tags, newsTitles, authors)
		allEvents = append(allEvents, newsEvents[i])
	}

	// 2. Связи MENTIONS (каждая 3-я)
	for i := 1; i < newsCount; i++ {
		if i%3 == 0 {
			allEvents = append(allEvents, g.GenerateMentions(
				fmt.Sprintf("news_%d", i),
				fmt.Sprintf("news_%d", i-1),
			))
		}
	}

	// 3. События взаимодействия
	for i := 0; i < eventsCount; i++ {
		newsIndex := g.rng.Intn(newsCount)
		newsID := fmt.Sprintf("news_%d", newsIndex)
		userID := users[g.rng.Intn(len(users))]
		timestamp := time.Now().Add(-time.Duration(g.rng.Intn(7*24)) * time.Hour)

		eventType := g.rng.Intn(100)
		if eventType < 60 {
			allEvents = append(allEvents, g.GenerateNewsView(newsID, userID, timestamp))
		} else if eventType < 85 {
			allEvents = append(allEvents, g.GenerateNewsLike(newsID, userID, timestamp))
		} else {
			toUserID := users[g.rng.Intn(len(users))]
			allEvents = append(allEvents, g.GenerateNewsShare(newsID, userID, toUserID, timestamp))
		}
	}

	// 4. Гарантированные лайки от demo_user
	for i := 0; i < 5 && i < newsCount; i++ {
		allEvents = append(allEvents, g.GenerateNewsLike(
			fmt.Sprintf("news_%d", i),
			"demo_user",
			time.Now(),
		))
	}

	return newsEvents, allEvents
}
