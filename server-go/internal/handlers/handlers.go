package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"news-aggregator/internal/cache"
	"news-aggregator/internal/mongo"
	"news-aggregator/internal/pgpool"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
)

type Handlers struct {
	pool  *pgpool.PgPool
	cache *cache.CacheManager
	mongo *mongo.MongoManager
}

var validTables = map[string]bool{
	"users":                        true,
	"authors":                      true,
	"news_texts":                   true,
	"sources":                      true,
	"channels":                     true,
	"posts":                        true,
	"media":                        true,
	"tags":                         true,
	"post_tags":                    true,
	"comments":                     true,
	"channel_activity_stats":       true,
	"author_performance":           true,
	"tag_popularity_detailed":      true,
	"source_post_stats":            true,
	"user_comment_activity":        true,
	"posts_ranked_by_popularity":   true,
	"author_likes_trend":           true,
	"cumulative_posts_analysis":    true,
	"tag_rank_by_channel":          true,
	"commenter_analysis":           true,
	"posts_with_detailed_authors":  true,
	"channels_with_sources":        true,
	"posts_with_authors_and_texts": true,
	"comments_with_post_info":      true,
	"posts_with_tags_and_channels": true,
	"media_with_context":           true,
	"comprehensive_post_info":      true,
	"extended_post_analytics":      true,
}

var pkMap = map[string]string{
	"users":      "user_id",
	"authors":    "author_id",
	"news_texts": "text_id",
	"sources":    "source_id",
	"channels":   "channel_id",
	"posts":      "post_id",
	"media":      "media_id",
	"tags":       "tag_id",
	"comments":   "comment_id",
}

func NewHandlers(pool *pgpool.PgPool, cache *cache.CacheManager, mongo *mongo.MongoManager) *Handlers {
	return &Handlers{
		pool:  pool,
		cache: cache,
		mongo: mongo,
	}
}

func (h *Handlers) SetupRoutes() http.Handler {
	r := mux.NewRouter()

	// Health check endpoint
	r.HandleFunc("/health", h.healthHandler).Methods("GET")

	// CRUD операции для PostgreSQL
	r.HandleFunc("/api/{table}", h.createHandler).Methods("POST")
	r.HandleFunc("/api/{table}", h.readAllHandler).Methods("GET")
	r.HandleFunc("/api/{table}/{id}", h.readOneHandler).Methods("GET")
	r.HandleFunc("/api/{table}/{id}", h.updateHandler).Methods("PUT")
	r.HandleFunc("/api/{table}/{id}", h.deleteHandler).Methods("DELETE")

	// Обработка post_tags с двумя ID
	r.HandleFunc("/api/{table}/{id}/{id2}", h.readOneHandler).Methods("GET")
	r.HandleFunc("/api/{table}/{id}/{id2}", h.updateHandler).Methods("PUT")
	r.HandleFunc("/api/{table}/{id}/{id2}", h.deleteHandler).Methods("DELETE")

	// MongoDB endpoints
	r.HandleFunc("/api/mongo/search/advanced", h.advancedSearchHandler).Methods("POST")
	r.HandleFunc("/api/mongo/analytics/top-tags", h.topTagsHandler).Methods("GET")
	r.HandleFunc("/api/mongo/analytics/engagement", h.engagementAnalysisHandler).Methods("GET")
	r.HandleFunc("/api/mongo/user/{user_id}/history", h.userHistoryHandler).Methods("GET")
	r.HandleFunc("/api/mongo/top-posts", h.topPostsViewHandler).Methods("GET")
	r.HandleFunc("/api/mongo/posts/{post_id}/operations", h.postOperationsHandler).Methods("POST")
	r.HandleFunc("/api/mongo/analytics/channels", h.channelPerformanceHandler).Methods("GET")
	r.HandleFunc("/api/mongo/materialize", h.materializeViewHandler).Methods("POST")

	return r
}

func (h *Handlers) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// ============ CRUD HANDLERS ============

func (h *Handlers) createHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]

	if !validTables[table] {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Проверка дубликатов для posts
	if table == "posts" {
		if title, ok := data["title"].(string); ok {
			if content, ok := data["content"].(string); ok {
				hash := fmt.Sprintf("%x", sha256.Sum256([]byte(title+content)))
				ctx := r.Context()
				isDup, _ := h.mongo.IsDuplicateContent(ctx, hash)
				if isDup {
					http.Error(w, "Duplicate post detected", http.StatusConflict)
					return
				}
			}
		}
	}

	if len(data) == 0 {
		http.Error(w, "No fields provided", http.StatusBadRequest)
		return
	}

	// Строим SQL запрос
	cols := []string{}
	placeholders := []string{}
	values := []interface{}{}
	i := 1

	for key, value := range data {
		cols = append(cols, key)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, value)
		i++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	ctx := r.Context()
	conn, err := h.pool.Acquire(ctx, false) // Запись - только мастер
	if err != nil {
		http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer conn.Release()

	// log.Printf("Using %s for WRITE operation on table: %s",
	// 	map[bool]string{true: "REPLICA", false: "MASTER"}[conn.isReplica], table)

	tx, err := conn.Begin(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	// Используем Query вместо QueryRow для получения FieldDescriptions
	rows, err := tx.Query(ctx, query, values...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := make(map[string]interface{})

	if rows.Next() {
		fields := rows.FieldDescriptions()
		values := make([]interface{}, len(fields))
		valuePtrs := make([]interface{}, len(fields))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for i, field := range fields {
			result[string(field.Name)] = values[i]
		}
	}

	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Индексация в MongoDB для posts
	if table == "posts" {
		if postID, ok := result["post_id"].(int32); ok {
			title := data["title"].(string)
			content := data["content"].(string)
			tags := []string{}
			if t, ok := data["tags"].([]interface{}); ok {
				for _, tag := range t {
					tags = append(tags, tag.(string))
				}
			}
			go h.mongo.IndexPost(context.Background(), int(postID), title, content, tags)
		}
	}

	// Инвалидация кеша
	h.cache.Del(ctx, "cache:"+table)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handlers) readAllHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]

	if !validTables[table] {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()
	cacheKey := "cache:" + table

	// Проверка кеша
	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	// Чтение из реплики
	conn, err := h.pool.Acquire(ctx, true)
	if err != nil {
		http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer conn.Release()

	// log.Printf("Using %s for READ operation on table: %s",
	// 	map[bool]string{true: "REPLICA", false: "MASTER"}[conn.isReplica], table)

	query := fmt.Sprintf("SELECT * FROM %s", table)
	rows, err := conn.Query(ctx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := h.rowsToJSON(rows)

	data, _ := json.Marshal(results)
	h.cache.SetEX(ctx, cacheKey, string(data), 300) // TTL 5 минут

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handlers) readOneHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	id := vars["id"]
	id2 := vars["id2"]

	if !validTables[table] {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()

	// Обработка post_tags
	if table == "post_tags" && id2 != "" {
		cacheKey := fmt.Sprintf("cache:post_tags:%s:%s", id, id2)
		if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(cached))
			return
		}

		conn, err := h.pool.Acquire(ctx, true)
		if err != nil {
			http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		defer conn.Release()

		query := fmt.Sprintf("SELECT * FROM %s WHERE post_id=$1 AND tag_id=$2", table)
		rows, err := conn.Query(ctx, query, id, id2)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := h.rowsToJSON(rows)
		data, _ := json.Marshal(results)
		h.cache.SetEX(ctx, cacheKey, string(data), 600)

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}

	// Обычное чтение по PK
	pk, ok := pkMap[table]
	if !ok {
		http.Error(w, "Table has no simple PK", http.StatusBadRequest)
		return
	}

	cacheKey := fmt.Sprintf("cache:%s:%s", table, id)
	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	conn, err := h.pool.Acquire(ctx, true)
	if err != nil {
		http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer conn.Release()

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", table, pk)
	rows, err := conn.Query(ctx, query, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := h.rowsToJSON(rows)
	data, _ := json.Marshal(results)
	h.cache.SetEX(ctx, cacheKey, string(data), 600)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handlers) updateHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	id := vars["id"]

	if !validTables[table] {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	pk, ok := pkMap[table]
	if !ok {
		http.Error(w, "Table has no simple PK", http.StatusBadRequest)
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(data) == 0 {
		http.Error(w, "No fields provided", http.StatusBadRequest)
		return
	}

	// Строим UPDATE запрос
	sets := []string{}
	values := []interface{}{}
	i := 1

	for key, value := range data {
		sets = append(sets, fmt.Sprintf("%s = $%d", key, i))
		values = append(values, value)
		i++
	}
	values = append(values, id)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d",
		table, strings.Join(sets, ", "), pk, i)

	ctx := r.Context()
	conn, err := h.pool.Acquire(ctx, false) // Запись - только мастер
	if err != nil {
		http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer conn.Release()

	if err := conn.Exec(ctx, query, values...); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Обновление в MongoDB для posts
	if table == "posts" {
		postID, _ := strconv.Atoi(id)
		title := ""
		content := ""
		tags := []string{}

		if t, ok := data["title"].(string); ok {
			title = t
		}
		if c, ok := data["content"].(string); ok {
			content = c
		}
		if t, ok := data["tags"].([]interface{}); ok {
			for _, tag := range t {
				tags = append(tags, tag.(string))
			}
		}

		if title != "" || content != "" || len(tags) > 0 {
			go h.mongo.UpdatePostIndex(context.Background(), postID, title, content, tags)
		}
	}

	// Инвалидация кеша
	h.cache.Del(ctx, "cache:"+table, "cache:"+table+":"+id)

	w.Write([]byte("Item updated\n"))
}

func (h *Handlers) deleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	id := vars["id"]

	if !validTables[table] {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	// Удаление из MongoDB для posts
	if table == "posts" {
		postID, _ := strconv.Atoi(id)
		go h.mongo.RemovePostIndex(context.Background(), postID)
	}

	pk, ok := pkMap[table]
	if !ok {
		http.Error(w, "Table has no simple PK", http.StatusBadRequest)
		return
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", table, pk)

	ctx := r.Context()
	conn, err := h.pool.Acquire(ctx, false) // Запись - только мастер
	if err != nil {
		http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer conn.Release()

	if err := conn.Exec(ctx, query, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Инвалидация кеша
	h.cache.Del(ctx, "cache:"+table, "cache:"+table+":"+id)

	w.Write([]byte("Item deleted\n"))
}

// ============ MONGODB HANDLERS ============

func (h *Handlers) advancedSearchHandler(w http.ResponseWriter, r *http.Request) {
	var filters map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&filters); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	cacheKey := "advanced_search:" + string(mustMarshal(filters))

	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	results, err := h.mongo.AdvancedSearch(ctx, filters, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := mustMarshal(results)
	h.cache.SetEX(ctx, cacheKey, string(data), 300)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handlers) topTagsHandler(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	ctx := r.Context()
	cacheKey := fmt.Sprintf("cache:top_tags:%d", limit)

	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	results, err := h.mongo.GetTopTags(ctx, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := mustMarshal(results)
	h.cache.SetEX(ctx, cacheKey, string(data), 600)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handlers) engagementAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		days, _ = strconv.Atoi(d)
	}

	ctx := r.Context()
	cacheKey := fmt.Sprintf("cache:engagement:%d", days)

	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	results, err := h.mongo.GetPostEngagementAnalysis(ctx, days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := mustMarshal(results)
	h.cache.SetEX(ctx, cacheKey, string(data), 300)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handlers) userHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	ctx := r.Context()
	cacheKey := fmt.Sprintf("user_history:%s:%d", userID, limit)

	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	results, err := h.mongo.GetUserHistory(ctx, userID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := mustMarshal(results)
	h.cache.SetEX(ctx, cacheKey, string(data), 300)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handlers) topPostsViewHandler(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	ctx := r.Context()
	cacheKey := fmt.Sprintf("cache:top_posts_view:%d", limit)

	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	results, err := h.mongo.GetTopPostsFromView(ctx, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := mustMarshal(results)
	h.cache.SetEX(ctx, cacheKey, string(data), 120)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handlers) postOperationsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	postID, _ := strconv.Atoi(vars["post_id"])

	var operations map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&operations); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	operationType, _ := operations["operation"].(string)

	switch operationType {
	case "increment_views":
		h.mongo.IncrementViewCount(ctx, postID)
		json.NewEncoder(w).Encode(map[string]string{"message": "Views incremented"})

	case "add_tag":
		tag, _ := operations["tag"].(string)
		h.mongo.AddTagToPost(ctx, postID, tag)
		json.NewEncoder(w).Encode(map[string]string{"message": "Tag added"})

	case "remove_tag":
		tag, _ := operations["tag"].(string)
		h.mongo.RemoveTagFromPost(ctx, postID, tag)
		json.NewEncoder(w).Encode(map[string]string{"message": "Tag removed"})

	case "update_stats":
		likesDelta := int(operations["likes_delta"].(float64))
		commentsDelta := int(operations["comments_delta"].(float64))
		h.mongo.UpdatePostStats(ctx, postID, likesDelta, commentsDelta)
		json.NewEncoder(w).Encode(map[string]string{"message": "Stats updated"})

	case "upsert":
		data := operations["data"].(map[string]interface{})
		wasInserted, _ := h.mongo.UpsertPost(ctx, postID, data)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      map[bool]string{true: "Post created", false: "Post updated"}[wasInserted],
			"was_inserted": wasInserted,
		})

	default:
		http.Error(w, "Unknown operation type", http.StatusBadRequest)
		return
	}

	// Инвалидация кеша
	h.cache.Del(ctx, fmt.Sprintf("cache:posts:%d", postID))
}

func (h *Handlers) channelPerformanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cacheKey := "cache:channel_performance"

	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
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
	w.Write(data)
}

func (h *Handlers) materializeViewHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.mongo.MaterializeTopPostsView(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Инвалидация кеша витрины
	h.cache.DelPattern(ctx, "cache:top_posts_view:*")

	response := map[string]interface{}{
		"message":   "View materialized successfully",
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ============ HELPER FUNCTIONS ============

func (h *Handlers) rowsToJSON(rows pgx.Rows) []map[string]interface{} {
	results := []map[string]interface{}{}
	fields := rows.FieldDescriptions()

	for rows.Next() {
		values := make([]interface{}, len(fields))
		valuePtrs := make([]interface{}, len(fields))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		item := make(map[string]interface{})
		for i, field := range fields {
			item[string(field.Name)] = values[i]
		}
		results = append(results, item)
	}
	return results
}

func (h *Handlers) rowToJSON(rows pgx.Rows) (map[string]interface{}, error) {
	fields := rows.FieldDescriptions()

	if !rows.Next() {
		return nil, fmt.Errorf("no rows returned")
	}

	values := make([]interface{}, len(fields))
	valuePtrs := make([]interface{}, len(fields))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for i, field := range fields {
		result[string(field.Name)] = values[i]
	}

	return result, nil
}

func mustMarshal(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
