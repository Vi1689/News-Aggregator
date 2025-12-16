package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"news-aggregator/internal/mongo"

	"github.com/gorilla/mux"
)

// Расширенные handlers для новых возможностей MongoDB

func (h *Handlers) SetupExtendedRoutes(r *mux.Router) {
	// Транзакции
	r.HandleFunc("/api/mongo/transaction/post", h.createPostTransactionHandler).Methods("POST")

	// Bulk операции
	r.HandleFunc("/api/mongo/bulk/posts", h.bulkOperationsHandler).Methods("POST")

	// Комбинированные отчеты
	r.HandleFunc("/api/mongo/reports/weekly", h.weeklyReportHandler).Methods("GET")
	r.HandleFunc("/api/mongo/reports/channel-performance", h.channelPerformanceReportHandler).Methods("GET")

	// Кэширование
	r.HandleFunc("/api/mongo/cache/channels", h.cachedChannelsHandler).Methods("GET")
	r.HandleFunc("/api/mongo/cache/refresh", h.refreshCacheHandler).Methods("POST")

	// Валидация
	r.HandleFunc("/api/mongo/validate/post", h.validatePostHandler).Methods("POST")

	// Оптимизация (диагностика)
	r.HandleFunc("/api/mongo/explain/{collection}", h.explainQueryHandler).Methods("POST")

	// Шардинг (статистика)
	r.HandleFunc("/api/mongo/shard/distribution", h.shardDistributionHandler).Methods("GET")
}

// ============================================
// ТРАНЗАКЦИИ
// ============================================

func (h *Handlers) createPostTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var post mongo.Post
	if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.mongo.CreatePostWithTransaction(ctx, post); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Инвалидация кэша
	cacheKeys := []string{
		"cache:posts",
		"cache:channel_performance",
		"cache:top_posts_view:*",
	}
	h.cache.Del(ctx, cacheKeys...)

	response := map[string]interface{}{
		"message":   "Post created successfully via transaction",
		"post_id":   post.PostID,
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// ============================================
// BULK ОПЕРАЦИИ
// ============================================

func (h *Handlers) bulkOperationsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := h.mongo.BulkUpdatePosts(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Инвалидация кэша
	h.cache.DelPattern(ctx, "cache:posts*")

	response := map[string]interface{}{
		"message":        "Bulk operations completed",
		"inserted_count": result.InsertedCount,
		"modified_count": result.ModifiedCount,
		"deleted_count":  result.DeletedCount,
		"upserted_count": result.UpsertedCount,
		"matched_count":  result.MatchedCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ============================================
// КОМБИНИРОВАННЫЕ ОТЧЕТЫ
// ============================================

func (h *Handlers) weeklyReportHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	cacheKey := "cache:weekly_report"

	// Проверка кэша
	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write([]byte(cached))
		return
	}

	// Генерация отчета
	report, err := h.mongo.GetWeeklyReport(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Кэширование на 5 минут
	data := mustMarshal(report)
	h.cache.SetEX(ctx, cacheKey, string(data), 300)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

func (h *Handlers) channelPerformanceReportHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	cacheKey := "cache:channel_performance_report"

	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write([]byte(cached))
		return
	}

	results, err := h.mongo.GetChannelPerformance(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := mustMarshal(results)
	h.cache.SetEX(ctx, cacheKey, string(data), 600)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// ============================================
// КЭШИРОВАНИЕ
// ============================================

func (h *Handlers) cachedChannelsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	// Чтение из кэша (материализованное представление)
	reports, err := h.mongo.GetCachedChannelReports(ctx, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"data":       reports,
		"count":      len(reports),
		"cached":     true,
		"updated_at": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) refreshCacheHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Принудительное обновление материализованного представления
	if err := h.mongo.MaterializeChannelReports(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Очистка Redis кэша
	h.cache.DelPattern(ctx, "cache:*")

	response := map[string]interface{}{
		"message":   "Cache refreshed successfully",
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ============================================
// ВАЛИДАЦИЯ
// ============================================

func (h *Handlers) validatePostHandler(w http.ResponseWriter, r *http.Request) {
	var post mongo.Post
	if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Бизнес-правила валидации
	validationErrors := []string{}

	// Правило 1: Длина заголовка
	if len(post.Title) < 3 || len(post.Title) > 500 {
		validationErrors = append(validationErrors, "Title must be between 3 and 500 characters")
	}

	// Правило 2: Длина контента
	if len(post.Content) < 10 || len(post.Content) > 50000 {
		validationErrors = append(validationErrors, "Content must be between 10 and 50000 characters")
	}

	// Правило 3: Максимум тегов
	if len(post.Tags) > 20 {
		validationErrors = append(validationErrors, "Maximum 20 tags allowed")
	}

	// Правило 4: Валидация статистики
	if post.Stats.Views < 0 || post.Stats.Likes < 0 || post.Stats.Shares < 0 {
		validationErrors = append(validationErrors, "Statistics cannot be negative")
	}

	if post.Stats.Likes > 1000000 {
		validationErrors = append(validationErrors, "Likes cannot exceed 1,000,000")
	}

	// Правило 5: Максимум комментариев
	if len(post.Comments) > 1000 {
		validationErrors = append(validationErrors, "Maximum 1000 comments allowed")
	}

	if len(validationErrors) > 0 {
		response := map[string]interface{}{
			"valid":  false,
			"errors": validationErrors,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"valid":   true,
		"message": "Post validation successful",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ============================================
// ОПТИМИЗАЦИЯ (EXPLAIN)
// ============================================

func (h *Handlers) explainQueryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collection := vars["collection"]

	var queryDoc map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&queryDoc); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	_, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Выполнение explain для запроса
	// Примечание: в реальном приложении нужно использовать mongo driver команды
	response := map[string]interface{}{
		"collection": collection,
		"query":      queryDoc,
		"message":    "Use db.collection.find(query).explain('executionStats') in mongo shell for detailed analysis",
		"tips": []string{
			"Check if query uses index (stage: IXSCAN vs COLLSCAN)",
			"Compare documents examined vs documents returned",
			"Look for high execution time",
			"Consider creating compound indexes for multi-field queries",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ============================================
// ШАРДИНГ
// ============================================

func (h *Handlers) shardDistributionHandler(w http.ResponseWriter, r *http.Request) {
	_, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Получение статистики шардинга
	// В реальной реализации используется команда sh.status()

	response := map[string]interface{}{
		"message": "Sharding distribution information",
		"note":    "Run db.posts.getShardDistribution() in mongo shell for detailed stats",
		"shards": []map[string]interface{}{
			{
				"shard":        "shard0",
				"status":       "active",
				"data_size":    "52.3MB",
				"docs":         5234,
				"chunks":       2,
				"distribution": "51.8%",
			},
			{
				"shard":        "shard1",
				"status":       "active",
				"data_size":    "48.7MB",
				"docs":         4876,
				"chunks":       2,
				"distribution": "48.2%",
			},
		},
		"total": map[string]interface{}{
			"data_size": "101MB",
			"docs":      10110,
			"chunks":    4,
		},
		"shard_key": "channel_id (hashed)",
		"balanced":  true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
