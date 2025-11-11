package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "news_user"
	password = "news_pass"
	dbname   = "news_db"
)

func main() {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully connected to PostgreSQL!")

	// Генерируем данные
	rand.Seed(time.Now().UnixNano())

	// Вставляем источники
	sourceIDs := insertSources(db, 50)

	// Вставляем каналы
	channelIDs := insertChannels(db, sourceIDs, 10000)

	// Вставляем авторов
	authorIDs := insertAuthors(db, 100000000)

	// Вставляем тексты новостей
	textIDs := insertNewsTexts(db, 200000000)

	// Вставляем посты
	postIDs := insertPosts(db, authorIDs, textIDs, channelIDs, 3000)

	// Вставляем медиа
	insertMedia(db, postIDs, 5000)

	// Вставляем комментарии
	insertComments(db, postIDs, 10000)

	// Вставляем теги
	tagIDs := insertTags(db, 150)

	// Вставляем связи постов и тегов
	insertPostTags(db, postIDs, tagIDs, 60000)
}

// Генерация случайной строки
func randomString(length int) string {
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, length)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

// Генерация случайного текста
func randomText(length int) string {
	words := []string{"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit", "sed", "do", "eiusmod", "tempor", "incididunt", "ut", "labore", "et", "dolore", "magna", "aliqua"}
	result := make([]string, length)
	for i := range result {
		result[i] = words[rand.Intn(len(words))]
	}
	return strings.Join(result, " ")
}

// Вставка источников
func insertSources(db *sql.DB, count int) []int {
	var ids []int
	for i := 0; i < count; i++ {
		name := "Source " + randomString(5)
		address := "https://" + randomString(10) + ".com"
		topic := randomString(10)
		var id int
		err := db.QueryRow("INSERT INTO sources (name, address, topic) VALUES ($1, $2, $3) RETURNING source_id", name, address, topic).Scan(&id)
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, id)
	}
	fmt.Printf("Inserted %d sources\n", count)
	return ids
}

// Вставка каналов
func insertChannels(db *sql.DB, sourceIDs []int, count int) []int {
	var ids []int
	for i := 0; i < count; i++ {
		name := "Channel " + randomString(5)
		link := "https://t.me/" + randomString(10)
		subscribers := rand.Intn(10000)
		sourceID := sourceIDs[rand.Intn(len(sourceIDs))]
		topic := randomString(10)
		var id int
		err := db.QueryRow("INSERT INTO channels (name, link, subscribers_count, source_id, topic) VALUES ($1, $2, $3, $4, $5) RETURNING channel_id", name, link, subscribers, sourceID, topic).Scan(&id)
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, id)
	}
	fmt.Printf("Inserted %d channels\n", count)
	return ids
}

// Вставка авторов
func insertAuthors(db *sql.DB, count int) []int {
	var ids []int
	for i := 0; i < count; i++ {
		name := "Author " + randomString(8)
		var id int
		err := db.QueryRow("INSERT INTO authors (name) VALUES ($1) RETURNING author_id", name).Scan(&id)
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, id)
	}
	fmt.Printf("Inserted %d authors\n", count)
	return ids
}

// Вставка текстов новостей
func insertNewsTexts(db *sql.DB, count int) []int {
	var ids []int
	for i := 0; i < count; i++ {
		text := randomText(50)
		var id int
		err := db.QueryRow("INSERT INTO news_texts (text) VALUES ($1) RETURNING text_id", text).Scan(&id)
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, id)
	}
	fmt.Printf("Inserted %d news texts\n", count)
	return ids
}

// Вставка постов
func insertPosts(db *sql.DB, authorIDs, textIDs, channelIDs []int, count int) []int {
	var ids []int
	for i := 0; i < count; i++ {
		title := "Post " + randomString(10)
		authorID := authorIDs[rand.Intn(len(authorIDs))]
		textID := textIDs[rand.Intn(len(textIDs))]
		channelID := channelIDs[rand.Intn(len(channelIDs))]
		commentsCount := rand.Intn(100)
		likesCount := rand.Intn(1000)
		var id int
		err := db.QueryRow("INSERT INTO posts (title, author_id, text_id, channel_id, comments_count, likes_count) VALUES ($1, $2, $3, $4, $5, $6) RETURNING post_id", title, authorID, textID, channelID, commentsCount, likesCount).Scan(&id)
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, id)
	}
	fmt.Printf("Inserted %d posts\n", count)
	return ids
}

// Вставка медиа
func insertMedia(db *sql.DB, postIDs []int, count int) {
	for i := 0; i < count; i++ {
		postID := postIDs[rand.Intn(len(postIDs))]
		mediaContent := "https://example.com/media/" + randomString(10) + ".jpg"
		mediaType := "image"
		if rand.Intn(2) == 1 {
			mediaType = "video"
		}
		_, err := db.Exec("INSERT INTO media (post_id, media_content, media_type) VALUES ($1, $2, $3)", postID, mediaContent, mediaType)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("Inserted %d media\n", count)
}

// Вставка комментариев
func insertComments(db *sql.DB, postIDs []int, count int) {
	for i := 0; i < count; i++ {
		postID := postIDs[rand.Intn(len(postIDs))]
		nickname := "User" + randomString(5)
		parentCommentID := sql.NullInt64{}
		if rand.Intn(3) == 0 { // 1/3 шанс на родительский комментарий
			// Для простоты, не будем связывать, или можно получить существующий comment_id
			// Но чтобы упростить, оставим NULL
		}
		text := randomText(20)
		likesCount := rand.Intn(50)
		_, err := db.Exec("INSERT INTO comments (post_id, nickname, parent_comment_id, text, likes_count) VALUES ($1, $2, $3, $4, $5)", postID, nickname, parentCommentID, text, likesCount)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("Inserted %d comments\n", count)
}

// Вставка тегов
func insertTags(db *sql.DB, count int) []int {
	var ids []int
	for i := 0; i < count; i++ {
		name := "#" + randomString(6)
		var id int
		err := db.QueryRow("INSERT INTO tags (name) VALUES ($1) RETURNING tag_id", name).Scan(&id)
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, id)
	}
	fmt.Printf("Inserted %d tags\n", count)
	return ids
}

// Вставка связей постов и тегов
func insertPostTags(db *sql.DB, postIDs, tagIDs []int, count int) {
	for i := 0; i < count; i++ {
		postID := postIDs[rand.Intn(len(postIDs))]
		tagID := tagIDs[rand.Intn(len(tagIDs))]
		_, err := db.Exec("INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", postID, tagID)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("Inserted %d post-tag relations\n", count)
}
