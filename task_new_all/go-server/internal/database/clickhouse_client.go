package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"news-aggregator/internal/models"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouseClient struct {
	Conn driver.Conn
}

func NewClickHouseClient(host, port, user, password, database string) (*ClickHouseClient, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", host, port)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: user,
			Password: password,
		},
		DialTimeout: 10 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	log.Println("Connected to ClickHouse successfully")

	return &ClickHouseClient{Conn: conn}, nil
}

func (c *ClickHouseClient) Close() {
	c.Conn.Close()
}

// Вставка события в ClickHouse
func (c *ClickHouseClient) InsertEvent(ctx context.Context, event models.EventWrapper) error {
	query := `
        INSERT INTO news_events_raw (
            eventId, eventType, entityId, timestamp, source, version,
            newsId, title, summary, fullText, author, authorType, categories, tags, published_at,
            userId, readDurationSec, likeValue, platform, sharedToUserId, commentText, parentCommentId
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	var newsID, title, summary, fullText, author, authorType, userID, platform, sharedToUserID, commentText, parentCommentID string
	var categories, tags []string
	var publishedAt time.Time
	var readDurationSec uint32
	var likeValue int8

	// Извлекаем данные из payload в зависимости от типа события
	switch event.EventType {
	case "NewsPublished":
		if payload, ok := event.Payload["newsId"]; ok {
			newsID = payload.(string)
		}
		if payload, ok := event.Payload["title"]; ok {
			title = payload.(string)
		}
		if payload, ok := event.Payload["summary"]; ok {
			summary = payload.(string)
		}
		if payload, ok := event.Payload["fullText"]; ok {
			fullText = payload.(string)
		}
		if payload, ok := event.Payload["author"]; ok {
			author = payload.(string)
		}
		if payload, ok := event.Payload["authorType"]; ok {
			authorType = payload.(string)
		}
		if payload, ok := event.Payload["categories"]; ok {
			if cats, ok := payload.([]interface{}); ok {
				for _, cat := range cats {
					categories = append(categories, cat.(string))
				}
			}
		}
		if payload, ok := event.Payload["tags"]; ok {
			if tagList, ok := payload.([]interface{}); ok {
				for _, tag := range tagList {
					tags = append(tags, tag.(string))
				}
			}
		}
		if payload, ok := event.Payload["publishedAt"]; ok {
			publishedAt = payload.(time.Time)
		}

	case "NewsViewed":
		if payload, ok := event.Payload["newsId"]; ok {
			newsID = payload.(string)
		}
		if payload, ok := event.Payload["userId"]; ok {
			userID = payload.(string)
		}
		if payload, ok := event.Payload["readDurationSec"]; ok {
			readDurationSec = uint32(payload.(int))
		}

	case "NewsLiked":
		if payload, ok := event.Payload["newsId"]; ok {
			newsID = payload.(string)
		}
		if payload, ok := event.Payload["userId"]; ok {
			userID = payload.(string)
		}
		if payload, ok := event.Payload["likeValue"]; ok {
			likeValue = int8(payload.(int))
		}

	case "NewsShared":
		if payload, ok := event.Payload["newsId"]; ok {
			newsID = payload.(string)
		}
		if payload, ok := event.Payload["userId"]; ok {
			userID = payload.(string)
		}
		if payload, ok := event.Payload["platform"]; ok {
			platform = payload.(string)
		}
		if payload, ok := event.Payload["sharedToUserId"]; ok {
			sharedToUserID = payload.(string)
		}
	}

	err := c.Conn.Exec(ctx, query,
		event.EventID,
		event.EventType,
		event.EntityID,
		event.Timestamp,
		event.Source,
		event.Version,
		newsID,
		title,
		summary,
		fullText,
		author,
		authorType,
		categories,
		tags,
		publishedAt,
		userID,
		readDurationSec,
		likeValue,
		platform,
		sharedToUserID,
		commentText,
		parentCommentID,
	)

	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// Аналитический запрос 1: Топ новостей по просмотрам
func (c *ClickHouseClient) GetTopNewsByViews(ctx context.Context, days int, limit int) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            newsId,
            any(title) AS title,
            sum(views) AS total_views,
            sum(likes) AS total_likes,
            if(total_views > 0, total_likes / total_views, 0) AS like_rate
        FROM news_daily_stats
        WHERE date >= today() - $1
        GROUP BY newsId
        ORDER BY total_views DESC
        LIMIT $2
    `

	rows, err := c.Conn.Query(ctx, query, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var newsID, title string
		var views, likes int
		var likeRate float64

		if err := rows.Scan(&newsID, &title, &views, &likes, &likeRate); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"news_id":   newsID,
			"title":     title,
			"views":     views,
			"likes":     likes,
			"like_rate": likeRate,
		})
	}

	return results, nil
}

// Аналитический запрос 2: Популярность тегов за период
func (c *ClickHouseClient) GetPopularTags(ctx context.Context, hours int, limit int) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            tag,
            sum(mentions_count) AS total_mentions,
            sum(unique_news) AS unique_news_count
        FROM tag_hourly_popularity
        WHERE hour >= now() - INTERVAL $1 HOUR
        GROUP BY tag
        ORDER BY total_mentions DESC
        LIMIT $2
    `

	rows, err := c.Conn.Query(ctx, query, hours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var tag string
		var mentions, uniqueNews int

		if err := rows.Scan(&tag, &mentions, &uniqueNews); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"tag":         tag,
			"mentions":    mentions,
			"unique_news": uniqueNews,
		})
	}

	return results, nil
}

// Аналитический запрос 3: Активность пользователей по часам
func (c *ClickHouseClient) GetUserActivityByHour(ctx context.Context, date time.Time) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            toHour(timestamp) AS hour,
            eventType,
            count() AS events_count,
            uniq(userId) AS unique_users
        FROM news_events_raw
        WHERE toDate(timestamp) = $1
          AND eventType IN ('NewsViewed', 'NewsLiked', 'NewsShared')
          AND userId != ''
        GROUP BY hour, eventType
        ORDER BY hour ASC
    `

	rows, err := c.Conn.Query(ctx, query, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var hour int
		var eventType string
		var eventsCount, uniqueUsers int

		if err := rows.Scan(&hour, &eventType, &eventsCount, &uniqueUsers); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"hour":         hour,
			"event_type":   eventType,
			"events_count": eventsCount,
			"unique_users": uniqueUsers,
		})
	}

	return results, nil
}

// Аналитический запрос 4: Статистика по источникам (авторам)
func (c *ClickHouseClient) GetAuthorStats(ctx context.Context, days int) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            author,
            authorType,
            countIf(eventType = 'NewsPublished') AS news_published,
            sumIf(readDurationSec, eventType = 'NewsViewed') AS total_read_time,
            countIf(eventType = 'NewsLiked') AS total_likes
        FROM news_events_raw
        WHERE timestamp >= now() - INTERVAL $1 DAY
        GROUP BY author, authorType
        HAVING news_published > 0
        ORDER BY news_published DESC
        LIMIT 20
    `

	rows, err := c.Conn.Query(ctx, query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var author, authorType string
		var newsPublished, totalLikes int
		var totalReadTime uint64

		if err := rows.Scan(&author, &authorType, &newsPublished, &totalReadTime, &totalLikes); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"author":          author,
			"author_type":     authorType,
			"news_published":  newsPublished,
			"total_read_time": totalReadTime,
			"total_likes":     totalLikes,
		})
	}

	return results, nil
}

// Аналитический запрос 5: Retention пользователей
func (c *ClickHouseClient) GetUserRetention(ctx context.Context, days int) ([]map[string]interface{}, error) {
	query := `
        WITH 
            first_activity AS (
                SELECT userId, min(toDate(timestamp)) AS first_date
                FROM news_events_raw
                WHERE userId != '' AND eventType = 'NewsViewed'
                GROUP BY userId
            ),
            daily_activity AS (
                SELECT DISTINCT toDate(timestamp) AS date, userId
                FROM news_events_raw
                WHERE userId != '' AND eventType = 'NewsViewed'
            )
        SELECT 
            first_date,
            count(DISTINCT fa.userId) AS new_users,
            count(DISTINCT da.userId) AS active_users,
            round(active_users * 100.0 / new_users, 2) AS retention_rate
        FROM first_activity fa
        LEFT JOIN daily_activity da ON fa.userId = da.userId AND da.date = fa.first_date + INTERVAL 1 DAY
        WHERE first_date >= today() - $1
        GROUP BY first_date
        ORDER BY first_date DESC
    `

	rows, err := c.Conn.Query(ctx, query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var firstDate time.Time
		var newUsers, activeUsers int
		var retentionRate float64

		if err := rows.Scan(&firstDate, &newUsers, &activeUsers, &retentionRate); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"date":           firstDate,
			"new_users":      newUsers,
			"active_users":   activeUsers,
			"retention_rate": retentionRate,
		})
	}

	return results, nil
}

// Аналитический запрос 6: Анализ вовлечённости по времени суток
func (c *ClickHouseClient) GetEngagementByHour(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            toHour(timestamp) AS hour,
            eventType,
            count() AS events,
            uniq(userId) AS unique_users,
            uniq(newsId) AS unique_news
        FROM news_events_raw
        WHERE timestamp >= today() - 7
          AND eventType IN ('NewsViewed', 'NewsLiked', 'NewsShared')
        GROUP BY hour, eventType
        ORDER BY hour ASC, eventType
    `

	rows, err := c.Conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var hour int
		var eventType string
		var events, uniqueUsers, uniqueNews int

		if err := rows.Scan(&hour, &eventType, &events, &uniqueUsers, &uniqueNews); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"hour":         hour,
			"event_type":   eventType,
			"events":       events,
			"unique_users": uniqueUsers,
			"unique_news":  uniqueNews,
		})
	}

	return results, nil
}

// 7. Распределение лайков по категориям
func (c *ClickHouseClient) GetLikesByCategory(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            cat,
            count() AS total_likes,
            uniq(newsId) AS unique_news
        FROM news_events_raw
        ARRAY JOIN categories AS cat
        WHERE eventType = 'NewsLiked' AND likeValue = 1
        GROUP BY cat
        ORDER BY total_likes DESC
        LIMIT 20
    `

	rows, err := c.Conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var category string
		var likes, uniqueNews int
		if err := rows.Scan(&category, &likes, &uniqueNews); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"category":    category,
			"total_likes": likes,
			"unique_news": uniqueNews,
		})
	}
	return results, nil
}

// 8. Пользователи с наибольшим временем чтения
func (c *ClickHouseClient) GetTopReaders(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            userId,
            count() AS read_events,
            sum(readDurationSec) AS total_read_time,
            uniq(newsId) AS unique_news_read
        FROM news_events_raw
        WHERE eventType = 'NewsViewed' AND userId != ''
        GROUP BY userId
        ORDER BY total_read_time DESC
        LIMIT $1
    `

	rows, err := c.Conn.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var userID string
		var readEvents, totalReadTime, uniqueNews int
		if err := rows.Scan(&userID, &readEvents, &totalReadTime, &uniqueNews); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"user_id":         userID,
			"read_events":     readEvents,
			"total_read_time": totalReadTime,
			"unique_news":     uniqueNews,
		})
	}
	return results, nil
}
