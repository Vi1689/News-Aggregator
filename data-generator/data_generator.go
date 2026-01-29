package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"io" 
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

// Конфигурация генератора
type GeneratorConfig struct {
	APIURL           string `json:"api_url"`
	BatchSize        int    `json:"batch_size"`
	DelayBetweenRuns int    `json:"delay_between_runs"` // секунд
	MaxCycles        int    `json:"max_cycles"`         // 0 = бесконечно
	LogLevel         string `json:"log_level"`
}

// Статистика генерации
type GenerationStats struct {
	sync.RWMutex
	SourcesCreated   int
	AuthorsCreated   int
	ChannelsCreated  int
	PostsCreated     int
	TagsCreated      int
	CommentsCreated  int
	MediaCreated     int
	Errors           int
	LastRun          time.Time
	StartTime        time.Time
}

var (
	config    GeneratorConfig
	stats     GenerationStats
	topics    = []string{"новости", "технологии", "спорт", "политика", "экономика", "культура", "наука", "здоровье", "образование", "развлечения"}
	tagPool   = []string{"важно", "срочно", "эксклюзив", "аналитика", "мнение", "факты", "интервью", "репортаж", "новости", "события", "тренд", "исследование", "открытие", "достижение"}
	logger    *log.Logger
	authorIDs []int
	channelIDs []int
	sourceIDs []int
	tagIDs    []int
)

func init() {
	// Настройка логгера
	logger = log.New(os.Stdout, "[DATA_GEN] ", log.LstdFlags|log.Lshortfile)
	
	// Инициализация случайных данных
	gofakeit.Seed(time.Now().UnixNano())
	
	// Конфигурация по умолчанию
	config = GeneratorConfig{
		APIURL:           getEnv("API_URL", "http://localhost:8080"),
		BatchSize:        getEnvAsInt("BATCH_SIZE", 5),
		DelayBetweenRuns: getEnvAsInt("DELAY_BETWEEN_RUNS", 30),
		MaxCycles:        getEnvAsInt("MAX_CYCLES", 0),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
	}
	
	stats.StartTime = time.Now()
}

func main() {
	logger.Printf("🚀 Запуск генератора данных")
	logger.Printf("Конфигурация: %+v", config)
	logger.Printf("API: %s", config.APIURL)
	
	// Проверяем доступность сервера
	if !checkServerHealth() {
		logger.Fatal("Сервер недоступен. Проверьте подключение.")
	}
	
	// Получаем существующие данные
	loadExistingData()
	
	cycle := 0
	for {
		if config.MaxCycles > 0 && cycle >= config.MaxCycles {
			logger.Printf("Достигнуто максимальное количество циклов: %d", config.MaxCycles)
			break
		}
		
		cycle++
		logger.Printf("\n=== ЦИКЛ %d ===", cycle)
		
		// Генерация данных
		generateBatch()
		
		// Показываем статистику
		showStats()
		
		// Пауза между циклами
		if cycle < config.MaxCycles || config.MaxCycles == 0 {
			logger.Printf("Ожидание %d секунд до следующего цикла...", config.DelayBetweenRuns)
			time.Sleep(time.Duration(config.DelayBetweenRuns) * time.Second)
		}
	}
	
	logger.Printf("\n✅ Генерация данных завершена")
	showFinalStats()
}

// ============ ФУНКЦИИ ГЕНЕРАЦИИ ДАННЫХ ============

// Добавим глобальные счетчики для всех объектов
var postCounter int
var tagCounter int
var channelCounter int
var authorCounter int
var sourceCounter int
var commentCounter int
var mediaCounter int

func generateBatch() {
	// 1. Источники (создаем один раз)
	if len(sourceIDs) == 0 {
		createSources(5) // Генерируем 5 уникальных источников
	}
	
	// 2. Авторы (создаем 3 новых автора, если их меньше 10)
	if len(authorIDs) < 10 {
		createAuthors(3)
	}
	
	// 3. Каналы (создаем 2 новых канала, если их меньше 5)
	if len(channelIDs) < 5 {
		createChannels(2)
	}
	
	// 4. Посты (основной контент, количество определяется BatchSize)
	createPosts(config.BatchSize)
	
	// 5. Теги (создаем 2 новых тега, если их меньше 10)
	if len(tagIDs) < 10 {
		createTags(2)
	}
	
	// 6. Комментарии (к некоторым постам, создаем от 1 до 3 комментариев)
	createComments(rand.Intn(3) + 1)
	
	// 7. Медиа (к некоторым постам, создаем от 1 до 2 медиа)
	createMedia(rand.Intn(2) + 1)
}

// Генерация уникальных источников с использованием счетчика
func createSources(count int) {
	for i := 0; i < count; i++ {
		// Генерация уникального имени для источника
		sourceCounter++
		sourceName := fmt.Sprintf("source_%d", sourceCounter)
		
		data := map[string]interface{}{
			"name":    sourceName,
			"address": fmt.Sprintf("https://source%d.example.com", sourceCounter),
			"topic":   topics[rand.Intn(len(topics))],
		}
		
		id, err := sendRequest("/api/sources", data, "source_id")
		if err != nil {
			logger.Printf("Ошибка создания источника: %v", err)
			stats.Errors++
		} else if id > 0 {
			sourceIDs = append(sourceIDs, id)
			stats.SourcesCreated++
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Генерация уникальных авторов с использованием счетчика
func createAuthors(count int) {
	for i := 0; i < count; i++ {
		// Генерация уникального имени для автора
		authorCounter++
		authorName := fmt.Sprintf("author_%d", authorCounter)
		
		data := map[string]interface{}{
			"name": authorName,
		}
		
		id, err := sendRequest("/api/authors", data, "author_id")
		if err != nil {
			logger.Printf("Ошибка создания автора: %v", err)
			stats.Errors++
		} else if id > 0 {
			authorIDs = append(authorIDs, id)
			stats.AuthorsCreated++
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Генерация уникальных каналов с использованием счетчика
func createChannels(count int) {
	if len(sourceIDs) == 0 {
		return
	}
	
	for i := 0; i < count; i++ {
		// Генерация уникального имени для канала
		channelCounter++
		channelName := fmt.Sprintf("channel_%d", channelCounter)
		
		subscribers := rand.Intn(100000) + 1000
		topic := topics[rand.Intn(len(topics))]
		
		data := map[string]interface{}{
			"name":               channelName,
			"link":               fmt.Sprintf("https://channel-%d.example.com", i+1),
			"subscribers_count":  subscribers,
			"source_id":          sourceIDs[rand.Intn(len(sourceIDs))],
			"topic":              topic,
		}
		
		id, err := sendRequest("/api/channels", data, "channel_id")
		if err != nil {
			logger.Printf("Ошибка создания канала: %v", err)
			stats.Errors++
		} else if id > 0 {
			channelIDs = append(channelIDs, id)
			stats.ChannelsCreated++
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Генерация уникальных тегов с использованием счетчика
func createTags(count int) {
	for i := 0; i < count; i++ {
		// Генерация уникального имени для тега
		tagCounter++
		tagName := fmt.Sprintf("tag_%d", tagCounter)
		
		data := map[string]interface{}{
			"name": tagName,
		}
		
		id, err := sendRequest("/api/tags", data, "tag_id")
		if err != nil {
			// Тег может уже существовать, это нормально
			if !strings.Contains(err.Error(), "duplicate") {
				logger.Printf("Ошибка создания тега: %v", err)
				stats.Errors++
			}
		} else if id > 0 {
			tagIDs = append(tagIDs, id)
			stats.TagsCreated++
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Генерация уникальных комментариев с использованием счетчика
func createComments(count int) {
	// Получаем последние посты
	posts := getRecentPosts(count * 2)
	if len(posts) == 0 {
		return
	}
	
	for i := 0; i < count && i < len(posts); i++ {
		post := posts[i].(map[string]interface{})
		postID := int(post["post_id"].(float64))
		
		// Генерация уникального комментария
		commentCounter++
		commentText := fmt.Sprintf("Комментарий %d для поста %d", commentCounter, postID)
		
		createCommentsForPost(postID, rand.Intn(2)+1, commentText)
	}
}

// Генерация комментариев для конкретного поста
func createCommentsForPost(postID, count int, commentText string) {
	if len(authorIDs) == 0 {
		return
	}
	
	for i := 0; i < count; i++ {
		data := map[string]interface{}{
			"post_id":   postID,
			"nickname":  gofakeit.Username(),
			"text":      commentText,
			"likes_count": rand.Intn(50),
			"created_at": time.Now().Add(-time.Duration(rand.Intn(86400)) * time.Second).Format("2006-01-02 15:04:05"),
		}
		
		_, err := sendRequest("/api/comments", data, "comment_id")
		if err != nil {
			logger.Printf("Ошибка создания комментария: %v", err)
			stats.Errors++
		} else {
			stats.CommentsCreated++
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Генерация уникальных медиа с использованием счетчика
func createMedia(count int) {
	// Получаем посты для добавления медиа
	posts := getRecentPosts(count)
	if len(posts) == 0 {
		return
	}
	
	mediaTypes := []string{"image", "video", "audio"}
	
	for i := 0; i < count && i < len(posts); i++ {
		post := posts[i].(map[string]interface{})
		postID := int(post["post_id"].(float64))
		mediaType := mediaTypes[rand.Intn(len(mediaTypes))]
		
		// Генерация уникального медиа
		mediaCounter++
		mediaContent := fmt.Sprintf("media_content_%d", mediaCounter)
		
		data := map[string]interface{}{
			"post_id":       postID,
			"media_content": generateMediaURL(mediaType),
			"media_type":    mediaType,
		}
		
		_, err := sendRequest("/api/media", data, "media_id")
		if err != nil {
			logger.Printf("Ошибка создания медиа: %v", err)
			stats.Errors++
		} else {
			stats.MediaCreated++
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ============ ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ============

func sendRequest(endpoint string, data map[string]interface{}, idField string) (int, error) {
	url := config.APIURL + endpoint
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("ошибка маршалинга JSON: %v", err)
	}
	
	// Отправляем запрос с ретраями
	for retry := 0; retry < 3; retry++ {
		if retry > 0 {
			time.Sleep(time.Duration(retry) * time.Second)
		}
		
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			if retry == 2 {
				return 0, fmt.Errorf("HTTP ошибка: %v", err)
			}
			continue
		}
		defer resp.Body.Close()
		
		body, err := decodeResponse(resp)
		if err != nil {
			if retry == 2 {
				return 0, err
			}
			continue
		}
		
		// Извлекаем ID
		if idValue, ok := body[idField]; ok {
			switch v := idValue.(type) {
			case float64:
				return int(v), nil
			case int:
				return v, nil
			case string:
				if id, err := strconv.Atoi(v); err == nil {
					return id, nil
				}
			}
		}
		
		return 0, nil // Успешно, но без ID
	}
	
	return 0, fmt.Errorf("максимальное количество попыток")
}

func decodeResponse(resp *http.Response) (map[string]interface{}, error) {
	// Читаем тело ответа
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул %d: %s", resp.StatusCode, string(bodyBytes))
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}
	
	return result, nil
}
func getRecentPosts(limit int) []interface{} {
	url := fmt.Sprintf("%s/api/posts", config.APIURL)
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	
	var posts []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil
	}
	
	if len(posts) > limit {
		return posts[:limit]
	}
	return posts
}

func generatePostTitle() string {
	templates := []string{
		"Важные новости о %s",
		"Эксклюзив: %s",
		"Что происходит с %s?",
		"Новое исследование о %s",
		"Сенсационные данные по %s",
		"Анализ ситуации с %s",
		"Прогноз развития %s",
		"Интервью с экспертом по %s",
		"Революция в области %s",
		"Главные события недели: %s",
	}
	
	topic := topics[rand.Intn(len(topics))]
	return fmt.Sprintf(templates[rand.Intn(len(templates))], topic)
}

func generatePostContent() string {
	paragraphs := rand.Intn(3) + 1
	content := ""
	
	for i := 0; i < paragraphs; i++ {
		content += gofakeit.Paragraph(rand.Intn(3)+1, rand.Intn(3)+1, rand.Intn(5)+3, " ") + "\n\n"
	}
	
	return content
}

func generateCommentText() string {
	templates := []string{
		"Интересная статья!",
		"Спасибо за информацию!",
		"А есть ли данные по этому вопросу?",
		"Полностью согласен с автором",
		"Интересная точка зрения",
		"Можно подробнее об этом?",
		"Очень полезная информация",
		"Ждем продолжения!",
		"Есть что добавить к этому",
		"Спасибо за качественный материал",
	}
	
	return templates[rand.Intn(len(templates))]
}

func generateTags(count int) []string {
	var selected []string
	for i := 0; i < count && i < len(tagPool); i++ {
		tag := tagPool[rand.Intn(len(tagPool))]
		if !contains(selected, tag) {
			selected = append(selected, tag)
		}
	}
	return selected
}

func generateMediaURL(mediaType string) string {
	switch mediaType {
	case "image":
		return fmt.Sprintf("https://picsum.photos/800/600?random=%d", rand.Intn(1000))
	case "video":
		return "https://example.com/video/" + gofakeit.UUID()
	case "audio":
		return "https://example.com/audio/" + gofakeit.UUID()
	default:
		return "https://example.com/media/" + gofakeit.UUID()
	}
}

func randomTimeInPast(days int) time.Time {
	now := time.Now()
	past := now.Add(-time.Duration(rand.Intn(days*86400)) * time.Second)
	return past
}

func checkServerHealth() bool {
	url := config.APIURL + "/health"
	resp, err := http.Get(url)
	if err != nil {
		logger.Printf("Ошибка проверки здоровья сервера: %v", err)
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}

func loadExistingData() {
	logger.Printf("Загрузка существующих данных...")
	
	// Загружаем авторов
	resp, err := http.Get(config.APIURL + "/api/authors")
	if err == nil {
		defer resp.Body.Close()
		var authors []map[string]interface{}
		if json.NewDecoder(resp.Body).Decode(&authors) == nil {
			for _, author := range authors {
				if id, ok := author["author_id"].(float64); ok {
					authorIDs = append(authorIDs, int(id))
				}
			}
		}
	}
	
	// Загружаем каналы
	resp, err = http.Get(config.APIURL + "/api/channels")
	if err == nil {
		defer resp.Body.Close()
		var channels []map[string]interface{}
		if json.NewDecoder(resp.Body).Decode(&channels) == nil {
			for _, channel := range channels {
				if id, ok := channel["channel_id"].(float64); ok {
					channelIDs = append(channelIDs, int(id))
				}
			}
		}
	}
	
	// Загружаем источники
	resp, err = http.Get(config.APIURL + "/api/sources")
	if err == nil {
		defer resp.Body.Close()
		var sources []map[string]interface{}
		if json.NewDecoder(resp.Body).Decode(&sources) == nil {
			for _, source := range sources {
				if id, ok := source["source_id"].(float64); ok {
					sourceIDs = append(sourceIDs, int(id))
				}
			}
		}
	}
	
	logger.Printf("Загружено: %d авторов, %d каналов, %d источников", 
		len(authorIDs), len(channelIDs), len(sourceIDs))
}

func showStats() {
	stats.Lock()
	defer stats.Unlock()
	
	elapsed := time.Since(stats.StartTime)
	
	logger.Printf("\n📊 СТАТИСТИКА ГЕНЕРАЦИИ:")
	logger.Printf("   Время работы: %v", elapsed.Round(time.Second))
	logger.Printf("   Источники: %d", stats.SourcesCreated)
	logger.Printf("   Авторы: %d", stats.AuthorsCreated)
	logger.Printf("   Каналы: %d", stats.ChannelsCreated)
	logger.Printf("   Посты: %d", stats.PostsCreated)
	logger.Printf("   Теги: %d", stats.TagsCreated)
	logger.Printf("   Комментарии: %d", stats.CommentsCreated)
	logger.Printf("   Медиа: %d", stats.MediaCreated)
	logger.Printf("   Ошибки: %d", stats.Errors)
	logger.Printf("   Всего записей: %d", 
		stats.SourcesCreated + stats.AuthorsCreated + stats.ChannelsCreated + 
		stats.PostsCreated + stats.TagsCreated + stats.CommentsCreated + stats.MediaCreated)
}

func showFinalStats() {
	showStats()
	
	// Показываем общую статистику базы
	logger.Printf("\n📈 ОБЩАЯ СТАТИСТИКА БАЗЫ:")
	
	endpoints := []string{
		"/api/sources", "/api/authors", "/api/channels", 
		"/api/posts", "/api/tags", "/api/comments", "/api/media",
	}
	
	for _, endpoint := range endpoints {
		url := config.APIURL + endpoint
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		
		var data []interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
			tableName := strings.TrimPrefix(endpoint, "/api/")
			logger.Printf("   %s: %d", tableName, len(data))
		}
		resp.Body.Close()
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}