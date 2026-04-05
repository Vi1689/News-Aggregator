package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"news-aggregator/internal/config"
	"news-aggregator/internal/database"
	"news-aggregator/internal/generator"
	"news-aggregator/internal/kafka"
	"news-aggregator/internal/models"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Config           *config.Config
	Neo4jClient      *database.Neo4jClient
	ClickHouseClient *database.ClickHouseClient
	KafkaProducer    *kafka.Producer
	Generator        *generator.Generator
}

func NewHandler(
	cfg *config.Config,
	neo4jClient *database.Neo4jClient,
	clickhouseClient *database.ClickHouseClient,
	kafkaProducer *kafka.Producer,
) *Handler {
	return &Handler{
		Config:           cfg,
		Neo4jClient:      neo4jClient,
		ClickHouseClient: clickhouseClient,
		KafkaProducer:    kafkaProducer,
		Generator:        generator.NewGenerator(),
	}
}

// Health check
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "news-aggregator",
		"timestamp": time.Now(),
	})
}

// Генерация данных
func (h *Handler) GenerateData(c *gin.Context) {
	var req models.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Значения по умолчанию
		req.Nodes = h.Config.GenerationNodes
		req.Relations = h.Config.GenerationRelations
		req.Events = h.Config.GenerationEvents
		req.CleanFirst = true
	}

	startTime := time.Now()

	ctx := c.Request.Context()

	// Очистка если нужно
	if req.CleanFirst {
		if err := h.Neo4jClient.CleanDatabase(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Генерируем данные
	log.Printf("Generating %d news and %d events...", req.Nodes, req.Events)
	newsEvents, allEvents := h.Generator.GenerateFullDataset(req.Nodes, req.Events)

	// Отправляем события в Kafka
	eventsSent := 0
	batchSize := h.Config.GenerationBatchSize

	for i := 0; i < len(allEvents); i += batchSize {
		end := i + batchSize
		if end > len(allEvents) {
			end = len(allEvents)
		}

		batch := allEvents[i:end]
		if err := h.KafkaProducer.SendBatch(ctx, batch); err != nil {
			log.Printf("Error sending batch: %v", err)
			continue
		}

		eventsSent += len(batch)

		// Задержка для имитации реального потока
		if h.Config.GenerationDelayMs > 0 {
			time.Sleep(time.Duration(h.Config.GenerationDelayMs) * time.Millisecond)
		}

		log.Printf("Sent %d/%d events", eventsSent, len(allEvents))
	}

	duration := time.Since(startTime)

	c.JSON(http.StatusOK, models.GenerateResponse{
		Message:      "Data generation completed",
		NodesCreated: len(newsEvents),
		RelsCreated:  req.Relations,
		EventsSent:   eventsSent,
		Duration:     duration.String(),
	})
}

// ============================================
// NEO4J ЗАПРОСЫ (ЗАДАНИЕ 1)
// ============================================

// 1. Похожие новости
func (h *Handler) GetSimilarNews(c *gin.Context) {
	newsID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	minWeight, _ := strconv.ParseFloat(c.DefaultQuery("min_weight", "0.5"), 64)

	similar, err := h.Neo4jClient.GetSimilarNews(c.Request.Context(), newsID, limit, minWeight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"similar_news": similar})
}

// 2. Цепочка цитирований
func (h *Handler) GetCitationChain(c *gin.Context) {
	newsID := c.Param("id")
	minHops, _ := strconv.Atoi(c.DefaultQuery("min_hops", "2"))
	maxHops, _ := strconv.Atoi(c.DefaultQuery("max_hops", "4"))

	chain, err := h.Neo4jClient.GetCitationChain(c.Request.Context(), newsID, minHops, maxHops)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"citation_chain": chain})
}

// 3. Общие соседи (авторы)
func (h *Handler) GetCommonNeighbors(c *gin.Context) {
	author1 := c.Query("author1")
	author2 := c.Query("author2")

	if author1 == "" || author2 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "author1 and author2 required"})
		return
	}

	neighbors, err := h.Neo4jClient.GetCommonNeighbors(c.Request.Context(), author1, author2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"common_neighbors": neighbors})
}

// 4. Топ авторов
func (h *Handler) GetTopAuthors(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	authors, err := h.Neo4jClient.GetTopAuthors(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"top_authors": authors})
}

// 5. Фильтр новостей
func (h *Handler) FilterNews(c *gin.Context) {
	language := c.DefaultQuery("language", "en")
	category := c.Query("category")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))

	fromDate := time.Now().AddDate(0, 0, -days)

	news, err := h.Neo4jClient.FilterNews(c.Request.Context(), language, category, fromDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"filtered_news": news})
}

// 6. Комбинированный запрос (рекомендации)
func (h *Handler) GetComplexRecommendations(c *gin.Context) {
	userID := c.Param("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	recs, err := h.Neo4jClient.GetComplexRecommendations(c.Request.Context(), userID, 2, 4, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}

// 7. Статистика графа
func (h *Handler) GetGraphStats(c *gin.Context) {
	stats, err := h.Neo4jClient.GetGraphStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ============================================
// CLICKHOUSE ЗАПРОСЫ (ЗАДАНИЕ 3)
// ============================================

// Топ новостей по просмотрам
func (h *Handler) GetTopNews(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	topNews, err := h.ClickHouseClient.GetTopNewsByViews(c.Request.Context(), days, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"top_news": topNews})
}

// Популярные теги
func (h *Handler) GetPopularTags(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	tags, err := h.ClickHouseClient.GetPopularTags(c.Request.Context(), hours, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"popular_tags": tags})
}

// Активность пользователей
func (h *Handler) GetUserActivity(c *gin.Context) {
	dateStr := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	date, _ := time.Parse("2006-01-02", dateStr)

	activity, err := h.ClickHouseClient.GetUserActivityByHour(c.Request.Context(), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user_activity": activity})
}

// Статистика по авторам
func (h *Handler) GetAuthorStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	stats, err := h.ClickHouseClient.GetAuthorStats(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"author_stats": stats})
}

// Retention пользователей
func (h *Handler) GetUserRetention(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	retention, err := h.ClickHouseClient.GetUserRetention(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"retention": retention})
}

// Анализ вовлечённости по часам
func (h *Handler) GetEngagementByHour(c *gin.Context) {
	engagement, err := h.ClickHouseClient.GetEngagementByHour(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"engagement_by_hour": engagement})
}

// Лайки по категориям
func (h *Handler) GetLikesByCategory(c *gin.Context) {
	stats, err := h.ClickHouseClient.GetLikesByCategory(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"likes_by_category": stats})
}

// Топ читателей
func (h *Handler) GetTopReaders(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	readers, err := h.ClickHouseClient.GetTopReaders(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"top_readers": readers})
}
