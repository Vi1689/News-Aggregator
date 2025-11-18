package vk

import (
	"math/rand"

	"github.com/google/uuid"
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
func GetRandComments(count int, offset int) ([]VKComment, error) {
	response := VKCommentsResponse{}
	response.Response.Count = count
	response.Response.CanPost = rand.Float32() < 0.8 // 80% вероятность возможности комментирования

	// Генерация комментариев
	comments := make([]VKComment, count)
	for i := 0; i < count; i++ {
		comments[i] = generateRandomComment(offset+i, count)
	}
	response.Response.Items = comments

	// Генерация пользователей и групп
	response.Response.Profiles = generateRandomUsers(rand.Intn(10) + 5)
	response.Response.Groups = generateRandomGroups(rand.Intn(3) + 1)

	return response.Response.Items, nil
}

// Генерация случайного комментария
func generateRandomComment(seed int, totalComments int) VKComment {
	commentID := rand.Intn(1000000) + 1000
	fromID := rand.Intn(1000000) + 1000

	// Определяем, будет ли это ответ на другой комментарий
	parentID := 0
	if rand.Float32() < 0.3 && totalComments > 1 { // 30% вероятность ответа
		parentID = rand.Intn(totalComments) + 1000
	}

	// Генерация вложенных комментариев (thread)
	threadCount := 0
	if rand.Float32() < 0.4 { // 40% вероятность наличия вложенных комментариев
		threadCount = rand.Intn(5)
	}

	threadItems := make([]VKComment, threadCount)
	for i := 0; i < threadCount; i++ {
		threadItems[i] = generateRandomThreadComment(seed+i+1000, commentID)
	}

	return VKComment{
		ID:       commentID,
		FromID:   fromID,
		Text:     generateRandomCommentText(),
		ParentID: parentID,
		Thread: VKThread{
			Count: threadCount,
			Items: threadItems,
		},
	}
}

// Генерация вложенного комментария
func generateRandomThreadComment(seed int, parentCommentID int) VKComment {
	return VKComment{
		ID:       rand.Intn(1000000) + 2000,
		FromID:   rand.Intn(1000000) + 1000,
		Text:     generateRandomCommentText(),
		ParentID: parentCommentID,
		Thread:   VKThread{Count: 0, Items: []VKComment{}}, // Вложенные не имеют своих вложенных
	}
}

// Генерация случайного текста комментария
func generateRandomCommentText() string {
	texts := []string{
		"Отличный пост! Спасибо за информацию!",
		"Полностью согласен с автором",
		"Интересная точка зрения, но я бы поспорил",
		"Ждем продолжения!",
		"Можно подробнее про этот момент?",
		"Полезная информация, взял на заметку",
		"У меня есть вопрос по этой теме",
		"Спасибо, что поделились!",
		"Интересно, а что думают другие?",
		"Полностью поддерживаю эту идею!",
		"Не совсем понял, можно объяснить?",
		"Отличные новости!",
		"Жаль, что я пропустил это мероприятие",
		"Буду рекомендовать друзьям!",
		"Есть ли аналогичные варианты?",
	}

	// Добавляем уникальности
	return texts[rand.Intn(len(texts))] + " #" + uuid.New().String()[:4]
}

// Генерация случайных пользователей
func generateRandomUsers(count int) []VKUser {
	users := make([]VKUser, count)
	firstNames := []string{"Иван", "Алексей", "Мария", "Екатерина", "Дмитрий", "Ольга", "Сергей", "Анна", "Павел", "Наталья"}
	lastNames := []string{"Иванов", "Петров", "Сидоров", "Смирнов", "Кузнецов", "Васильев", "Попов", "Михайлов", "Новиков", "Федоров"}

	for i := 0; i < count; i++ {
		users[i] = VKUser{
			ID:        rand.Intn(1000000) + 1000,
			FirstName: firstNames[rand.Intn(len(firstNames))],
			LastName:  lastNames[rand.Intn(len(lastNames))],
		}
	}

	return users
}

// Генерация случайных групп
func generateRandomGroups(count int) []VKGroup {
	groups := make([]VKGroup, count)
	groupNames := []string{
		"Технологии будущего",
		"Кулинарные рецепты",
		"Автомобильные новости",
		"Путешествия по миру",
		"Фитнес и здоровье",
		"Книжный клуб",
		"Музыкальные обзоры",
		"Кино и сериалы",
		"Научные открытия",
		"Бизнес идеи",
	}

	for i := 0; i < count; i++ {
		groups[i] = VKGroup{
			ID:   -rand.Intn(1000000) - 1000, // Отрицательный ID для групп
			Name: groupNames[rand.Intn(len(groupNames))],
		}
	}

	return groups
}
