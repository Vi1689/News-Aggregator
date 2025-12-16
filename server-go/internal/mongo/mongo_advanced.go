package mongo

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// ============================================
// СТРУКТУРЫ ДЛЯ СВЯЗЕЙ
// ============================================

// Post с встроенными комментариями (1:N встраивание)
type Post struct {
	PostID    int       `bson:"post_id"`
	Title     string    `bson:"title"`
	Content   string    `bson:"content"`
	ChannelID int       `bson:"channel_id"` // Ссылка на Channel (1:N)
	AuthorID  int       `bson:"author_id,omitempty"`
	Tags      []string  `bson:"tags"` // Связь M:N через массив
	Comments  []Comment `bson:"comments"`
	Stats     Stats     `bson:"stats"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

type Comment struct {
	CommentID       int       `bson:"comment_id"`
	Nickname        string    `bson:"nickname"`
	Text            string    `bson:"text"`
	LikesCount      int       `bson:"likes_count"`
	CreatedAt       time.Time `bson:"created_at"`
	ParentCommentID *int      `bson:"parent_comment_id,omitempty"`
}

type Stats struct {
	Views  int `bson:"views"`
	Likes  int `bson:"likes"`
	Shares int `bson:"shares"`
}

type Channel struct {
	ChannelID        int       `bson:"channel_id"`
	Name             string    `bson:"name"`
	SourceID         int       `bson:"source_id"`
	SubscribersCount int       `bson:"subscribers_count"`
	Topic            string    `bson:"topic"`
	PostCount        int       `bson:"post_count,omitempty"`
	LastPostDate     time.Time `bson:"last_post_date,omitempty"`
	CreatedAt        time.Time `bson:"created_at"`
}

type Tag struct {
	TagID      int       `bson:"tag_id"`
	Name       string    `bson:"name"`
	UsageCount int       `bson:"usage_count"`
	CreatedAt  time.Time `bson:"created_at"`
}

// ============================================
// ТРАНЗАКЦИИ
// ============================================

// CreatePostWithTransaction создает пост с обновлением статистики канала (многошаговая транзакция)
func (m *MongoManager) CreatePostWithTransaction(ctx context.Context, post Post) error {
	session, err := m.client.StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	// Callback функция для транзакции
	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		posts := m.db.Collection("posts")
		channels := m.db.Collection("channels")
		tags := m.db.Collection("tags")

		// Шаг 1: Вставка поста
		post.CreatedAt = time.Now()
		post.UpdatedAt = time.Now()
		if post.Stats.Views == 0 && post.Stats.Likes == 0 && post.Stats.Shares == 0 {
			post.Stats = Stats{Views: 0, Likes: 0, Shares: 0}
		}

		_, err := posts.InsertOne(sessCtx, post)
		if err != nil {
			return nil, fmt.Errorf("failed to insert post: %w", err)
		}

		// Шаг 2: Обновление счетчика постов в канале
		_, err = channels.UpdateOne(sessCtx,
			bson.M{"channel_id": post.ChannelID},
			bson.M{
				"$inc": bson.M{"post_count": 1},
				"$set": bson.M{"last_post_date": time.Now()},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update channel: %w", err)
		}

		// Шаг 3: Обновление счетчика использования тегов
		if len(post.Tags) > 0 {
			_, err = tags.UpdateMany(sessCtx,
				bson.M{"name": bson.M{"$in": post.Tags}},
				bson.M{"$inc": bson.M{"usage_count": 1}},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to update tags: %w", err)
			}
		}

		log.Printf("✅ Transaction completed for post %d", post.PostID)
		return nil, nil
	}

	// Опции транзакции
	txnOptions := options.Transaction().
		SetReadConcern(readconcern.Snapshot()).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority())).
		SetReadPreference(readpref.Primary())

	_, err = session.WithTransaction(ctx, callback, txnOptions)
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// ============================================
// BULK ОПЕРАЦИИ
// ============================================

// BulkUpdatePosts выполняет пакетные операции над постами
func (m *MongoManager) BulkUpdatePosts(ctx context.Context) (*mongo.BulkWriteResult, error) {
	posts := m.db.Collection("posts")

	models := []mongo.WriteModel{
		// Update: увеличение просмотров
		mongo.NewUpdateOneModel().
			SetFilter(bson.M{"post_id": 1}).
			SetUpdate(bson.M{
				"$inc": bson.M{
					"stats.views": 100,
					"stats.likes": 5,
				},
				"$set": bson.M{"updated_at": time.Now()},
			}),

		// Insert: новый пост
		mongo.NewInsertOneModel().SetDocument(Post{
			PostID:    9999,
			Title:     "Bulk операция",
			Content:   "Создан через bulkWrite",
			ChannelID: 1,
			Tags:      []string{"bulk", "mongodb"},
			Comments:  []Comment{},
			Stats:     Stats{Views: 0, Likes: 0, Shares: 0},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}),

		// UpdateMany: обновление всех постов по тегу
		mongo.NewUpdateManyModel().
			SetFilter(bson.M{"tags": "технологии"}).
			SetUpdate(bson.M{
				"$inc": bson.M{"stats.views": 10},
				"$set": bson.M{"trending": true},
			}),

		// DeleteMany: удаление старых постов
		mongo.NewDeleteManyModel().SetFilter(bson.M{
			"created_at":  bson.M{"$lt": time.Now().AddDate(-1, 0, 0)},
			"stats.views": bson.M{"$lt": 100},
		}),

		// ReplaceOne с upsert
		mongo.NewReplaceOneModel().
			SetFilter(bson.M{"post_id": 10000}).
			SetReplacement(Post{
				PostID:    10000,
				Title:     "Заменен через bulk",
				Content:   "Полная замена документа",
				ChannelID: 2,
				Tags:      []string{"обновление"},
				Comments:  []Comment{},
				Stats:     Stats{Views: 0, Likes: 0, Shares: 0},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).
			SetUpsert(true),
	}

	opts := options.BulkWrite().SetOrdered(false)
	result, err := posts.BulkWrite(ctx, models, opts)
	if err != nil {
		return nil, fmt.Errorf("bulk write failed: %w", err)
	}

	log.Printf("📊 Bulk operations result:")
	log.Printf("  Inserted: %d", result.InsertedCount)
	log.Printf("  Modified: %d", result.ModifiedCount)
	log.Printf("  Deleted: %d", result.DeletedCount)
	log.Printf("  Upserted: %d", result.UpsertedCount)

	return result, nil
}

// ============================================
// КОМБИНИРОВАННЫЕ ОТЧЕТЫ
// ============================================

type WeeklyReport struct {
	BySource []SourceStats  `bson:"by_source"`
	ByTopic  []TopicStats   `bson:"by_topic"`
	ByDay    []DayStats     `bson:"by_day"`
	Summary  []SummaryStats `bson:"summary"`
}

type SourceStats struct {
	SourceID      int     `bson:"_id"`
	SourceName    string  `bson:"source_name"`
	TotalPosts    int     `bson:"total_posts"`
	TotalViews    int     `bson:"total_views"`
	TotalLikes    int     `bson:"total_likes"`
	AvgEngagement float64 `bson:"avg_engagement"`
}

type TopicStats struct {
	Topic           string `bson:"topic"`
	PostCount       int    `bson:"post_count"`
	TotalEngagement int    `bson:"total_engagement"`
}

type DayStats struct {
	Day      int      `bson:"_id"`
	Count    int      `bson:"count"`
	AvgLikes float64  `bson:"avg_likes"`
	Posts    []string `bson:"posts"`
}

type SummaryStats struct {
	TotalPosts          int     `bson:"total_posts"`
	UniqueChannelsCount int     `bson:"unique_channels_count"`
	UniqueTagsCount     int     `bson:"unique_tags_count"`
	TotalViews          int     `bson:"total_views"`
	TotalEngagement     int     `bson:"total_engagement"`
	AvgViewsPerPost     float64 `bson:"avg_views_per_post"`
}

// GetWeeklyReport генерирует сложный отчет с $facet, $lookup, $bucket
func (m *MongoManager) GetWeeklyReport(ctx context.Context) (*WeeklyReport, error) {
	posts := m.db.Collection("posts")

	weekAgo := time.Now().AddDate(0, 0, -7)

	pipeline := mongo.Pipeline{
		// Фильтр по дате
		{{Key: "$match", Value: bson.M{
			"created_at": bson.M{"$gte": weekAgo},
		}}},

		// JOIN с каналами
		{{Key: "$lookup", Value: bson.M{
			"from":         "channels",
			"localField":   "channel_id",
			"foreignField": "channel_id",
			"as":           "channel_info",
		}}},

		{{Key: "$unwind", Value: bson.M{
			"path":                       "$channel_info",
			"preserveNullAndEmptyArrays": false,
		}}},

		// Развертывание тегов
		{{Key: "$unwind", Value: bson.M{
			"path":                       "$tags",
			"preserveNullAndEmptyArrays": true,
		}}},

		// Многоуровневая аналитика
		{{Key: "$facet", Value: bson.M{
			"by_source": []bson.M{
				{
					"$group": bson.M{
						"_id":         "$channel_info.source_id",
						"source_name": bson.M{"$first": "$channel_info.name"},
						"total_posts": bson.M{"$sum": 1},
						"total_views": bson.M{"$sum": "$stats.views"},
						"total_likes": bson.M{"$sum": "$stats.likes"},
						"avg_engagement": bson.M{
							"$avg": bson.M{
								"$divide": []interface{}{
									bson.M{"$add": []string{"$stats.likes", "$stats.shares"}},
									bson.M{"$max": []interface{}{"$stats.views", 1}},
								},
							},
						},
					},
				},
				{"$sort": bson.M{"total_posts": -1}},
				{"$limit": 10},
			},

			"by_topic": []bson.M{
				{
					"$group": bson.M{
						"_id":              "$tags",
						"topic":            bson.M{"$first": "$tags"},
						"post_count":       bson.M{"$sum": 1},
						"total_engagement": bson.M{"$sum": bson.M{"$add": []string{"$stats.likes", "$stats.shares"}}},
					},
				},
				{"$sort": bson.M{"post_count": -1}},
				{"$limit": 20},
			},

			"by_day": []bson.M{
				{
					"$bucket": bson.M{
						"groupBy":    bson.M{"$dayOfWeek": "$created_at"},
						"boundaries": []int{1, 2, 3, 4, 5, 6, 7, 8},
						"default":    "other",
						"output": bson.M{
							"count":     bson.M{"$sum": 1},
							"avg_likes": bson.M{"$avg": "$stats.likes"},
							"posts":     bson.M{"$push": "$title"},
						},
					},
				},
			},

			"summary": []bson.M{
				{
					"$group": bson.M{
						"_id":              nil,
						"total_posts":      bson.M{"$sum": 1},
						"unique_channels":  bson.M{"$addToSet": "$channel_id"},
						"unique_tags":      bson.M{"$addToSet": "$tags"},
						"total_views":      bson.M{"$sum": "$stats.views"},
						"total_engagement": bson.M{"$sum": bson.M{"$add": []string{"$stats.likes", "$stats.shares"}}},
					},
				},
				{
					"$project": bson.M{
						"_id":                   0,
						"total_posts":           1,
						"unique_channels_count": bson.M{"$size": "$unique_channels"},
						"unique_tags_count":     bson.M{"$size": "$unique_tags"},
						"total_views":           1,
						"total_engagement":      1,
						"avg_views_per_post":    bson.M{"$divide": []string{"$total_views", "$total_posts"}},
					},
				},
			},
		}}},
	}

	cursor, err := posts.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregation failed: %w", err)
	}
	defer cursor.Close(ctx)

	var results []WeeklyReport
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	if len(results) == 0 {
		return &WeeklyReport{}, nil
	}

	return &results[0], nil
}

// ============================================
// КЭШИРОВАНИЕ
// ============================================

type CachedChannelReport struct {
	ChannelID       int       `bson:"channel_id"`
	ChannelName     string    `bson:"channel_name"`
	TotalPosts      int       `bson:"total_posts"`
	TotalViews      int       `bson:"total_views"`
	TotalLikes      int       `bson:"total_likes"`
	AvgLikesPerPost float64   `bson:"avg_likes_per_post"`
	TopTags         []string  `bson:"top_tags"`
	LastPostDate    time.Time `bson:"last_post_date"`
	EngagementRate  float64   `bson:"engagement_rate"`
	CachedAt        time.Time `bson:"cached_at"`
}

// MaterializeChannelReports создает/обновляет кэш отчетов по каналам
func (m *MongoManager) MaterializeChannelReports(ctx context.Context) error {
	posts := m.db.Collection("posts")
	// cachedReports := m.db.Collection("cached_channel_reports")

	pipeline := mongo.Pipeline{
		{{Key: "$lookup", Value: bson.M{
			"from":         "channels",
			"localField":   "channel_id",
			"foreignField": "channel_id",
			"as":           "channel",
		}}},

		{{Key: "$unwind", Value: "$channel"}},

		{{Key: "$group", Value: bson.M{
			"_id":                "$channel_id",
			"channel_name":       bson.M{"$first": "$channel.name"},
			"total_posts":        bson.M{"$sum": 1},
			"total_views":        bson.M{"$sum": "$stats.views"},
			"total_likes":        bson.M{"$sum": "$stats.likes"},
			"avg_likes_per_post": bson.M{"$avg": "$stats.likes"},
			"top_tags":           bson.M{"$push": "$tags"},
			"last_post_date":     bson.M{"$max": "$created_at"},
		}}},

		{{Key: "$project", Value: bson.M{
			"channel_id":         "$_id",
			"channel_name":       1,
			"total_posts":        1,
			"total_views":        1,
			"total_likes":        1,
			"avg_likes_per_post": bson.M{"$round": []interface{}{"$avg_likes_per_post", 2}},
			"top_tags": bson.M{
				"$slice": []interface{}{
					bson.M{
						"$reduce": bson.M{
							"input":        "$top_tags",
							"initialValue": []interface{}{},
							"in":           bson.M{"$setUnion": []string{"$$value", "$$this"}},
						},
					},
					10,
				},
			},
			"last_post_date": 1,
			"engagement_rate": bson.M{
				"$round": []interface{}{
					bson.M{
						"$multiply": []interface{}{
							bson.M{
								"$divide": []interface{}{
									"$total_likes",
									bson.M{"$max": []interface{}{"$total_views", 1}},
								},
							},
							100,
						},
					},
					2,
				},
			},
			"cached_at": time.Now(),
			"_id":       0,
		}}},

		{{Key: "$out", Value: "cached_channel_reports"}},
	}

	_, err := posts.Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("failed to materialize reports: %w", err)
	}

	log.Println("✅ Channel reports cache updated")
	return nil
}

// GetCachedChannelReports получает данные из кэша
func (m *MongoManager) GetCachedChannelReports(ctx context.Context, limit int) ([]CachedChannelReport, error) {
	cachedReports := m.db.Collection("cached_channel_reports")

	opts := options.Find().
		SetSort(bson.D{{Key: "engagement_rate", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := cachedReports.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cached reports: %w", err)
	}
	defer cursor.Close(ctx)

	var reports []CachedChannelReport
	if err := cursor.All(ctx, &reports); err != nil {
		return nil, fmt.Errorf("failed to decode cached reports: %w", err)
	}

	return reports, nil
}

// StartCacheInvalidationWatcher запускает Change Stream для автоматического обновления кэша
func (m *MongoManager) StartCacheInvalidationWatcher(ctx context.Context) {
	posts := m.db.Collection("posts")

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"operationType": bson.M{"$in": []string{"insert", "update", "delete"}},
		}}},
	}

	stream, err := posts.Watch(ctx, pipeline)
	if err != nil {
		log.Printf("❌ Failed to start change stream: %v", err)
		return
	}
	defer stream.Close(ctx)

	log.Println("👁️ Cache invalidation watcher started")

	for stream.Next(ctx) {
		var event bson.M
		if err := stream.Decode(&event); err != nil {
			log.Printf("Failed to decode change event: %v", err)
			continue
		}

		log.Printf("⚠️ Data changed, invalidating cache...")
		log.Printf("  Operation: %v", event["operationType"])

		// Обновляем кэш
		if err := m.MaterializeChannelReports(ctx); err != nil {
			log.Printf("Failed to update cache: %v", err)
		}
	}

	if err := stream.Err(); err != nil {
		log.Printf("Change stream error: %v", err)
	}
}
