package vk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// Структура для пользователя
type VKUser struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// Структура для вложенных комментариев (thread) — НОВАЯ
type VKThread struct {
	Count int         `json:"count"` // Общее количество вложенных комментариев
	Items []VKComment `json:"items"` // Массив вложенных комментариев
}

// Структура для комментария (с ветвлениями, ID родительского и автором)
type VKComment struct {
	ID         int      `json:"id"`
	FromID     int      `json:"from_id"`
	Text       string   `json:"text"`
	ParentID   int      `json:"reply_to_comment"` // ID родительского комментария
	AuthorName string   `json:"-"`                // Имя автора (заполняется вручную, не из JSON)
	Thread     VKThread `json:"thread"`           // Вложенные комментарии как объект (изменено с []VKComment)
}

// Структура ответа от VK API для wall.getComments
type VKCommentsResponse struct {
	Response struct {
		Count    int         `json:"count"`
		Items    []VKComment `json:"items"`
		CanPost  bool        `json:"can_post"`
		Groups   []VKGroup   `json:"groups"`
		Profiles []VKUser    `json:"profiles"`
	} `json:"response"`
}

// Вспомогательная функция для получения имени автора по ID
func getAuthorName(fromID int, users map[int]VKUser, groups map[int]VKGroup) string {
	if fromID > 0 {
		// Пользователь
		if user, ok := users[fromID]; ok {
			return user.FirstName + " " + user.LastName
		}
	} else if fromID < 0 {
		// Группа (ID отрицательный)
		groupID := -fromID
		if group, ok := groups[groupID]; ok {
			return group.Name
		}
	}
	return "Неизвестный"
}

// Рекурсивная функция для заполнения AuthorName в комментариях и их ветвлениях
func fillAuthorNames(comments []VKComment, users map[int]VKUser, groups map[int]VKGroup) {
	for i := range comments {
		comments[i].AuthorName = getAuthorName(comments[i].FromID, users, groups)
		// Теперь проверяем Thread.Items (массив вложенных комментариев)
		if len(comments[i].Thread.Items) > 0 {
			fillAuthorNames(comments[i].Thread.Items, users, groups)
		}
	}
}

// Функция для получения заданного количества комментариев с ветвлениями
func GetCommentsWithThreads(accessToken string, ownerID int, postID int, count int) ([]VKComment, error) {
	// Базовый URL VK API для получения комментариев
	baseURL := "https://api.vk.com/method/wall.getComments"

	// Параметры запроса
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("v", "5.131")                      // Версия API
	params.Set("owner_id", strconv.Itoa(ownerID)) // ID владельца поста
	params.Set("post_id", strconv.Itoa(postID))   // ID поста
	params.Set("count", strconv.Itoa(count))      // Количество корневых комментариев (макс 100)
	params.Set("thread_items_count", "10")        // Количество вложенных комментариев на уровень (макс 10)
	params.Set("extended", "1")                   // Получить расширенную информацию (профили, группы)

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
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var vkResp VKCommentsResponse
	if err := json.Unmarshal(bodyBytes, &vkResp); err != nil {
		return nil, err
	}

	// Создаём мапы для быстрого поиска профилей и групп
	users := make(map[int]VKUser)
	for _, user := range vkResp.Response.Profiles {
		users[user.ID] = user
	}
	groups := make(map[int]VKGroup)
	for _, group := range vkResp.Response.Groups {
		groups[group.ID] = group
	}

	// Заполняем AuthorName в комментариях и их ветвлениях
	fillAuthorNames(vkResp.Response.Items, users, groups)

	// Возвращаем комментарии
	return vkResp.Response.Items, nil
}
