package sendRequests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"researcher-vk/internal/vk"
	"strconv"
	"strings"
	"time"
)

// Базовый URL сервера
const baseURL = "http://server:8080/api" 

func init() {
    if err := InitLogger("/var/log/vk-researcher"); err != nil {
        log.Printf("Failed to init logger: %v", err)
    }
}

// ============ ОСНОВНЫЕ ФУНКЦИИ ============

// Добавление источника VK
func AddVKSource() (int, error) {
    fmt.Println("DEBUG: Adding VK source...")
    
    data := map[string]interface{}{
        "name":    "VK",
        "address": "vk.com",
        "topic":   "social",
    }
    return postRequest("/vk/sources", data, "source_id")
}

// Добавление группы VK
func AddVKChannel(group vk.VKGroup, sourceID int) (int, error) {
    fmt.Printf("DEBUG: Adding channel for group %s (sourceID: %d)\n", group.Name, sourceID)
    
    data := map[string]interface{}{
        "name":              group.Name,
        "link":              fmt.Sprintf("https://vk.com/%s", group.ScreenName),
        "subscribers_count": group.MembersCount,
        "source_id":         sourceID,
        "topic":             "general",
    }
    return postRequest("/vk/channels", data, "channel_id")
}

// Добавление автора
func AddVKAuthor(name string) (int, error) {
    fmt.Printf("DEBUG AddVKAuthor called: name='%s'\n", name)
    
    if len(name) == 0 {
        fmt.Printf("WARNING: Empty author name, using default author ID 1\n")
        return 1, nil // Дефолтный автор
    }

    // Проверяем, нет ли уже такого автора
    existingID, err := findExistingAuthor(name)
    if err == nil && existingID > 0 {
        fmt.Printf("DEBUG: Author '%s' already exists with ID: %d\n", name, existingID)
        return existingID, nil
    }
    
    // data := map[string]interface{}{
    //     "name": name,
    // }
    
    // authorID, err := postRequest("/vk/authors", data, "author_id")
    // if err != nil {
    //     // Пробуем обычный endpoint
    //     authorID, err = postRequest("/api/authors", data, "author_id")
    //     if err != nil {
    //         return 0, fmt.Errorf("failed to create author via both endpoints: %v", err)
    //     }
    // }
    
    fmt.Printf("WARNING: Failed to create author '%s', using default author ID 1\n", name)
    return 1, nil
}

// Поиск существующего автора
func findExistingAuthor(name string) (int, error) {
    // Пробуем получить список авторов
    resp, err := http.Get(baseURL + "/api/authors")
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return 0, err
    }
    
    var authors []map[string]interface{}
    if err := json.Unmarshal(body, &authors); err != nil {
        return 0, err
    }
    
    for _, author := range authors {
        if authorName, ok := author["name"].(string); ok && authorName == name {
            if id, ok := author["author_id"].(float64); ok {
                return int(id), nil
            }
        }
    }
    
    return 0, fmt.Errorf("author not found")
}

// Добавление поста VK
func AddVKPost(post vk.VKPost, channelID int, authorID int, groupName string) (int, error) {
    fmt.Printf("DEBUG AddVKPost called: post.ID=%d, channelID=%d, authorID=%d, text length=%d\n", 
        post.ID, channelID, authorID, len(post.Text))
    
    // Проверка даты
    if post.Date <= 0 {
        post.Date = time.Now().Unix()
    }
    
    t := time.Unix(post.Date, 0)
    timeStampString := t.Format("2006-01-02 15:04:05")
    
    // Извлечь теги из текста
    tags := extractTags(post.Text)
    
    // Генерируем заголовок если его нет
    title := post.Text
    if len(title) > 100 {
        title = title[:100] + "..."
    }
    if title == "" {
        title = fmt.Sprintf("Post %d from %s", post.ID, groupName)
    }
    
    fmt.Printf("DEBUG: Post date: %s, Title: %s\n", timeStampString, title)
    
    // Подготавливаем данные
    data := map[string]interface{}{
        "title":           title,
        "text":            post.Text,
        "author_id":       authorID,
        "channel_id":      channelID,
        "comments_count":  post.Comments,
        "likes_count":     post.Likes,
        "created_at":      timeStampString,
        "tags":            tags,
    }
    
    fmt.Printf("DEBUG: Data for post %d: title=%s, author_id=%d, channel_id=%d\n",
        post.ID, data["title"], authorID, channelID)
    
    // Добавить пост
    postID, err := postRequest("/vk/posts", data, "post_id")
    
    if err != nil {
        fmt.Printf("ERROR in AddVKPost for post %d: %v\n", post.ID, err)
        // Пробуем обычный endpoint
        postID, err = postRequest("/api/posts", data, "post_id")
        if err != nil {
            fmt.Printf("FATAL ERROR: Failed via both endpoints for post %d: %v\n", post.ID, err)
            return 0, err
        }
    }
    
    fmt.Printf("SUCCESS: Post %d added with ID: %d\n", post.ID, postID)
    
    // Добавляем теги в отдельную таблицу
    if len(tags) > 0 {
        go addPostTags(postID, tags)
    }
    
    return postID, nil
}

// Добавление тегов к посту
func addPostTags(postID int, tags []string) {
    for _, tagName := range tags {
        // Создаем или получаем тег
        tagData := map[string]interface{}{
            "name": tagName,
        }
        
        tagID, err := postRequest("/api/tags", tagData, "tag_id")
        if err != nil {
            fmt.Printf("WARNING: Failed to create tag '%s': %v\n", tagName, err)
            continue
        }
        
        // Связываем тег с постом
        linkData := map[string]interface{}{
            "post_id": postID,
            "tag_id":  tagID,
        }
        
        _, err = postRequest("/api/post_tags", linkData, "post_id")
        if err != nil {
            fmt.Printf("WARNING: Failed to link tag '%s' to post %d: %v\n", tagName, postID, err)
        }
    }
}

// Добавление медиа
func AddVKMedia(media vk.VKMedia, postID int) error {
    fmt.Printf("DEBUG: Adding media for postID=%d, type=%s\n", postID, media.Type)
    
    data := map[string]interface{}{
        "post_id":       postID,
        "media_content": media.URL,
        "media_type":    media.Type,
    }

    _, err := postRequest("/vk/media", data, "media_id")
    if err != nil {
        // Попробуем обычный endpoint
        _, err = postRequest("/api/media", data, "media_id")
    }
    
    return err
}

// Добавление комментария VK
func AddVKComment(comment vk.VKComment, postID int, parentID *int) error {
    authorName := comment.AuthorName
    if authorName == "" {
        authorName = fmt.Sprintf("User %d", comment.FromID)
    }

    // Создаем автора комментария
    authorID, err := AddVKAuthor(authorName)
    if err != nil {
        authorID = 1 // Дефолтный автор
    }

    t := time.Now()
    timeStampString := t.Format("2006-01-02 15:04:05")

    data := map[string]interface{}{
        "post_id":           postID,
        "author_id":         authorID,
        "text":              comment.Text,
        "parent_comment_id": parentID,
        "likes_count":       0,
        "created_at":        timeStampString,
    }

    _, err = postRequest("/vk/comments", data, "comment_id")
    if err != nil {
        fmt.Printf("ERROR in AddVKComment: %v\n", err)
    }
    
    return err
}

// ============ ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ============

// POST запрос с ретраями
func postRequest(endpoint string, data map[string]interface{}, fieldName string) (int, error) {
    logger := GetLogger()
    
    startTime := time.Now()
    
    // Маршалинг JSON
    jsonData, err := json.Marshal(data)
    if err != nil {
        if logger != nil {
            logger.LogRequest(endpoint, data, false, nil, fmt.Sprintf("JSON marshal error: %v", err))
        }
        return 0, fmt.Errorf("failed to marshal JSON: %v", err)
    }

    // Пробуем несколько раз при ошибках
    maxRetries := 3
    for retry := 0; retry < maxRetries; retry++ {
        if retry > 0 {
            waitTime := time.Duration(retry) * time.Second * 2
            fmt.Printf("Retry %d/%d for %s after %v\n", retry+1, maxRetries, endpoint, waitTime)
            time.Sleep(waitTime)
        }
        
        // Отправляем запрос
        resp, err := http.Post(baseURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
        if err != nil {
            fmt.Printf("ERROR: HTTP request failed for %s (retry %d): %v\n", endpoint, retry+1, err)
            if retry == maxRetries-1 {
                return 0, fmt.Errorf("HTTP request failed after %d retries: %v", maxRetries, err)
            }
            continue
        }
        
        // Читаем ответ
        bodyBytes, err := io.ReadAll(resp.Body)
        resp.Body.Close()
        
        if err != nil {
            fmt.Printf("ERROR: Failed to read response for %s: %v\n", endpoint, err)
            continue
        }

        // Если сервер вернул ошибку, пробуем снова
        if resp.StatusCode >= 400 && resp.StatusCode < 500 && retry < maxRetries-1 {
            fmt.Printf("Server error %d for %s, retrying...\n", resp.StatusCode, endpoint)
            continue
        }
        
        if resp.StatusCode != http.StatusOK {
            fmt.Printf("ERROR: Server returned %d for %s\n", resp.StatusCode, endpoint)
            return 0, fmt.Errorf("server error %d: %s", resp.StatusCode, string(bodyBytes))
        }

        // Проверяем, если тело ответа пустое
        if len(bodyBytes) == 0 {
            fmt.Printf("ERROR: Empty response from server for %s\n", endpoint)
            return 0, fmt.Errorf("empty response from server")
        }

        // Если не JSON, возвращаем ошибку
        if len(bodyBytes) > 0 && bodyBytes[0] != '{' && bodyBytes[0] != '[' {
            fmt.Printf("ERROR: Non-JSON response for %s: %s\n", endpoint, string(bodyBytes))
            return 0, fmt.Errorf("non-JSON response: %s", string(bodyBytes))
        }

        // Парсим JSON ответ
        var respData map[string]interface{}
        err = json.Unmarshal(bodyBytes, &respData)
        if err != nil {
            fmt.Printf("ERROR: Failed to parse JSON for %s: %v\n", endpoint, err)
            continue
        }

        // Проверяем наличие ошибки в ответе
        if errorMsg, ok := respData["error"].(string); ok && errorMsg != "" {
            fmt.Printf("ERROR: Server error for %s: %s\n", endpoint, errorMsg)
            return 0, fmt.Errorf("server error: %s", errorMsg)
        }

        // Ищем ID в ответе
        idFieldName := ""
        
        switch endpoint {
        case "/vk/sources", "/api/sources":
            idFieldName = "source_id"
        case "/vk/channels", "/api/channels":
            idFieldName = "channel_id"
        case "/vk/posts", "/api/posts":
            idFieldName = "post_id"
        case "/vk/authors", "/api/authors":
            idFieldName = "author_id"
        case "/vk/media", "/api/media":
            idFieldName = "media_id"
        case "/vk/comments", "/api/comments":
            idFieldName = "comment_id"
        default:
            idFieldName = fieldName
        }

        // Ищем ID
        idValue, found := respData[idFieldName]
        if !found {
            idValue, found = respData["id"]
            if !found {
                for key, value := range respData {
                    if strings.HasSuffix(key, "_id") {
                        idValue = value
                        idFieldName = key
                        found = true
                        break
                    }
                }
                if !found {
                    fmt.Printf("WARNING: ID not found in response for %s. Available: %v\n", 
                        endpoint, respData)
                    return 0, fmt.Errorf("ID not found in response")
                }
            }
        }

        // Конвертируем ID
        id, err := convertToInt(idValue, idFieldName)
        if err != nil {
            fmt.Printf("ERROR: Failed to convert ID for %s: %v\n", endpoint, err)
            return 0, err
        }

        duration := time.Since(startTime)
        fmt.Printf("SUCCESS: %s completed in %d ms, ID: %d\n", 
            endpoint, duration.Milliseconds(), id)
        
        return id, nil
    }
    
    return 0, fmt.Errorf("max retries exceeded for %s", endpoint)
}

// Конвертация значения в int
func convertToInt(value interface{}, fieldName string) (int, error) {
    switch v := value.(type) {
    case float64:
        return int(v), nil
    case int:
        return v, nil
    case int32:
        return int(v), nil
    case int64:
        return int(v), nil
    case string:
        if i, err := strconv.Atoi(v); err == nil {
            return i, nil
        }
        return 0, fmt.Errorf("cannot convert string '%s' to int", v)
    default:
        return 0, fmt.Errorf("cannot convert %T to int for field '%s'", value, fieldName)
    }
}

// Извлечение тегов из текста
func extractTags(text string) []string {
    var tags []string
    
    // 1. Хэштеги
    reHashtag := regexp.MustCompile(`#[\p{L}\p{N}_]+`)
    hashtags := reHashtag.FindAllString(text, -1)
    for _, tag := range hashtags {
        cleanTag := strings.ToLower(strings.TrimPrefix(tag, "#"))
        if cleanTag != "" {
            tags = append(tags, cleanTag)
        }
    }
    
    // 2. Ключевые слова
    keywords := []string{"новости", "срочно", "эксклюзив", "факты", "информация", 
                         "важно", "главное", "обновление", "события", "репортаж"}
    textLower := strings.ToLower(text)
    for _, keyword := range keywords {
        if strings.Contains(textLower, keyword) && !contains(tags, keyword) {
            tags = append(tags, keyword)
        }
    }
    
    return tags
}

// Проверка наличия элемента в слайсе
func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}