package validators

import (
	"context"
	"database/sql"
	"fmt"
	"news-aggregator/internal/pgpool"
	"time"
)

// таблица функций валидации для таблиц
type ValidatorFunc func(ctx context.Context, conn *pgpool.PConn, data map[string]interface{}) error

type ValidatorRegistry struct {
	validators map[string]ValidatorFunc
	db         *sql.DB
}

func NewValidatorRegistry() *ValidatorRegistry {
	return &ValidatorRegistry{
		validators: make(map[string]ValidatorFunc),
	}
}

// Register регистрирует функцию валидации для таблицы
func (r *ValidatorRegistry) Register(tableName string, validatorFunc ValidatorFunc) {
	r.validators[tableName] = validatorFunc
}

// непосредственно запуск фукнции валидации
func (r *ValidatorRegistry) Validate(ctx context.Context, conn *pgpool.PConn, tableName string, data map[string]interface{}) error {
	validatorFunc, exists := r.validators[tableName]
	if !exists {
		return fmt.Errorf("validator for table '%s' not found", tableName)
	}
	return validatorFunc(ctx, conn, data)
}

// регистрация для каждой таблицы валидаторов
func InitValidatorRegistry() *ValidatorRegistry {
	registry := NewValidatorRegistry()

	// Регистрируем все валидаторы
	registry.Register("users", UsersValidator)
	registry.Register("posts", PostsValidator)
	registry.Register("authors", AuthorsValidator)
	registry.Register("channels", ChannelsValidator)
	registry.Register("sources", SourcesValidator)
	registry.Register("news_texts", DefaultValidator)
	registry.Register("media", DefaultValidator)
	registry.Register("tags", DefaultValidator)
	registry.Register("comments", DefaultValidator)

	return registry
}

// =========================== Функции валидации =============================

// проверка уникальности пользователей на каждой платформе
func UsersValidator(ctx context.Context, conn *pgpool.PConn, data map[string]interface{}) error {
	username, err := getString(data, "username")
	if err != nil {
		return err
	}

	// Проверка уникальности username + platform_id
	var exists int
	query := `SELECT COUNT(*) FROM users WHERE username = $1`
	err = conn.QueryRow(ctx, query, username).Scan(&exists)
	if err != nil {
		return err
	}

	if exists > 0 {
		return fmt.Errorf("user with username '%s' already exists", username)
	}

	return nil
}

// проверка уникальности постов
func PostsValidator(ctx context.Context, conn *pgpool.PConn, data map[string]interface{}) error {
	title, err := getString(data, "title")
	if err != nil {
		return err
	}

	textID, err := getInt(data, "text_id")
	if err != nil {
		return err
	}

	authorID, err := getInt(data, "author_id")
	if err != nil {
		return err
	}

	createdAt, err := getTime(data, "created_at")
	if err != nil {
		return fmt.Errorf("There is no creation time in post")
	}

	// Проверка уникальности по нескольким полям
	query := `SELECT COUNT(*) FROM posts 
              WHERE title = $1 AND text_id = $2 AND author_id = $3 AND created_at >= $4 AND created_at < $5`

	// Проверяем в интервале ±1 минута для created_at
	startTime := createdAt.Add(-time.Minute)
	endTime := createdAt.Add(time.Minute)

	var exists int
	err = conn.QueryRow(ctx, query, title, textID, authorID, startTime, endTime).Scan(&exists)

	if err != nil {
		return err
	}

	if exists > 0 {
		return fmt.Errorf("post with similar title, text, author and creation time already exists")
	}

	return nil
}

// AuthorsValidator - проверка уникальности авторов
func AuthorsValidator(ctx context.Context, conn *pgpool.PConn, data map[string]interface{}) error {
	name, err := getString(data, "name")
	if err != nil {
		return err
	}

	var exists int
	query := "SELECT EXISTS(SELECT 1 FROM authors WHERE name=$1)"
	err = conn.QueryRow(ctx, query, name).Scan(&exists)

	if err != nil {
		return err
	}

	if exists > 0 {
		return fmt.Errorf("author with name '%s' already exists", name)
	}

	return nil
}

// SourcesValidator - проверка уникальности источников
func SourcesValidator(ctx context.Context, conn *pgpool.PConn, data map[string]interface{}) error {
	name, err := getString(data, "name")
	if err != nil {
		return err
	}

	link, err := getString(data, "link")
	if err != nil {
		return err
	}

	var exists int
	query := "SELECT EXISTS(SELECT 1 FROM sources WHERE name=$1 AND link=$2)"
	err = conn.QueryRow(ctx, query, name, link).Scan(&exists)
	if err != nil {
		return err
	}

	if exists > 0 {
		return fmt.Errorf("source with name '%s' already exists", name)
	}

	return nil
}

// ChannelsValidator - проверка каналов
func ChannelsValidator(ctx context.Context, conn *pgpool.PConn, data map[string]interface{}) error {
	name, err := getString(data, "name")
	if err != nil {
		return err
	}

	source_id, err := getString(data, "source_id")
	if err != nil {
		return fmt.Errorf("channel has no source id")
	}

	var exists int
	query := "SELECT EXISTS(SELECT 1 FROM channels WHERE name=$1 AND source_id=$2)"
	err = conn.QueryRow(ctx, query, name, source_id).Scan(&exists)
	if err != nil {
		return err
	}

	if exists > 0 {
		return fmt.Errorf("channel with name '%s' already exists", name)
	}

	return nil
}

// заглушка, где валидация не нужна
func DefaultValidator(ctx context.Context, conn *pgpool.PConn, data map[string]interface{}) error {
	return nil
}

// ===================== вспомогательные функции ======================

func getString(data map[string]interface{}, key string) (string, error) {
	value, exists := data[key]
	if !exists {
		return "", fmt.Errorf("key '%s' not found in data", key)
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("key '%s' is not a string", key)
	}

	return str, nil
}

func getInt(data map[string]interface{}, key string) (int, error) {
	value, exists := data[key]
	if !exists {
		return 0, fmt.Errorf("key '%s' not found in data", key)
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("key '%s' is not an integer", key)
	}
}

func getTime(data map[string]interface{}, key string) (time.Time, error) {
	value, exists := data[key]
	if !exists {
		return time.Time{}, fmt.Errorf("key '%s' not found in data", key)
	}

	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		// Пробуем разные форматы времени
		formats := []string{
			time.RFC3339,          // "2006-01-02T15:04:05Z07:00"
			"2006-01-02 15:04:05", // ваш текущий формат
			"2006-01-02T15:04:05", // без часового пояса
			"2006-01-02",          // только дата
			time.RFC1123,          // "Mon, 02 Jan 2006 15:04:05 MST"
		}

		for _, format := range formats {
			parsedTime, err := time.Parse(format, v)
			if err == nil {
				return parsedTime, nil
			}
		}

		return time.Time{}, fmt.Errorf("key '%s' has invalid time format: %s", key, v)
	default:
		return time.Time{}, fmt.Errorf("key '%s' is not a valid time type", key)
	}
}
