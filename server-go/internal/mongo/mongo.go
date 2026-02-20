package mongo

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoManager struct {
	client *mongo.Client
	db     *mongo.Database
}

type SearchResult struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Preview     string   `json:"preview"`
	Relevance   float64  `json:"relevance"`
	MatchedTags []string `json:"matched_tags"`
}

func NewMongoManager(uri string) (*MongoManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(50).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(30 * time.Minute)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database("news_aggregator")
	mm := &MongoManager{
		client: client,
		db:     db,
	}

	// Создание коллекций и индексов
	if err := mm.createCollectionsAndIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	}

	log.Println("✓ MongoDB connected successfully")
	return mm, nil
}

func (m *MongoManager) createCollectionsAndIndexes(ctx context.Context) error {
	posts := m.db.Collection("posts")

	// Текстовый индекс для полнотекстового поиска
	_, err := posts.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "title", Value: "text"},
			{Key: "content", Value: "text"},
			{Key: "tags", Value: "text"},
		},
		Options: options.Index().
			SetWeights(bson.D{
				{Key: "title", Value: 10},
				{Key: "content", Value: 5},
				{Key: "tags", Value: 3},
			}).
			SetDefaultLanguage("russian"),
	})
	if err != nil {
		return err
	}

	// Unique индекс для post_id
	_, err = posts.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "post_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Unique индекс для content_hash
	_, err = posts.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "content_hash", Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true),
	})
	if err != nil {
		return err
	}

	// Составной индекс для поиска по тегам
	_, err = posts.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "tags", Value: 1},
			{Key: "stats.likes", Value: -1},
			{Key: "created_at", Value: -1},
		},
	})
	if err != nil {
		return err
	}

	// TTL индекс для автоудаления старых постов (1 год)
	_, err = posts.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(31536000),
	})
	// Добавляем индекс для AdvancedSearch
	_, err = posts.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "tags", Value: 1},
			{Key: "stats.likes", Value: -1},
		},
	})
	if err != nil {
		log.Printf("Warning: failed to create advanced search index: %v", err)
	}

	return err
}

// ============ CRUD ОПЕРАЦИИ ============

func (m *MongoManager) IndexPost(ctx context.Context, postID int, title, content string, tags []string) error {
	posts := m.db.Collection("posts")

	contentHash := fmt.Sprintf("%d", hashString(title+content))

	doc := bson.M{
		"post_id":      postID,
		"title":        title,
		"content":      content,
		"content_hash": contentHash,
		"tags":         tags,
		"stats": bson.M{
			"views":    0,
			"likes":    0,
			"comments": 0,
		},
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}

	_, err := posts.InsertOne(ctx, doc)
	return err
}

func (m *MongoManager) UpdatePostIndex(ctx context.Context, postID int, title, content string, tags []string) error {
	posts := m.db.Collection("posts")

	contentHash := fmt.Sprintf("%d", hashString(title+content))

	update := bson.M{
		"$set": bson.M{
			"title":        title,
			"content":      content,
			"content_hash": contentHash,
			"tags":         tags,
			"updated_at":   time.Now(),
		},
	}

	_, err := posts.UpdateOne(ctx, bson.M{"post_id": postID}, update)
	return err
}

func (m *MongoManager) RemovePostIndex(ctx context.Context, postID int) error {
	posts := m.db.Collection("posts")
	_, err := posts.DeleteOne(ctx, bson.M{"post_id": postID})
	return err
}

func (m *MongoManager) IsDuplicateContent(ctx context.Context, contentHash string) (bool, error) {
	posts := m.db.Collection("posts")
	count, err := posts.CountDocuments(ctx, bson.M{"content_hash": contentHash})
	return count > 0, err
}

// ============ ОПЕРАЦИИ СО СТАТИСТИКОЙ ============

func (m *MongoManager) IncrementViewCount(ctx context.Context, postID int) error {
	posts := m.db.Collection("posts")
	_, err := posts.UpdateOne(ctx,
		bson.M{"post_id": postID},
		bson.M{"$inc": bson.M{"stats.views": 1}},
	)
	return err
}

func (m *MongoManager) AddTagToPost(ctx context.Context, postID int, tag string) error {
	posts := m.db.Collection("posts")
	_, err := posts.UpdateOne(ctx,
		bson.M{"post_id": postID},
		bson.M{"$addToSet": bson.M{"tags": tag}},
	)
	return err
}

func (m *MongoManager) RemoveTagFromPost(ctx context.Context, postID int, tag string) error {
	posts := m.db.Collection("posts")
	_, err := posts.UpdateOne(ctx,
		bson.M{"post_id": postID},
		bson.M{"$pull": bson.M{"tags": tag}},
	)
	return err
}

func (m *MongoManager) UpdatePostStats(ctx context.Context, postID, likesDelta, commentsDelta int) error {
	posts := m.db.Collection("posts")
	_, err := posts.UpdateOne(ctx,
		bson.M{"post_id": postID},
		bson.M{"$inc": bson.M{
			"stats.likes":    likesDelta,
			"stats.comments": commentsDelta,
		}},
	)
	return err
}

func (m *MongoManager) UpsertPost(ctx context.Context, postID int, data map[string]interface{}) (bool, error) {
	posts := m.db.Collection("posts")

	update := bson.M{
		"$set": bson.M{
			"post_id":    postID,
			"title":      data["title"],
			"content":    data["content"],
			"updated_at": time.Now(),
		},
		"$setOnInsert": bson.M{
			"created_at": time.Now(),
			"stats": bson.M{
				"views":    0,
				"likes":    0,
				"comments": 0,
			},
		},
	}

	opts := options.Update().SetUpsert(true)
	result, err := posts.UpdateOne(ctx, bson.M{"post_id": postID}, update, opts)
	return result.UpsertedCount > 0, err
}

// ============ ПОИСК ============

func (m *MongoManager) AdvancedSearch(ctx context.Context, filters map[string]interface{}, limit int) ([]map[string]interface{}, error) {
	posts := m.db.Collection("posts")
	filter := bson.M{}

	// Более эффективная обработка фильтров
	if tags, ok := filters["tags"].([]interface{}); ok && len(tags) > 0 {
		filter["tags"] = bson.M{"$all": tags}
	}

	if minLikes, ok := filters["min_likes"].(float64); ok && minLikes > 0 {
		filter["stats.likes"] = bson.M{"$gte": int(minLikes)}
	}

	if excludeTags, ok := filters["exclude_tags"].([]interface{}); ok && len(excludeTags) > 0 {
		filter["tags"] = bson.M{"$nin": excludeTags}
	}

	// Оптимизация: используем более эффективный набор полей
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "stats.likes", Value: -1}}).
		SetBatchSize(100). // Увеличиваем batch size
		SetProjection(bson.M{
			"post_id": 1,
			"title":   1,
			"tags":    1,
			"stats":   1,
			"_id":     0,
		})

	cursor, err := posts.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Оптимизация: используем более эффективное чтение
	var results []map[string]interface{}
	for cursor.Next(ctx) {
		var result map[string]interface{}
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, cursor.Err()
}

// ============ АГРЕГАЦИИ ============

func (m *MongoManager) GetTopTags(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	posts := m.db.Collection("posts")

	// Only use allowDiskUse for large limits (to avoid the 2938ms bug)
	var opts *options.AggregateOptions
	if limit > 20 {
		opts = options.Aggregate().SetAllowDiskUse(true)
	}

	pipeline := mongo.Pipeline{
		{{Key: "$unwind", Value: "$tags"}},
		{{Key: "$group", Value: bson.M{
			"_id":         "$tags",
			"count":       bson.M{"$sum": 1},
			"total_likes": bson.M{"$sum": "$stats.likes"},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
		{{Key: "$limit", Value: limit}},
		{{Key: "$project", Value: bson.M{
			"tag":         "$_id",
			"count":       1,
			"total_likes": 1,
			"_id":         0,
		}}},
	}

	var cursor *mongo.Cursor
	var err error

	if opts != nil {
		cursor, err = posts.Aggregate(ctx, pipeline, opts)
	} else {
		cursor, err = posts.Aggregate(ctx, pipeline)
	}

	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (m *MongoManager) GetPostEngagementAnalysis(ctx context.Context, days int) (map[string]interface{}, error) {
	posts := m.db.Collection("posts")

	cutoffDate := time.Now().AddDate(0, 0, -days)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"created_at": bson.M{"$gte": cutoffDate},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"engagement_rate": bson.M{
				"$divide": bson.A{
					bson.M{"$add": bson.A{"$stats.likes", "$stats.comments"}},
					bson.M{"$max": bson.A{"$stats.views", 1}},
				},
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":            nil,
			"avg_engagement": bson.M{"$avg": "$engagement_rate"},
			"max_engagement": bson.M{"$max": "$engagement_rate"},
			"posts_analyzed": bson.M{"$sum": 1},
		}}},
	}

	cursor, err := posts.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) > 0 {
		return results[0], nil
	}
	return map[string]interface{}{}, nil
}

func (m *MongoManager) GetChannelPerformance(ctx context.Context) ([]map[string]interface{}, error) {
	posts := m.db.Collection("posts")

	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":                "$channel_id",
			"post_count":         bson.M{"$sum": 1},
			"total_likes":        bson.M{"$sum": "$stats.likes"},
			"total_views":        bson.M{"$sum": "$stats.views"},
			"avg_likes_per_post": bson.M{"$avg": "$stats.likes"},
		}}},
		{{Key: "$sort", Value: bson.M{"total_likes": -1}}},
		{{Key: "$limit", Value: 10}},
	}

	cursor, err := posts.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (m *MongoManager) RecordUserInteraction(ctx context.Context, userID string, postID int, action string) error {
	interactions := m.db.Collection("user_interactions")

	doc := bson.M{
		"user_id":   userID,
		"post_id":   postID,
		"action":    action,
		"timestamp": time.Now(),
	}

	_, err := interactions.InsertOne(ctx, doc)
	return err
}

func (m *MongoManager) GetUserHistory(ctx context.Context, userID string, limit int) ([]map[string]interface{}, error) {
	interactions := m.db.Collection("user_interactions")
	posts := m.db.Collection("posts")

	// АДАПТИВНЫЙ ПОДХОД: проверяем количество записей
	count, err := interactions.CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}

	// Если мало записей (< 50) - используем простой pipeline
	if count < 50 {
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"user_id": userID}}},
			{{Key: "$lookup", Value: bson.M{
				"from":         "posts",
				"localField":   "post_id",
				"foreignField": "post_id",
				"as":           "post_details",
			}}},
			{{Key: "$unwind", Value: "$post_details"}},
			{{Key: "$sort", Value: bson.M{"timestamp": -1}}},
			{{Key: "$limit", Value: limit}},
			{{Key: "$project", Value: bson.M{
				"action":     1,
				"timestamp":  1,
				"post_id":    1,
				"post_title": "$post_details.title",
				"_id":        0,
			}}},
		}

		cursor, err := interactions.Aggregate(ctx, pipeline)
		if err != nil {
			return nil, err
		}
		defer cursor.Close(ctx)

		var results []map[string]interface{}
		if err := cursor.All(ctx, &results); err != nil {
			return nil, err
		}
		return results, nil
	}

	// Для больших объемов - используем оптимизированный подход
	// Оптимизация 1: Сначала получаем только ID постов
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user_id": userID}}},
		{{Key: "$sort", Value: bson.M{"timestamp": -1}}},
		{{Key: "$limit", Value: limit}},
		{{Key: "$project", Value: bson.M{
			"action":    1,
			"timestamp": 1,
			"post_id":   1,
			"_id":       0,
		}}},
	}

	cursor, err := interactions.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var interactionResults []map[string]interface{}
	if err := cursor.All(ctx, &interactionResults); err != nil {
		return nil, err
	}

	// Оптимизация 2: Получаем данные постов одним запросом
	if len(interactionResults) == 0 {
		return []map[string]interface{}{}, nil
	}

	postIDs := make([]int, 0, len(interactionResults))
	for _, inter := range interactionResults {
		if id, ok := inter["post_id"].(int32); ok {
			postIDs = append(postIDs, int(id))
		}
	}

	// Получаем посты одним запросом
	postFilter := bson.M{"post_id": bson.M{"$in": postIDs}}
	postCursor, err := posts.Find(ctx, postFilter, options.Find().
		SetProjection(bson.M{
			"post_id": 1,
			"title":   1,
		}))
	if err != nil {
		return nil, err
	}
	defer postCursor.Close(ctx)

	var postResults []map[string]interface{}
	if err := postCursor.All(ctx, &postResults); err != nil {
		return nil, err
	}

	// Создаем map для быстрого поиска постов
	postMap := make(map[int]string)
	for _, post := range postResults {
		if id, ok := post["post_id"].(int32); ok {
			if title, ok := post["title"].(string); ok {
				postMap[int(id)] = title
			}
		}
	}

	// Объединяем результаты
	results := make([]map[string]interface{}, 0, len(interactionResults))
	for _, inter := range interactionResults {
		if postID, ok := inter["post_id"].(int32); ok {
			result := make(map[string]interface{})
			result["action"] = inter["action"]
			result["timestamp"] = inter["timestamp"]
			result["post_id"] = postID
			result["post_title"] = postMap[int(postID)]
			results = append(results, result)
		}
	}

	return results, nil
}

// ============ МАТЕРИАЛИЗОВАННЫЕ ПРЕДСТАВЛЕНИЯ ============

func (m *MongoManager) MaterializeTopPostsView(ctx context.Context) error {
	posts := m.db.Collection("posts")
	topPostsView := m.db.Collection("top_posts_view")

	cutoffDate := time.Now().AddDate(0, 0, -7)

	// Оптимизация: используем более эффективный pipeline
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"created_at":  bson.M{"$gte": cutoffDate},
			"stats.views": bson.M{"$gt": 0}, // Только посты с просмотрами
		}}},
		{{Key: "$addFields", Value: bson.M{
			"total_score": bson.M{
				"$add": bson.A{
					bson.M{"$multiply": bson.A{"$stats.likes", 3}},
					bson.M{"$multiply": bson.A{"$stats.comments", 2}},
					bson.M{"$multiply": bson.A{"$stats.views", 0.5}},
				},
			},
		}}},
		{{Key: "$sort", Value: bson.M{"total_score": -1}}},
		{{Key: "$limit", Value: 100}},
	}

	// Оптимизация: создаем временную коллекцию и затем переименовываем
	tempCollection := "top_posts_view_temp"

	pipelineWithOut := append(pipeline, bson.D{{Key: "$out", Value: tempCollection}})
	_, err := posts.Aggregate(ctx, pipelineWithOut)
	if err != nil {
		return err
	}

	// Атомарная замена коллекций
	if err := topPostsView.Drop(ctx); err != nil {
		log.Printf("Warning: failed to drop old view: %v", err)
	}

	// УБИРАЕМ НЕНУЖНУЮ ПЕРЕМЕННУЮ tempColl
	renameCmd := bson.D{
		{Key: "renameCollection", Value: m.db.Name() + "." + tempCollection},
		{Key: "to", Value: m.db.Name() + "." + "top_posts_view"},
	}

	return m.db.RunCommand(ctx, renameCmd).Err()
}

func (m *MongoManager) GetTopPostsFromView(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	topPostsView := m.db.Collection("top_posts_view")

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "total_score", Value: -1}})

	cursor, err := topPostsView.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (m *MongoManager) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}

// Вспомогательная функция для хэширования
func hashString(s string) uint64 {
	var hash uint64 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + uint64(c)
	}
	return hash
}
