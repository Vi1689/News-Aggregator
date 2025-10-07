package vk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// Структура для медиа (упрощённая)
type VKMedia struct {
	Type string `json:"type"` // "photo", "video", "audio" и т.д.
	URL  string `json:"url"`  // URL медиа (если доступен)
}

// Структура для фото (упрощённая)
type VKPhoto struct {
	Sizes []VKPhotoSize `json:"sizes"`
}
type VKPhotoSize struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Структура для видео (упрощённая)
type VKVideo struct {
	Player string `json:"player"` // URL плеера
}

// Структура для аудио (упрощённая)
type VKAudio struct {
	URL string `json:"url"`
}

// Функция для получения медиа из заданного количества постов
func GetMediaFromPosts(accessToken string, ownerID int, count int) ([]VKMedia, error) {
	// Базовый URL VK API для получения постов
	baseURL := "https://api.vk.com/method/wall.get"

	// Параметры запроса
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("v", "5.131")                      // Версия API
	params.Set("owner_id", strconv.Itoa(ownerID)) // ID владельца стены (группа с минусом)
	params.Set("count", strconv.Itoa(count))      // Количество постов (макс 100)
	params.Set("extended", "1")                   // Расширенная информация (опционально)

	// Формируем полный URL
	fullURL := baseURL + "?" + params.Encode()
	fmt.Printf("Запрос: %s\n", fullURL)

	// Делаем HTTP GET запрос
	resp, err := http.Get(fullURL)
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

	// Извлекаем медиа из attachments всех постов
	var mediaList []VKMedia
	for _, post := range vkResp.Response.Items {
		for _, att := range post.Attachments {
			mediaType, ok := att["type"].(string)
			if !ok {
				continue
			}
			var url string
			switch mediaType {
			case "photo":
				if photoData, ok := att["photo"].(map[string]interface{}); ok {
					photoBytes, _ := json.Marshal(photoData)
					var photo VKPhoto
					json.Unmarshal(photoBytes, &photo)
					if len(photo.Sizes) > 0 {
						url = photo.Sizes[len(photo.Sizes)-1].URL // Самый большой размер
					}
				}
			case "video":
				if videoData, ok := att["video"].(map[string]interface{}); ok {
					videoBytes, _ := json.Marshal(videoData)
					var video VKVideo
					json.Unmarshal(videoBytes, &video)
					url = video.Player // URL плеера
				}
			case "audio":
				if audioData, ok := att["audio"].(map[string]interface{}); ok {
					audioBytes, _ := json.Marshal(audioData)
					var audio VKAudio
					json.Unmarshal(audioBytes, &audio)
					url = audio.URL
				}
			case "doc":
				// Для документов можно взять URL, если есть
				if docData, ok := att["doc"].(map[string]interface{}); ok {
					if docURL, ok := docData["url"].(string); ok {
						url = docURL
					}
				}
			// Можно добавить другие типы: link, poll и т.д.
			default:
				continue
			}
			if url != "" {
				mediaList = append(mediaList, VKMedia{Type: mediaType, URL: url})
			}
		}
	}

	return mediaList, nil
}
