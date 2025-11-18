package vk

import (
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// Расширенная структура для поста
type VKPost struct {
	ID          int                      `json:"id"`
	Text        string                   `json:"text"`
	Date        int64                    `json:"date"` // Unix timestamp
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

func GetRandPosts(count int, id int) ([]VKPost, error) {
	posts := make([]VKPost, count)
	for i := 0; i < count; i++ {
		posts[i] = generateRandomPost(id)
	}

	return posts, nil
}

// Функция генерации одного случайного поста
func generateRandomPost(id int) VKPost {
	return VKPost{
		ID:          rand.Intn(1000000) + 1000,
		Date:        time.Now().Add(-time.Duration(rand.Intn(365)) * 24 * time.Hour).Unix(),
		Text:        generateRandomText(),
		Likes:       rand.Intn(1000),
		Reposts:     rand.Intn(100),
		Comments:    rand.Intn(50),
		Attachments: generateRandomAttachments(rand.Intn(4)),
	}
}

// Генерация случайного текста поста
func generateRandomText() string {
	texts := []string{
		"Отличная новость! Не пропустите!",
		"Сегодня у нас интересное мероприятие",
		"Делимся с вами полезной информацией",
		"Внимание! Важное объявление для всех участников",
		"Спасибо за вашу активность в нашей группе",
		"Новый контент уже на канале!",
		"А вы знали об этом?",
		"Дорогие друзья, приветствуем вас!",
		"Спешим поделиться радостной новостью",
		"Только сегодня специальное предложение",
	}

	// Добавляем немного случайного текста
	if rand.Float32() < 0.7 {
		return texts[rand.Intn(len(texts))] + " " + uuid.New().String()[:8]
	}
	return texts[rand.Intn(len(texts))]
}

// Генерация случайных вложений

func generateRandomAttachments(count int) []map[string]interface{} {
	if count == 0 {
		return []map[string]interface{}{}
	}

	attachments := make([]map[string]interface{}, count)
	attachmentTypes := []string{"photo", "video", "audio", "doc", "poll", "link"}

	for i := 0; i < count; i++ {
		attType := attachmentTypes[rand.Intn(len(attachmentTypes))]

		attachments[i] = map[string]interface{}{
			"type": attType,
			attType: map[string]interface{}{
				"id":       rand.Intn(1000),
				"owner_id": rand.Intn(1000),
				// Добавьте другие поля в зависимости от типа вложения
			},
		}
	}

	return attachments
}
