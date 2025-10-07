package vk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Структура для поста (упрощённая, с базовыми полями)
type VKPost struct {
	ID       int    `json:"id"`
	Text     string `json:"text"`
	Date     int    `json:"date"` // Unix timestamp
	Likes    int    `json:"likes_count"`
	Reposts  int    `json:"reposts_count"`
	Comments int    `json:"comments_count"`
}

// Структура ответа от VK API для wall.get
type VKWallResponse struct {
	Response struct {
		Count int      `json:"count"`
		Items []VKPost `json:"items"`
	} `json:"response"`
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
		params.Set("extended", "0")                    // Без расширенной информации (для простоты)
		params.Set("fields", "likes,reposts,comments") // Поля для постов

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

		fmt.Printf("Получено постов в этом запросе: %d\n", len(vkResp.Response.Items))
		for i, p := range vkResp.Response.Items {
			fmt.Printf("  Пост %d: ID=%d, Текст='%s', Дата=%d, Лайки=%d, Репосты=%d, Комменты=%d\n",
				offset+i+1, p.ID, p.Text, p.Date, p.Likes, p.Reposts, p.Comments)
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

	fmt.Printf("Всего получено постов: %d\n", len(allPosts))
	return allPosts, nil
}
