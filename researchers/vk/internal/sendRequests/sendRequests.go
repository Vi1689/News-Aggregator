package sendRequests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"researcher-vk/internal/vk"
	"strconv"
	"strings"
	"time"
)

// Базовый URL сервера (из примеров curl)
const baseURL = "http://localhost:8080/api"

//const baseURL = "http://server:8080/api"

// Структура для ответа сервера (предполагаем, что сервер возвращает JSON с id или успехом)
type APIResponse struct {
	ID      int    `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Функция для добавления источника VK (sources)
// Предполагаем, что источник VK один, но можно параметризовать
func AddVKSource() (int, error) {
	data := map[string]interface{}{
		"name":    "VK",
		"address": "vk.com",
		"topic":   "social",
	}

	return postRequest("/sources", data)
}

// getSourceID отправляет GET-запрос на /api/sources, ищет source по имени и возвращает source_id как int
func GetSourceID() (int, error) {

	type Source struct {
		Address  string `json:"address"`
		Name     string `json:"name"`
		SourceID string `json:"source_id"` // API возвращает как строку
		Topic    string `json:"topic"`
	}

	// Формируем URL
	url := baseURL + "/sources"

	// Отправляем GET-запрос
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to send GET request: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус-код
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Декодируем JSON-ответ в срез структур
	var sources []Source
	if err := json.NewDecoder(resp.Body).Decode(&sources); err != nil {
		return 0, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	// Ищем source по имени
	for _, source := range sources {
		if source.Name == "VK" {
			// Конвертируем source_id из строки в int (поскольку foreign key в БД, вероятно, int)
			sourceID, err := strconv.Atoi(source.SourceID)
			if err != nil {
				return 0, fmt.Errorf("failed to convert source_id to int: %w", err)
			}
			return sourceID, nil
		}
	}

	// Если source не найден
	return 0, fmt.Errorf("source with name VK not found")
}

// Функция для добавления группы VK (channels)
// sourceID - ID источника VK (получить из addVKSource или заранее)
func AddVKChannel(group vk.VKGroup, sourceID int) (int, error) {
	data := map[string]interface{}{
		"name":              group.Name,
		"link":              fmt.Sprintf("https://vk.com/%s", group.ScreenName),
		"subscribers_count": group.MembersCount,
		"source_id":         sourceID,
		"topic":             "general", // Или извлечь из группы, если есть
	}
	return postRequest("/channels", data)
}

// Функция для добавления поста VK (posts)
// channelID - ID канала (группы), authorID - ID автора (если from_id > 0, иначе nil)
// Извлекает теги из текста (#tag) и добавляет их
func AddVKPost(post vk.VKPost, channelID int, authorID *int) (int, error) {
	fmt.Printf("AddVKPost\n")
	// Извлечь теги из текста
	tags := extractTags(post.Text)

	// Добавить автора, если from_id > 0 и authorID nil
	if post.AuthorID > 0 && authorID == nil {
		// Предполагаем, что у тебя есть функция для получения имени автора по ID (или добавить из VKUser)
		// Здесь hardcode, замени на реальный запрос к VK API для имени
		authorName := post.AuthorName // Или запроси через VK API
		if authorName == "" {
			authorName = fmt.Sprintf("User %d", post.AuthorID) // Fallback
		}
		aid, err := AddVKAuthor(authorName)
		if err != nil {
			return 0, fmt.Errorf("failed to add author: %v", err)
		}
		authorID = &aid
	}

	t := time.Unix(post.Date, 0)                       // Создаём time.Time из Unix seconds
	timeStampString := t.Format("2006-01-02 15:04:05") // Формат для PostgreSQL TIMESTAMP (YYYY-MM-DD HH:MM:SS)

	data := map[string]interface{}{
		"title":          fmt.Sprintf("Post %d", post.ID), // Или извлечь заголовок из текста
		"author_id":      authorID,
		"text_id":        nil, // Заглушка. Сначала добавим text, потом обновим post
		"channel_id":     channelID,
		"comments_count": post.Comments,
		"likes_count":    post.Likes,
		"created_at":     timeStampString,
	}

	// Сначала добавить текст новости (news_texts)
	textID, err := AddVKNewsText(post.Text)
	if err != nil {
		return 0, fmt.Errorf("failed to add text: %v", err)
	}
	fmt.Printf("======================== textID = %d\n", textID)
	data["text_id"] = textID

	// Добавить пост
	postID, err := postRequest("/posts", data)
	if err != nil {
		return 0, err
	}

	// Добавить теги
	for _, tag := range tags {
		tagID, err := addVKTag(tag)
		if err != nil {
			continue // Игнорируем ошибки тегов
		}
		addVKPostTag(postID, tagID)
	}

	// Добавить медиа из attachments
	for _, att := range post.Attachments {
		media := parseVKAttachment(att)
		if media != nil {
			AddVKMedia(*media, postID)
		}
	}

	return postID, nil
}

// Функция для добавления автора (authors)
func AddVKAuthor(name string) (int, error) {
	data := map[string]interface{}{
		"name": name,
	}
	return postRequest("/authors", data)
}

// Функция для добавления текста новости (news_texts)
func AddVKNewsText(text string) (int, error) {
	data := map[string]interface{}{
		"text": text,
	}
	_, err := postRequest("/news_texts", data)
	if err != nil {
		return 0, err
	}
	type news_text struct {
		Address string `json:"text"`
		Text_id int    `json:"text_id"`
	}

	url := baseURL + "/sources"
	// Отправляем GET-запрос
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to send GET request: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус-код
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Декодируем JSON-ответ в срез структур
	var sources []Source
	if err := json.NewDecoder(resp.Body).Decode(&sources); err != nil {
		return 0, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	// Ищем source по имени
	for _, source := range sources {
		if source.Name == "VK" {
			// Конвертируем source_id из строки в int (поскольку foreign key в БД, вероятно, int)
			sourceID, err := strconv.Atoi(source.SourceID)
			if err != nil {
				return 0, fmt.Errorf("failed to convert source_id to int: %w", err)
			}
			return sourceID, nil
		}
	}

	return
}

// Функция для добавления тега (tags)
func addVKTag(name string) (int, error) {
	data := map[string]interface{}{
		"name": name,
	}
	return postRequest("/tags", data)
}

// Функция для связи поста и тега (post_tags)
func addVKPostTag(postID, tagID int) error {
	data := map[string]interface{}{
		"post_id": postID,
		"tag_id":  tagID,
	}
	_, err := postRequest("/post_tags", data)
	return err
}

// Функция для добавления комментария VK (comments)
// Рекурсивно обрабатывает thread (вложенные комментарии)
func AddVKComment(comment vk.VKComment, postID int, parentID *int) error {
	// Добавить автора комментария (nickname)
	authorName := comment.AuthorName
	if authorName == "" {
		authorName = fmt.Sprintf("User %d", comment.FromID)
	}
	// Предполагаем, что authors уникальны по name, так что добавим или получим
	_, err := AddVKAuthor(authorName) // Если уже есть, сервер вернёт существующий ID?
	if err != nil {
		return fmt.Errorf("failed to add comment author: %v", err)
	}

	data := map[string]interface{}{
		"post_id":           postID,
		"nickname":          authorName, // Или authorID, но схема использует nickname
		"parent_comment_id": parentID,
		"text":              comment.Text,
		"likes_count":       0,   // VK не даёт likes для комментариев в базовом API?
		"created_at":        nil, // Если есть timestamp, добавь
	}

	commentID, err := postRequest("/comments", data)
	if err != nil {
		return err
	}

	// Рекурсивно добавить вложенные комментарии
	for _, child := range comment.Thread {
		AddVKComment(child, postID, &commentID)
	}

	return nil
}

// Функция для добавления медиа (media)
func AddVKMedia(media vk.VKMedia, postID int) error {
	data := map[string]interface{}{
		"post_id":       postID,
		"media_content": media.URL,
		"media_type":    media.Type,
	}
	_, err := postRequest("/media", data)
	return err
}

// Вспомогательная функция для POST-запроса
func postRequest(endpoint string, data map[string]interface{}) (int, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}

	fmt.Printf("\n\njsonData = %s\n\n", jsonData)

	resp, err := http.Post(baseURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Read the entire response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %v", err)
	}

	bodyStr := string(bodyBytes)

	// Log the response for debugging (remove after fixing)
	fmt.Printf("DEBUG: Response from %s: Status %d, Body: %s\n", endpoint, resp.StatusCode, bodyStr)

	// Try to decode as JSON
	var apiResp APIResponse
	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&apiResp); err == nil {
		// Successfully decoded JSON
		if apiResp.Error != "" {
			return 0, fmt.Errorf("API error: %s", apiResp.Error)
		}
		return apiResp.ID, nil
	}

	// If decoding failed, check if it's a plain text success message
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		if strings.Contains(strings.ToLower(bodyStr), "added") || strings.Contains(strings.ToLower(bodyStr), "success") {
			// Assume success, return ID=0 (since no ID provided in plain text)
			fmt.Printf("INFO: Plain text success response, assuming ID=0 for %s\n", endpoint)
			return 0, nil
		}
	}

	// Otherwise, it's an error
	return 0, fmt.Errorf("unexpected response: Status %d, Body: %s", resp.StatusCode, bodyStr)
}

// Вспомогательная функция для извлечения тегов из текста (#tag)
func extractTags(text string) []string {
	words := strings.Fields(text)
	var tags []string
	for _, word := range words {
		if strings.HasPrefix(word, "#") {
			tag := strings.Trim(word, "#.,!?")
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// Вспомогательная функция для парсинга attachment в VKMedia
func parseVKAttachment(att map[string]interface{}) *vk.VKMedia {
	typ, ok := att["type"].(string)
	if !ok {
		return nil
	}

	var url string
	switch typ {
	case "photo":
		if photo, ok := att["photo"].(map[string]interface{}); ok {
			if sizes, ok := photo["sizes"].([]interface{}); ok && len(sizes) > 0 {
				if size, ok := sizes[len(sizes)-1].(map[string]interface{}); ok { // Больший размер
					if u, ok := size["url"].(string); ok {
						url = u
					}
				}
			}
		}
	case "video":
		if video, ok := att["video"].(map[string]interface{}); ok {
			if player, ok := video["player"].(string); ok {
				url = player
			}
		}
	case "audio":
		if audio, ok := att["audio"].(map[string]interface{}); ok {
			if u, ok := audio["url"].(string); ok {
				url = u
			}
		}
	// Добавь другие типы, если нужно
	default:
		return nil
	}

	return &vk.VKMedia{Type: typ, URL: url}
}
