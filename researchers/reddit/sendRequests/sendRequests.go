package sendRequests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"researcher-reddit/Reddit"
	"strconv"
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

// Функция для добавления источника Reddit (sources)
// Предполагаем, что источник Reddit один, но можно параметризовать
func AddRedditSource() (int, error) {
	data := map[string]interface{}{
		"name":    "Reddit",
		"address": "reddit.com",
		"topic":   "social",
	}
	return postRequest("/sources", data, "source_id")
}

func AddRedditChannel(group Reddit.Subreddit, sourceID int) (int, error) {
	data := map[string]interface{}{
		"name":              group.DisplayName,
		"link":              fmt.Sprintf("https://reddit.com%s", group.URL),
		"subscribers_count": group.Subscribers,
		"source_id":         sourceID,
		"topic":             group.Title,
	}
	return postRequest("/channels", data, "channel_id")
}

func AddRedditPost(post Reddit.Post, channelID int, authorID *int) (int, error) {
	// Добавить автора, если AuthorName != "" и authorID nil
	if post.AuthorName != "" && authorID == nil {
		aid, err := AddRedditAuthor(post.AuthorName)
		if err != nil {
			return 0, fmt.Errorf("failed to add author: %v", err)
		}
		authorID = &aid
	}

	data := map[string]interface{}{
		"title":          post.Title,
		"author_id":      authorID,
		"text_id":        nil, // Заглушка. Сначала добавим text, потом обновим post
		"channel_id":     channelID,
		"comments_count": post.Comments,
		"likes_count":    post.Votes,
		"created_at":     time.Unix(int64(post.Date), 0).UTC().Format("2006-01-02 15:04:05"),
	}

	// Сначала добавить текст новости (news_texts)
	textID, err := AddRedditNewsText(post.Text)
	if err != nil {
		return 0, fmt.Errorf("failed to add text: %v", err)
	}
	data["text_id"] = textID

	// Добавить пост
	postID, err := postRequest("/posts", data, "post_id")
	if err != nil {
		return 0, err
	}

	return postID, nil
}

// Функция для добавления автора (authors)
func AddRedditAuthor(name string) (int, error) {
	data := map[string]interface{}{
		"name": name,
	}
	return postRequest("/authors", data, "author_id")
}

// Функция для добавления текста новости (news_texts)
func AddRedditNewsText(text string) (int, error) {
	data := map[string]interface{}{
		"text": text,
	}
	return postRequest("/news_texts", data, "text_id")
}

func AddRedditComment(comment Reddit.Comment, postID int, parentID *int) error {
	data := map[string]interface{}{
		"post_id":           postID,
		"nickname":          comment.AuthorName,
		"parent_comment_id": parentID,
		"text":              comment.Text,
		"likes_count":       0,
		"created_at":        time.Unix(int64(comment.CreatedUTC), 0).UTC().Format("2006-01-02 15:04:05"),
	}

	commentID, err := postRequest("/comments", data, "comment_id")
	if err != nil {
		return err
	}

	// Рекурсивно добавить вложенные комментарии
	for _, child := range comment.Thread.Items {
		AddRedditComment(child, postID, &commentID)
	}

	return nil
}

func AddVKMedia(media Reddit.Media, postID int) error {
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
