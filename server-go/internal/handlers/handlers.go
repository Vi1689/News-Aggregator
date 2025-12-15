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
<<<<<<< HEAD
    r := mux.NewRouter()

    // Health check endpoint
    r.HandleFunc("/health", h.healthHandler).Methods("GET")

    // Специальные endpoint для VK ресерчера (должны быть ПЕРЕД табличными маршрутами)
    r.HandleFunc("/api/vk/posts", h.createVKPostHandler).Methods("POST")
    r.HandleFunc("/api/vk/sources", h.createVKSourceHandler).Methods("POST")
    r.HandleFunc("/api/vk/channels", h.createVKChannelHandler).Methods("POST")
    r.HandleFunc("/api/vk/media", h.createVKMediaHandler).Methods("POST")
    r.HandleFunc("/api/vk/authors", h.createVKAuthorHandler).Methods("POST")
    r.HandleFunc("/api/vk/comments", h.createVKCommentHandler).Methods("POST")

    // MongoDB endpoints
    r.HandleFunc("/api/mongo/search/advanced", h.advancedSearchHandler).Methods("POST")
    r.HandleFunc("/api/mongo/analytics/top-tags", h.topTagsHandler).Methods("GET")
    r.HandleFunc("/api/mongo/analytics/engagement", h.engagementAnalysisHandler).Methods("GET")
    r.HandleFunc("/api/mongo/user/{user_id}/history", h.userHistoryHandler).Methods("GET")
    r.HandleFunc("/api/mongo/top-posts", h.topPostsViewHandler).Methods("GET")
    r.HandleFunc("/api/mongo/posts/{post_id}/operations", h.postOperationsHandler).Methods("POST")
    r.HandleFunc("/api/mongo/analytics/channels", h.channelPerformanceHandler).Methods("GET")
    r.HandleFunc("/api/mongo/materialize", h.materializeViewHandler).Methods("POST")

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

        r.HandleFunc("/api/authors", h.createHandler).Methods("POST")
    r.HandleFunc("/api/authors", h.readAllHandler).Methods("GET")

    return r
}

// ============ VK RESEARCHER SPECIAL HANDLERS ============

func (h *Handlers) createVKSourceHandler(w http.ResponseWriter, r *http.Request) {
    var data map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    log.Printf("[VK Researcher] Creating source: %+v", data)
    
    // Преобразуем данные от VK
    transformedData := map[string]interface{}{
        "name":    data["name"],
        "address": data["address"],
        "topic":   data["topic"],
    }
    
    // Используем обычный обработчик
    h.createHandlerInternal(w, r, "sources", transformedData)
}

func (h *Handlers) createVKChannelHandler(w http.ResponseWriter, r *http.Request) {
    var data map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    log.Printf("[VK Researcher] Creating channel: %+v", data)
    
    // Преобразуем данные от VK
    transformedData := map[string]interface{}{
        "name":              data["name"],
        "link":              data["link"],
        "subscribers_count": data["subscribers_count"],
        "source_id":         data["source_id"],
        "topic":             data["topic"],
    }
    
    // Используем обычный обработчик
    h.createHandlerInternal(w, r, "channels", transformedData)
}

func (h *Handlers) createVKPostHandler(w http.ResponseWriter, r *http.Request) {
    var data map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    log.Printf("[VK Researcher] Creating post: %+v", data)
    
    // Преобразуем данные от VK в формат сервера
    transformedData := h.transformVKPostData(data)
    
    // Используем существующий обработчик для постов
    h.createPostHandler(w, r, transformedData)
}

func (h *Handlers) createVKMediaHandler(w http.ResponseWriter, r *http.Request) {
    var data map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    log.Printf("[VK Researcher] Creating media: %+v", data)
    
    // Преобразуем данные от VK
    transformedData := map[string]interface{}{
        "post_id":       data["post_id"],
        "media_content": data["media_content"],
        "media_type":    data["media_type"],
    }
    
    // Используем обычный обработчик
    h.createHandlerInternal(w, r, "media", transformedData)
}

func (h *Handlers) createVKAuthorHandler(w http.ResponseWriter, r *http.Request) {
    var data map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    log.Printf("[VK Researcher] Creating author: %+v", data)
    
    // Преобразуем данные от VK
    transformedData := map[string]interface{}{
        "name": data["name"],
    }
    
    // Используем обычный обработчик
    h.createHandlerInternal(w, r, "authors", transformedData)
}

func (h *Handlers) createVKCommentHandler(w http.ResponseWriter, r *http.Request) {
    var data map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    log.Printf("[VK Researcher] Creating comment: %+v", data)
    
    // Преобразуем данные от VK
    transformedData := map[string]interface{}{
        "post_id":           data["post_id"],
        "nickname":          data["nickname"],
        "parent_comment_id": data["parent_comment_id"],
        "text":              data["text"],
        "likes_count":       data["likes_count"],
        "created_at":        data["created_at"],
    }
    
    // Используем обычный обработчик
    h.createHandlerInternal(w, r, "comments", transformedData)
}

func (h *Handlers) transformVKPostData(vkData map[string]interface{}) map[string]interface{} {
    transformed := make(map[string]interface{})
    
    // Маппинг полей VK → сервер
    
    // Title (обязательное поле)
    if title, ok := vkData["title"].(string); ok && title != "" {
        transformed["title"] = title
    } else {
        // Генерируем заголовок из контента
        if text, ok := vkData["text"].(string); ok && len(text) > 0 {
            if len(text) > 50 {
                transformed["title"] = text[:50] + "..."
            } else {
                transformed["title"] = text
            }
        } else {
            transformed["title"] = "VK Post"
        }
    }
    
    // Content (обязательное поле) - VK использует "text", сервер ожидает "content"
    if text, ok := vkData["text"].(string); ok {
        transformed["content"] = text  // Преобразуем text → content
    } else {
        transformed["content"] = ""
    }
    
authorID := 1 // Дефолтный автор ID
    
    // Если VK предоставил author_id, используем его
    if vkAuthorID, ok := vkData["author_id"].(float64); ok && vkAuthorID > 0 {
        authorID = int(vkAuthorID)
    } else if vkAuthorID, ok := vkData["author_id"].(int); ok && vkAuthorID > 0 {
        authorID = vkAuthorID
    }
    // Иначе используем дефолтного автора (ID=1)
    
    transformed["author_id"] = authorID


    
    // Channel ID
    if channelID, ok := vkData["channel_id"].(float64); ok {
        transformed["channel_id"] = int(channelID)
    } else if channelID, ok := vkData["channel_id"].(int); ok {
        transformed["channel_id"] = channelID
    } else {
        transformed["channel_id"] = 1 // Дефолтный канал
    }
    
    // Likes count
    if likes, ok := vkData["likes_count"].(float64); ok {
        transformed["likes_count"] = int(likes)
    } else if likes, ok := vkData["likes_count"].(int); ok {
        transformed["likes_count"] = likes
    } else {
        transformed["likes_count"] = 0
    }
    
    // Comments count
    if comments, ok := vkData["comments_count"].(float64); ok {
        transformed["comments_count"] = int(comments)
    } else if comments, ok := vkData["comments_count"].(int); ok {
        transformed["comments_count"] = comments
    } else {
        transformed["comments_count"] = 0
    }
    
    // Tags (VK ресерчер отправляет массив тегов)
    if tags, ok := vkData["tags"].([]interface{}); ok {
        transformed["tags"] = tags
    } else if tags, ok := vkData["tags"].([]string); ok {
        // Конвертируем []string в []interface{}
        interfaceTags := make([]interface{}, len(tags))
        for i, tag := range tags {
            interfaceTags[i] = tag
        }
        transformed["tags"] = interfaceTags
    }
    
    // Created at (VK использует timestamp, а сервер - datetime string)
    if createdAt, ok := vkData["created_at"].(string); ok {
        transformed["created_at"] = createdAt
    } else if timestamp, ok := vkData["date"].(float64); ok {
        // Конвертируем Unix timestamp в строку
        t := time.Unix(int64(timestamp), 0)
        transformed["created_at"] = t.Format("2006-01-02 15:04:05")
    } else {
        transformed["created_at"] = time.Now().Format("2006-01-02 15:04:05")
    }
    
    log.Printf("[VK Researcher] Transformed post data: %+v", transformed)
    return transformed
}

// Внутренний обработчик создания (без проверки дубликатов для VK)
func (h *Handlers) createHandlerInternal(w http.ResponseWriter, r *http.Request, table string, data map[string]interface{}) {
    if !validTables[table] {
        http.Error(w, "Table not found", http.StatusNotFound)
        return
    }

    if len(data) == 0 {
        http.Error(w, "No fields provided", http.StatusBadRequest)
        return
    }

    // Строим SQL запрос для обычных таблиц
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

    tx, err := conn.Begin(ctx)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer tx.Rollback(ctx)

    // Используем Query вместо QueryRow для получения FieldDescriptions
    rows, err := tx.Query(ctx, query, values...)
    if err != nil {
        log.Printf("[VK Researcher] Error executing query for table %s: %v", table, err)
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
                    if tagStr, ok := tag.(string); ok {
                        tags = append(tags, tagStr)
                    }
                }
            }
            
            go h.mongo.IndexPost(context.Background(), int(postID), title, content, tags)
        }
    }

    // Инвалидация кеша
    h.cache.Del(ctx, "cache:"+table)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
=======
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
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
	// ОСОБАЯ ЛОГИКА ДЛЯ СОЗДАНИЯ ПОСТОВ
	if table == "posts" {
		h.createPostHandler(w, r, data)
		return
	}

	// Проверка дубликатов для posts (для обычных таблиц)
	if table == "posts" && data["content"] != nil {
=======
	// Проверка дубликатов для posts
	if table == "posts" {
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
	// Строим SQL запрос для обычных таблиц
=======
	// Строим SQL запрос
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
=======
	// log.Printf("Using %s for WRITE operation on table: %s",
	// 	map[bool]string{true: "REPLICA", false: "MASTER"}[conn.isReplica], table)

>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
	// Индексация в MongoDB для posts (если это обычный пост)
	if table == "posts" && data["content"] != nil {
=======
	// Индексация в MongoDB для posts
	if table == "posts" {
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
// createPostHandler - специальный обработчик для создания постов
func (h *Handlers) createPostHandler(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
    ctx := r.Context()
    
    // Проверка обязательных полей
    title, hasTitle := data["title"].(string)
    content, hasContent := data["content"].(string)
    if !hasTitle || !hasContent {
        http.Error(w, "Title and content are required", http.StatusBadRequest)
        return
    }
    
    // Проверка дубликатов (только для обычных запросов, не для VK)
    if !strings.Contains(r.URL.Path, "/vk/") {
        hash := fmt.Sprintf("%x", sha256.Sum256([]byte(title+content)))
        isDup, _ := h.mongo.IsDuplicateContent(ctx, hash)
        if isDup {
            http.Error(w, "Duplicate post detected", http.StatusConflict)
            return
        }
    }
    
    authorID, hasAuthor := data["author_id"].(float64)
    if !hasAuthor {
        // Пробуем получить как int
        if authorInt, ok := data["author_id"].(int); ok {
            authorID = float64(authorInt)
            hasAuthor = true
        }
    }
    
    channelID, hasChannel := data["channel_id"].(float64)
    if !hasChannel {
        // Пробуем получить как int
        if channelInt, ok := data["channel_id"].(int); ok {
            channelID = float64(channelInt)
            hasChannel = true
        }
    }
    
    if !hasAuthor || !hasChannel {
        http.Error(w, "Author ID and Channel ID are required", http.StatusBadRequest)
        return
    }

    conn, err := h.pool.Acquire(ctx, false) // Запись - только мастер
    if err != nil {
        http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
        return
    }
    defer conn.Release()

    tx, err := conn.Begin(ctx)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer tx.Rollback(ctx)

    // 1. Вставляем контент в news_texts - ИСПРАВЛЕНО: используем колонку "text" вместо "content"
    textQuery := "INSERT INTO news_texts (text) VALUES ($1) RETURNING text_id"
    var textID int32
    err = tx.QueryRow(ctx, textQuery, content).Scan(&textID)
    if err != nil {
        http.Error(w, "Failed to insert content: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // 2. Вставляем пост в posts
    postQuery := `INSERT INTO posts (title, author_id, text_id, channel_id, comments_count, likes_count, created_at) 
                  VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING post_id, title, author_id, text_id, channel_id, comments_count, likes_count, created_at`
    
    commentsCount := 0
    likesCount := 0
    if cc, ok := data["comments_count"].(float64); ok {
        commentsCount = int(cc)
    } else if cc, ok := data["comments_count"].(int); ok {
        commentsCount = cc
    }
    if lc, ok := data["likes_count"].(float64); ok {
        likesCount = int(lc)
    } else if lc, ok := data["likes_count"].(int); ok {
        likesCount = lc
    }
    
    // Определяем дату создания
    createdAt := time.Now()
    if ca, ok := data["created_at"].(string); ok {
        // Пробуем распарсить строку даты
        if parsedTime, err := time.Parse("2006-01-02 15:04:05", ca); err == nil {
            createdAt = parsedTime
        }
    }
    
    var postID int32
    var resultTitle string
    var resultAuthorID int32
    var resultTextID int32
    var resultChannelID int32
    var resultCommentsCount int32
    var resultLikesCount int32
    var resultCreatedAt time.Time
    
    err = tx.QueryRow(ctx, postQuery, 
        title, 
        int(authorID), 
        textID, 
        int(channelID), 
        commentsCount, 
        likesCount, 
        createdAt,
    ).Scan(
        &postID,
        &resultTitle,
        &resultAuthorID,
        &resultTextID,
        &resultChannelID,
        &resultCommentsCount,
        &resultLikesCount,
        &resultCreatedAt,
    )
    
    if err != nil {
        http.Error(w, "Failed to insert post: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // 3. Обработка тегов
    if tags, ok := data["tags"].([]interface{}); ok && len(tags) > 0 {
        for _, tagValue := range tags {
            if tagName, ok := tagValue.(string); ok && tagName != "" {
                // Проверяем существует ли тег, если нет - создаем
                var tagID int32
                tagCheckQuery := "SELECT tag_id FROM tags WHERE name = $1"
                err := tx.QueryRow(ctx, tagCheckQuery, tagName).Scan(&tagID)
                
                if err != nil {
                    // Тег не существует, создаем новый
                    createTagQuery := "INSERT INTO tags (name) VALUES ($1) RETURNING tag_id"
                    err = tx.QueryRow(ctx, createTagQuery, tagName).Scan(&tagID)
                    if err != nil {
                        log.Printf("Failed to create tag %s: %v", tagName, err)
                        continue
                    }
                }
                
                // Связываем тег с постом
                linkTagQuery := "INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING"
                _, err = tx.Exec(ctx, linkTagQuery, postID, tagID)
                if err != nil {
                    log.Printf("Failed to link tag %s to post %d: %v", tagName, postID, err)
                }
            }
        }
    }

    if err := tx.Commit(ctx); err != nil {
        http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // 4. Индексация в MongoDB
    tags := []string{}
    if t, ok := data["tags"].([]interface{}); ok {
        for _, tag := range t {
            if tagStr, ok := tag.(string); ok {
                tags = append(tags, tagStr)
            }
        }
    }
    
    go h.mongo.IndexPost(context.Background(), int(postID), title, content, tags)

    // 5. Подготовка ответа
    result := map[string]interface{}{
        "post_id":        postID,
        "title":          resultTitle,
        "author_id":      resultAuthorID,
        "text_id":        resultTextID,
        "channel_id":     resultChannelID,
        "comments_count": resultCommentsCount,
        "likes_count":    resultLikesCount,
        "created_at":     resultCreatedAt.Format("2006-01-02 15:04:05"),
        "content":        content,
        "tags":           tags,
    }

    // 6. Инвалидация кеша
    h.cache.Del(ctx, "cache:posts", "cache:news_texts", "cache:tags", "cache:post_tags")

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}

=======
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
	// ОСОБАЯ ЛОГИКА ДЛЯ ПОСТОВ - собираем данные из нескольких таблиц
	if table == "posts" {
		h.readAllPostsHandler(w, r)
		return
	}

=======
>>>>>>> 6f851b4 (mango)
	// Чтение из реплики
	conn, err := h.pool.Acquire(ctx, true)
	if err != nil {
		http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer conn.Release()

<<<<<<< HEAD
=======
	// log.Printf("Using %s for READ operation on table: %s",
	// 	map[bool]string{true: "REPLICA", false: "MASTER"}[conn.isReplica], table)

>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
// readAllPostsHandler - специальный обработчик для чтения всех постов
func (h *Handlers) readAllPostsHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    cacheKey := "cache:posts:full"

    // Проверка кеша
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

    // Собираем полные данные о постах из нескольких таблиц
    query := `
        SELECT 
            p.post_id,
            p.title,
            p.author_id,
            a.name as author_name,
            p.text_id,
            nt.text as content,  -- ИСПРАВЛЕНО: используем nt.text вместо nt.content
            p.channel_id,
            c.name as channel_name,
            p.comments_count,
            p.likes_count,
            p.created_at,
            COALESCE(
                ARRAY_AGG(t.name) FILTER (WHERE t.name IS NOT NULL), 
                '{}'::text[]
            ) as tags
        FROM posts p
        LEFT JOIN authors a ON p.author_id = a.author_id
        LEFT JOIN news_texts nt ON p.text_id = nt.text_id
        LEFT JOIN channels c ON p.channel_id = c.channel_id
        LEFT JOIN post_tags pt ON p.post_id = pt.post_id
        LEFT JOIN tags t ON pt.tag_id = t.tag_id
        GROUP BY p.post_id, a.name, nt.text, c.name
        ORDER BY p.created_at DESC
    `

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

=======
>>>>>>> 6f851b4 (mango)
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
<<<<<<< HEAD
	defer conn.Release()
=======
		defer conn.Release()
>>>>>>> 6f851b4 (mango)

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

<<<<<<< HEAD
	// ОСОБАЯ ЛОГИКА ДЛЯ ПОСТОВ
	if table == "posts" {
		h.readOnePostHandler(w, r, id)
		return
	}

=======
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
// readOnePostHandler - специальный обработчик для чтения одного поста
func (h *Handlers) readOnePostHandler(w http.ResponseWriter, r *http.Request, id string) {
    ctx := r.Context()
    cacheKey := fmt.Sprintf("cache:posts:full:%s", id)

    // Проверка кеша
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

    // Собираем полные данные о посте из нескольких таблиц
    query := `
        SELECT 
            p.post_id,
            p.title,
            p.author_id,
            a.name as author_name,
            p.text_id,
            nt.text as content,  -- ИСПРАВЛЕНО: используем nt.text вместо nt.content
            p.channel_id,
            c.name as channel_name,
            p.comments_count,
            p.likes_count,
            p.created_at,
            COALESCE(
                ARRAY_AGG(t.name) FILTER (WHERE t.name IS NOT NULL), 
                '{}'::text[]
            ) as tags
        FROM posts p
        LEFT JOIN authors a ON p.author_id = a.author_id
        LEFT JOIN news_texts nt ON p.text_id = nt.text_id
        LEFT JOIN channels c ON p.channel_id = c.channel_id
        LEFT JOIN post_tags pt ON p.post_id = pt.post_id
        LEFT JOIN tags t ON pt.tag_id = t.tag_id
        WHERE p.post_id = $1
        GROUP BY p.post_id, a.name, nt.text, c.name
    `

    rows, err := conn.Query(ctx, query, id)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    results := h.rowsToJSON(rows)
    
    if len(results) == 0 {
        http.Error(w, "Post not found", http.StatusNotFound)
        return
    }

    data, _ := json.Marshal(results[0])
    h.cache.SetEX(ctx, cacheKey, string(data), 600)

    w.Header().Set("Content-Type", "application/json")
    w.Write(data)
}

=======
>>>>>>> 6f851b4 (mango)
func (h *Handlers) updateHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	id := vars["id"]

	if !validTables[table] {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

<<<<<<< HEAD
	// ОСОБАЯ ЛОГИКА ДЛЯ ПОСТОВ
	if table == "posts" {
		h.updatePostHandler(w, r, id)
		return
	}

=======
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
// updatePostHandler - специальный обработчик для обновления постов
func (h *Handlers) updatePostHandler(w http.ResponseWriter, r *http.Request, id string) {
    ctx := r.Context()
    postID, err := strconv.Atoi(id)
    if err != nil {
        http.Error(w, "Invalid post ID", http.StatusBadRequest)
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

    conn, err := h.pool.Acquire(ctx, false) // Запись - только мастер
    if err != nil {
        http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
        return
    }
    defer conn.Release()

    tx, err := conn.Begin(ctx)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer tx.Rollback(ctx)

    // 1. Получаем текущий text_id поста
    var currentTextID int32
    err = tx.QueryRow(ctx, "SELECT text_id FROM posts WHERE post_id = $1", postID).Scan(&currentTextID)
    if err != nil {
        http.Error(w, "Post not found", http.StatusNotFound)
        return
    }

    // 2. Обновляем контент если есть
    contentUpdated := false
    newTitle := ""
    newContent := ""
    
    if content, ok := data["content"].(string); ok && content != "" {
        // ИСПРАВЛЕНО: обновляем колонку "text" вместо "content"
        updateContentQuery := "UPDATE news_texts SET text = $1 WHERE text_id = $2"
        _, err = tx.Exec(ctx, updateContentQuery, content, currentTextID)
        if err != nil {
            http.Error(w, "Failed to update content: "+err.Error(), http.StatusInternalServerError)
            return
        }
        contentUpdated = true
        newContent = content
    }

    // 3. Обновляем основную информацию о посте
    updates := []string{}
    values := []interface{}{}
    paramCount := 1

    for key, value := range data {
        if key == "content" || key == "tags" {
            continue // Эти поля обрабатываются отдельно
        }
        
        updates = append(updates, fmt.Sprintf("%s = $%d", key, paramCount))
        values = append(values, value)
        paramCount++
        
        if key == "title" {
            if title, ok := value.(string); ok {
                newTitle = title
            }
        }
    }

    if len(updates) > 0 {
        values = append(values, postID)
        updatePostQuery := fmt.Sprintf("UPDATE posts SET %s WHERE post_id = $%d", 
            strings.Join(updates, ", "), paramCount)
        
        _, err = tx.Exec(ctx, updatePostQuery, values...)
        if err != nil {
            http.Error(w, "Failed to update post: "+err.Error(), http.StatusInternalServerError)
            return
        }
    }

    // 4. Обновляем теги если есть
    newTags := []string{}
    if tags, ok := data["tags"].([]interface{}); ok {
        // Удаляем старые теги
        deleteTagsQuery := "DELETE FROM post_tags WHERE post_id = $1"
        _, err = tx.Exec(ctx, deleteTagsQuery, postID)
        if err != nil {
            http.Error(w, "Failed to clear old tags: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // Добавляем новые теги
        for _, tagValue := range tags {
            if tagName, ok := tagValue.(string); ok && tagName != "" {
                newTags = append(newTags, tagName)
                
                // Проверяем существует ли тег
                var tagID int32
                tagCheckQuery := "SELECT tag_id FROM tags WHERE name = $1"
                err := tx.QueryRow(ctx, tagCheckQuery, tagName).Scan(&tagID)
                
                if err != nil {
                    // Тег не существует, создаем новый
                    createTagQuery := "INSERT INTO tags (name) VALUES ($1) RETURNING tag_id"
                    err = tx.QueryRow(ctx, createTagQuery, tagName).Scan(&tagID)
                    if err != nil {
                        log.Printf("Failed to create tag %s: %v", tagName, err)
                        continue
                    }
                }
                
                // Связываем тег с постом
                linkTagQuery := "INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2)"
                _, err = tx.Exec(ctx, linkTagQuery, postID, tagID)
                if err != nil {
                    log.Printf("Failed to link tag %s to post %d: %v", tagName, postID, err)
                }
            }
        }
    }

    if err := tx.Commit(ctx); err != nil {
        http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // 5. Обновление в MongoDB если нужно
    if contentUpdated || newTitle != "" || len(newTags) > 0 {
        go h.mongo.UpdatePostIndex(context.Background(), postID, newTitle, newContent, newTags)
    }

    // 6. Инвалидация кеша
    h.cache.Del(ctx, 
        "cache:posts", 
        "cache:posts:full", 
        fmt.Sprintf("cache:posts:%d", postID),
        fmt.Sprintf("cache:posts:full:%d", postID),
        "cache:news_texts",
        "cache:tags",
        "cache:post_tags",
    )

    w.Write([]byte("Post updated successfully\n"))
}

=======
>>>>>>> 6f851b4 (mango)
func (h *Handlers) deleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	id := vars["id"]

	if !validTables[table] {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

<<<<<<< HEAD
	// ОСОБАЯ ЛОГИКА ДЛЯ ПОСТОВ
	if table == "posts" {
		h.deletePostHandler(w, r, id)
		return
	}

=======
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
// deletePostHandler - специальный обработчик для удаления постов
func (h *Handlers) deletePostHandler(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()
	postID, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	conn, err := h.pool.Acquire(ctx, false) // Запись - только мастер
	if err != nil {
		http.Error(w, "Database temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	// 1. Получаем text_id поста
	var textID int32
	err = tx.QueryRow(ctx, "SELECT text_id FROM posts WHERE post_id = $1", postID).Scan(&textID)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// 2. Удаляем связи с тегами
	deleteTagsQuery := "DELETE FROM post_tags WHERE post_id = $1"
	_, err = tx.Exec(ctx, deleteTagsQuery, postID)
	if err != nil {
		http.Error(w, "Failed to delete tag associations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Удаляем пост
	deletePostQuery := "DELETE FROM posts WHERE post_id = $1"
	_, err = tx.Exec(ctx, deletePostQuery, postID)
	if err != nil {
		http.Error(w, "Failed to delete post: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Удаляем контент (оставляем на случай если используется другими постами)
	// deleteContentQuery := "DELETE FROM news_texts WHERE text_id = $1"
	// _, err = tx.Exec(ctx, deleteContentQuery, textID)
	// if err != nil {
	//     http.Error(w, "Failed to delete content: "+err.Error(), http.StatusInternalServerError)
	//     return
	// }

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Удаление из MongoDB
	go h.mongo.RemovePostIndex(context.Background(), postID)

	// 6. Инвалидация кеша
	h.cache.Del(ctx, 
		"cache:posts", 
		"cache:posts:full", 
		fmt.Sprintf("cache:posts:%d", postID),
		fmt.Sprintf("cache:posts:full:%d", postID),
		"cache:news_texts",
		"cache:tags",
		"cache:post_tags",
	)

	w.Write([]byte("Post deleted successfully\n"))
}

=======
>>>>>>> 6f851b4 (mango)
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
<<<<<<< HEAD
    vars := mux.Vars(r)
    userID := vars["user_id"]

    limit := 50
    if l := r.URL.Query().Get("limit"); l != "" {
        limit, _ = strconv.Atoi(l)
    }

    ctx := r.Context()
    cacheKey := fmt.Sprintf("user_history:%s:%d", userID, limit)

    // Увеличиваем TTL для часто запрашиваемых данных
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
    
    // Динамический TTL: чем больше результатов, тем дольше кэш
    ttl := 300 // 5 минут по умолчанию
    if len(results) > 0 {
        ttl = 600 // 10 минут для непустых результатов
    }
    
    h.cache.SetEX(ctx, cacheKey, string(data), ttl)

    w.Header().Set("Content-Type", "application/json")
    w.Write(data)
=======
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
>>>>>>> 6f851b4 (mango)
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

<<<<<<< HEAD
func mustMarshal(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
=======
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
>>>>>>> 6f851b4 (mango)
