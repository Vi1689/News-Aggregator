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

// Расширенная структура для поста
type VKPost struct {
	ID          int                      `json:"id"`
	Text        string                   `json:"text"`
	Date        time.Time                `json:"date"` // Unix timestamp
	Likes       int                      `json:"likes_count"`
	Reposts     int                      `json:"reposts_count"`
	Comments    int                      `json:"comments_count"`
	AuthorID    int                      `json:"from_id"` // ID автора (пользователь или группа)
	AuthorName  string                   // Имя автора (заполнится автоматически)
	Attachments []map[string]interface{} `json:"attachments,omitempty"` // Добавлено: массив вложений (фото, видео и т.д.)
	Tags        []string                 // Добавлено: массив хэштегов (извлекается из текста)
}

// Структура ответа от VK API для wall.get с extended=1
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

// Функция для извлечения хэштегов из текста
func extractTags(text string) []string {
	re := regexp.MustCompile(`#\w+`)
	matches := re.FindAllString(text, -1)
	var tags []string
	for _, match := range matches {
		// Убираем # и приводим к нижнему регистру для нормализации
		tag := strings.TrimPrefix(match, "#")
		tags = append(tags, strings.ToLower(tag))
	}
	return tags
}

// Функция для получения заданного количества постов группы
func GetGroupPosts(accessToken string, groupID int, count int) ([]VKPost, error) {
	if count <= 0 {
		return []VKPost{}, nil // Пустой слайс, если count <= 0
	}

	// Максимум 100 постов на запрос
	const maxPerRequest = 100
	var allPosts []VKPost
	offset := 0

	// Цикл для пагинации
	for offset < count {
		// Сколько брать в этом запросе
		currentCount := maxPerRequest
		if remaining := count - offset; remaining < maxPerRequest {
			currentCount = remaining
		}

		// Базовый URL VK API
		baseURL := "https://api.vk.com/method/wall.get"

		// Параметры запроса
		params := url.Values{}
		params.Set("access_token", accessToken)
		params.Set("v", "5.131")                          // Версия API
		params.Set("owner_id", "-"+strconv.Itoa(groupID)) // ID группы с минусом
		params.Set("count", strconv.Itoa(currentCount))
		params.Set("offset", strconv.Itoa(offset))
		params.Set("extended", "1")                       // Расширенная информация
		params.Set("fields", "first_name,last_name,name") // Поля для профилей и групп

		// Формируем полный URL
		fullURL := baseURL + "?" + params.Encode()
		fmt.Printf("Запрос: %s (offset: %d, count: %d)\n", fullURL, offset, currentCount) // Отладка

		// Задаём таймаут
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		// Делаем HTTP GET запрос
		resp, err := client.Get(fullURL)
		if err != nil {
			return nil, fmt.Errorf("ошибка запроса к VK API: %w", err)
		}
		defer resp.Body.Close()

		// Проверяем статус ответа
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("VK API вернул ошибку: %s, тело: %s", resp.Status, string(body))
		}

		// Читаем и парсим JSON
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
		}

		var vkResp VKWallResponse
		if err := json.Unmarshal(body, &vkResp); err != nil {
			return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
		}

		// Создаём мапы для имён: профили (положительные ID) и группы (отрицательные ID)
		profilesMap := make(map[int]string)
		for _, p := range vkResp.Response.Profiles {
			profilesMap[p.ID] = p.FirstName + " " + p.LastName
		}
		groupsMap := make(map[int]string)
		for _, g := range vkResp.Response.Groups {
			groupsMap[-g.ID] = g.Name // Группы имеют отрицательные ID в from_id
		}

		// Заполняем AuthorName и Tags для каждого поста
		for i := range vkResp.Response.Items {
			post := &vkResp.Response.Items[i]
			if name, ok := profilesMap[post.AuthorID]; ok {
				post.AuthorName = name
			} else if name, ok := groupsMap[post.AuthorID]; ok {
				post.AuthorName = name
			} else {
				post.AuthorName = "Неизвестный автор" // Если не найден
			}
			// Извлекаем теги из текста
			post.Tags = extractTags(post.Text)
		}

		// Добавляем посты в общий слайс
		allPosts = append(allPosts, vkResp.Response.Items...)

		// Если постов меньше, чем запрошено, выходим
		if len(vkResp.Response.Items) < currentCount {
			break
		}

		// Увеличиваем offset
		offset += currentCount

		// Небольшая пауза между запросами (чтобы не превысить лимиты VK)
		time.Sleep(100 * time.Millisecond)
	}

	return allPosts, nil
}
