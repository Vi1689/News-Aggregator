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

	// Обработка тегов
	if tags, ok := filters["tags"].([]interface{}); ok {
		filter["tags"] = bson.M{"$in": tags}
	}

	// Обработка минимальных лайков
	if minLikes, ok := filters["min_likes"].(float64); ok {
		filter["stats.likes"] = bson.M{"$gte": int(minLikes)}
	}

	// Исключение тегов
	if excludeTags, ok := filters["exclude_tags"].([]interface{}); ok {
		filter["tags"] = bson.M{"$nin": excludeTags}
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "stats.likes", Value: -1}}).
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

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// ============ АГРЕГАЦИИ ============

func (m *MongoManager) GetTopTags(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	posts := m.db.Collection("posts")

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

// ============ ПОЛЬЗОВАТЕЛЬСКИЕ ВЗАИМОДЕЙСТВИЯ ============

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

// ============ МАТЕРИАЛИЗОВАННЫЕ ПРЕДСТАВЛЕНИЯ ============

func (m *MongoManager) MaterializeTopPostsView(ctx context.Context) error {
	posts := m.db.Collection("posts")
	topPostsView := m.db.Collection("top_posts_view")

	// Очищаем старую витрину
	if err := topPostsView.Drop(ctx); err != nil {
		log.Printf("Warning: failed to drop top_posts_view: %v", err)
	}

	cutoffDate := time.Now().AddDate(0, 0, -7)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"created_at": bson.M{"$gte": cutoffDate},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"total_score": bson.M{
				"$add": bson.A{
					bson.M{"$multiply": bson.A{"$stats.likes", 3}},
					bson.M{"$multiply": bson.A{"$stats.comments", 2}},
					"$stats.views",
				},
			},
		}}},
		{{Key: "$sort", Value: bson.M{"total_score": -1}}},
		{{Key: "$limit", Value: 100}},
		{{Key: "$out", Value: "top_posts_view"}},
	}

	_, err := posts.Aggregate(ctx, pipeline)
	return err
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
