package vk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Структура поста
type VKPost struct {
	ID          int                      `json:"id"`
	Text        string                   `json:"text"`
	Date        int64                    `json:"date"`
	Likes       int                      `json:"likes_count"`
	Reposts     int                      `json:"reposts_count"`
	Comments    int                      `json:"comments_count"`
	AuthorID    int                      `json:"from_id"`
	AuthorName  string                   
	Attachments []map[string]interface{} `json:"attachments,omitempty"`
	Tags        []string                 
}

// Структура ответа
type VKWallResponse struct {
	Response struct {
		Count    int      `json:"count"`
		Items    []VKPost `json:"items"`
		Profiles []struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"profiles"`
		Groups []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"groups"`
	} `json:"response"`
}

// Извлечение тегов
func extractTags(text string) []string {
	re := regexp.MustCompile(`#[\p{L}\p{N}_]+`)
	matches := re.FindAllString(text, -1)
	var tags []string
	for _, match := range matches {
		tag := strings.ToLower(strings.TrimPrefix(match, "#"))
		if tag != "" && !contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	return tags
}

// Проверка наличия в слайсе
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Получение постов с ретраями
func GetGroupPostsWithRetry(accessToken string, groupID int, count int) ([]VKPost, error) {
    fmt.Printf("DEBUG GetGroupPostsWithRetry: groupID=%d, count=%d\n", groupID, count)
    
    if count <= 0 {
        return []VKPost{}, nil
    }

    // Ограничиваем count для избежания Flood Control
    if count > 50 {
        count = 50
        fmt.Printf("Reducing post count to %d to avoid flood control\n", count)
    }

    const maxPerRequest = 20 // Меньше запросов за раз
    var allPosts []VKPost
    offset := 0

    // Цикл для пагинации
    for offset < count {
        currentCount := maxPerRequest
        if remaining := count - offset; remaining < maxPerRequest {
            currentCount = remaining
        }

        // Пробуем несколько раз при ошибках
        var posts []VKPost
        var err error
        
for retry := 0; retry < 3; retry++ {
    if retry > 0 {
        waitTime := time.Duration(retry) * time.Second * 10 // Увеличить с 5 до 10 секунд
        fmt.Printf("Retry %d for group %d, waiting %v\n", retry+1, groupID, waitTime)
        time.Sleep(waitTime)
    }
    
    posts, err = getPostsPage(accessToken, groupID, currentCount, offset)
    if err != nil {
        if strings.Contains(err.Error(), "Flood control") {
            fmt.Printf("Flood control detected, waiting 30 seconds...\n")
            time.Sleep(30 * time.Second) // Добавить большую паузу при flood control
            if retry < 2 {
                continue
            }
            return allPosts, fmt.Errorf("flood control")
        }
        return allPosts, err
    }
    break
}
        
        if err != nil {
            return allPosts, err
        }

        // Заполняем имена авторов (переименовали функцию)
        fillPostAuthorNames(posts, groupID)

        // Добавляем посты в общий слайс
        allPosts = append(allPosts, posts...)

        // Если постов меньше, чем запрошено, выходим
        if len(posts) < currentCount {
            break
        }

        // Увеличиваем offset
        offset += currentCount

        // Пауза между запросами
        time.Sleep(1 * time.Second)
    }

    fmt.Printf("DEBUG: Total posts retrieved: %d\n", len(allPosts))
    return allPosts, nil
}

// Получение одной страницы постов
func getPostsPage(accessToken string, groupID int, count, offset int) ([]VKPost, error) {
    baseURL := "https://api.vk.com/method/wall.get"

    // ВАЖНО: Для групп owner_id должен быть отрицательным!
    ownerID := "-" + strconv.Itoa(groupID)
    
    params := url.Values{}
    params.Set("access_token", accessToken)
    params.Set("v", "5.199") // Обновленная версия API
    params.Set("owner_id", ownerID)
    params.Set("count", strconv.Itoa(count))
    params.Set("offset", strconv.Itoa(offset))
    params.Set("extended", "1")
    params.Set("fields", "first_name,last_name,name")

    fullURL := baseURL + "?" + params.Encode()
    
    // ДОБАВИМ ПОДРОБНУЮ ОТЛАДКУ
    fmt.Printf("DEBUG getPostsPage: owner_id=%s, groupID=%d\n", ownerID, groupID)
    
    client := &http.Client{
        Timeout: 30 * time.Second,
    }

    resp, err := client.Get(fullURL)
    if err != nil {
        fmt.Printf("ERROR getPostsPage HTTP: %v\n", err)
        return nil, fmt.Errorf("HTTP request error: %w", err)
    }
    defer resp.Body.Close()

    // Читаем ответ
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        fmt.Printf("ERROR getPostsPage read body: %v\n", err)
        return nil, fmt.Errorf("read response error: %w", err)
    }

    // Выводим ответ для отладки (без токена)
    fmt.Printf("DEBUG getPostsPage response (%d bytes): %s\n", 
        len(body), 
        string(body[:minInt(500, len(body))]) + "...")

    if resp.StatusCode != http.StatusOK {
        fmt.Printf("ERROR getPostsPage status %d: %s\n", resp.StatusCode, string(body))
        return nil, fmt.Errorf("VK API error %d: %s", resp.StatusCode, string(body))
    }

    // Парсим JSON
    var vkResp VKWallResponse
    if err := json.Unmarshal(body, &vkResp); err != nil {
        // Проверяем на специфические ошибки VK
        responseStr := string(body)
        if strings.Contains(responseStr, "Flood control") {
            fmt.Printf("ERROR getPostsPage: Flood control detected\n")
            return nil, fmt.Errorf("flood control")
        }
        if strings.Contains(responseStr, "error") {
            // Извлекаем ошибку из JSON
            var errorResp struct {
                Error struct {
                    ErrorCode int    `json:"error_code"`
                    ErrorMsg  string `json:"error_msg"`
                } `json:"error"`
            }
            if json.Unmarshal(body, &errorResp) == nil {
                fmt.Printf("ERROR getPostsPage VK API: Code=%d, Message=%s\n", 
                    errorResp.Error.ErrorCode, errorResp.Error.ErrorMsg)
                return nil, fmt.Errorf("VK API error %d: %s", 
                    errorResp.Error.ErrorCode, errorResp.Error.ErrorMsg)
            }
        }
        fmt.Printf("ERROR getPostsPage JSON parse: %v\n", err)
        return nil, fmt.Errorf("JSON parse error: %w", err)
    }

    // Проверяем ответ
    fmt.Printf("DEBUG getPostsPage: response count=%d, items=%d\n", 
        vkResp.Response.Count, len(vkResp.Response.Items))
    
    // Если count = 0, это может означать что стена закрыта или нет постов
    if vkResp.Response.Count == 0 {
        fmt.Printf("WARNING getPostsPage: Wall is empty or closed for group %d\n", groupID)
    }

    return vkResp.Response.Items, nil
}

func minInt(a, b int) int {
    if a < b {
        return a
    }
    return b
}

// Заполнение имен авторов для постов (переименованная функция)
func fillPostAuthorNames(posts []VKPost, groupID int) {
    for i := range posts {
        // Для групп автором является сама группа
        posts[i].AuthorID = -groupID
        posts[i].AuthorName = fmt.Sprintf("VK Group %d", groupID)
        
        // Извлекаем теги
        posts[i].Tags = extractTags(posts[i].Text)
        
        // Если текст пустой, пропускаем
        if posts[i].Text == "" {
            posts[i].Text = fmt.Sprintf("Post %d from group %d", posts[i].ID, groupID)
        }
    }
}