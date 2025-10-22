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
//const baseURL = "http://localhost:8080/api"

const baseURL = "http://server:8080/api"

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
	return postRequest("/sources", data, "source_id")
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
	return postRequest("/channels", data, "channel_id")
}

// Функция для добавления поста VK (posts)
// channelID - ID канала (группы), authorID - ID автора (если from_id > 0, иначе nil)
// Извлекает теги из текста (#tag) и добавляет их
func AddVKPost(post vk.VKPost, channelID int, authorID *int) (int, error) {
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
	data["text_id"] = textID

	// Добавить пост
	postID, err := postRequest("/posts", data, "post_id")
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

	return postID, nil
}

// Функция для добавления автора (authors)
func AddVKAuthor(name string) (int, error) {
	data := map[string]interface{}{
		"name": name,
	}
	return postRequest("/authors", data, "author_id")
}

// Функция для добавления текста новости (news_texts)
func AddVKNewsText(text string) (int, error) {
	data := map[string]interface{}{
		"text": text,
	}
	return postRequest("/news_texts", data, "text_id")
}

// Функция для добавления тега (tags)
func addVKTag(name string) (int, error) {
	data := map[string]interface{}{
		"name": name,
	}
	return postRequest("/tags", data, "tag_id")
}

// Функция для связи поста и тега (post_tags)
func addVKPostTag(postID, tagID int) error {
	data := map[string]interface{}{
		"post_id": postID,
		"tag_id":  tagID,
	}
	_, err := postRequest("/post_tags", data, "tag_id")
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

	commentID, err := postRequest("/comments", data, "comment_id")
	if err != nil {
		return err
	}

	// Рекурсивно добавить вложенные комментарии
	for _, child := range comment.Thread.Items {
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

	_, err := postRequest("/media", data, "media_id")
	return err
}

// Вспомогательная функция для POST-запроса
// возвращает ошибку и id сущности fieldName - название поля id сущности
func postRequest(endpoint string, data map[string]interface{}, fieldName string) (int, error) {
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

	if bodyBytes[0] != '{' {
		return 0, fmt.Errorf("error in response: %s", bodyBytes)
	}
	// Otherwise, it's an error

	var resp_unmarshal map[string]interface{}
	err = json.Unmarshal(bodyBytes, &resp_unmarshal)
	if err != nil {
		return 0, err
	}

	fieldValue, exists := resp_unmarshal[fieldName]
	if !exists {
		return 0, fmt.Errorf("there is no field %s in response", fieldName)
	}

	strValue, ok := fieldValue.(string)
	if !ok {
		return 0, fmt.Errorf("field '%s' is not a string (got %T)", fieldName, fieldValue)
	}

	sourceId, err := strconv.Atoi(strValue)
	if err != nil {
		return 0, err
	}

	return sourceId, nil
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
