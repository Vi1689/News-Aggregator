package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"news-aggregator/internal/models"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Neo4jClient struct {
	Driver   neo4j.DriverWithContext
	Database string
}

func NewNeo4jClient(uri, username, password, database string) (*Neo4jClient, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create driver: %w", err)
	}

	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("failed to verify connectivity: %w", err)
	}

	log.Println("Connected to Neo4j successfully")

	return &Neo4jClient{
		Driver:   driver,
		Database: database,
	}, nil
}

func (c *Neo4jClient) Close() {
	c.Driver.Close(context.Background())
}

// Создание ограничений и индексов
func (c *Neo4jClient) SetupConstraints(ctx context.Context) error {
	queries := []string{
		"CREATE CONSTRAINT news_id_unique IF NOT EXISTS FOR (n:News) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT category_name_unique IF NOT EXISTS FOR (c:Category) REQUIRE c.name IS UNIQUE",
		"CREATE CONSTRAINT author_name_unique IF NOT EXISTS FOR (a:Author) REQUIRE a.name IS UNIQUE",
		"CREATE CONSTRAINT tag_name_unique IF NOT EXISTS FOR (t:Tag) REQUIRE t.name IS UNIQUE",
		"CREATE CONSTRAINT user_id_unique IF NOT EXISTS FOR (u:User) REQUIRE u.id IS UNIQUE",

		"CREATE INDEX news_published_at_idx IF NOT EXISTS FOR (n:News) ON (n.published_at)",
		"CREATE INDEX news_language_idx IF NOT EXISTS FOR (n:News) ON (n.language)",
		"CREATE INDEX author_type_idx IF NOT EXISTS FOR (a:Author) ON (a.type)",
	}

	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	for _, query := range queries {
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			log.Printf("Warning: constraint/index creation error: %v", err)
		}
	}

	log.Println("Constraints and indexes created")
	return nil
}

// Очистка всей базы данных
func (c *Neo4jClient) CleanDatabase(ctx context.Context) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	_, err := session.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		return fmt.Errorf("failed to clean database: %w", err)
	}

	log.Println("Database cleaned")
	return nil
}

// Создание узла новости
func (c *Neo4jClient) CreateNews(ctx context.Context, news *models.News) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MERGE (n:News {id: $id})
        SET n.title = $title,
            n.summary = $summary,
            n.published_at = $published_at,
            n.source_url = $source_url,
            n.language = $language
    `

	_, err := session.Run(ctx, query, map[string]interface{}{
		"id":           news.ID,
		"title":        news.Title,
		"summary":      news.Summary,
		"published_at": news.PublishedAt,
		"source_url":   news.SourceURL,
		"language":     news.Language,
	})

	return err
}

// Создание связи BELONGS_TO
func (c *Neo4jClient) CreateBelongsToRelation(ctx context.Context, newsID, categoryName string) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (n:News {id: $news_id})
        MERGE (c:Category {name: $category_name})
        MERGE (n)-[:BELONGS_TO]->(c)
    `

	_, err := session.Run(ctx, query, map[string]interface{}{
		"news_id":       newsID,
		"category_name": categoryName,
	})

	return err
}

// Создание связи WRITTEN_BY
func (c *Neo4jClient) CreateWrittenByRelation(ctx context.Context, newsID, authorName, authorType string) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (n:News {id: $news_id})
        MERGE (a:Author {name: $author_name})
        SET a.type = $author_type
        MERGE (n)-[:WRITTEN_BY]->(a)
    `

	_, err := session.Run(ctx, query, map[string]interface{}{
		"news_id":     newsID,
		"author_name": authorName,
		"author_type": authorType,
	})

	return err
}

// Создание связи TAGGED_WITH
func (c *Neo4jClient) CreateTaggedWithRelation(ctx context.Context, newsID, tagName string, weight float64) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (n:News {id: $news_id})
        MERGE (t:Tag {name: $tag_name})
        MERGE (n)-[:TAGGED_WITH {weight: $weight}]->(t)
    `

	_, err := session.Run(ctx, query, map[string]interface{}{
		"news_id":  newsID,
		"tag_name": tagName,
		"weight":   weight,
	})

	return err
}

// Создание связи LIKED
func (c *Neo4jClient) CreateLikedRelation(ctx context.Context, userID, newsID string, timestamp time.Time) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (u:User {id: $user_id})
        MATCH (n:News {id: $news_id})
        MERGE (u)-[:LIKED {timestamp: $timestamp}]->(n)
    `

	_, err := session.Run(ctx, query, map[string]interface{}{
		"user_id":   userID,
		"news_id":   newsID,
		"timestamp": timestamp,
	})

	return err
}

// Создание связи VIEWED
func (c *Neo4jClient) CreateViewedRelation(ctx context.Context, userID, newsID string, durationSec int, timestamp time.Time) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (u:User {id: $user_id})
        MATCH (n:News {id: $news_id})
        MERGE (u)-[:VIEWED {timestamp: $timestamp, duration_sec: $duration}]->(n)
    `

	_, err := session.Run(ctx, query, map[string]interface{}{
		"user_id":   userID,
		"news_id":   newsID,
		"duration":  durationSec,
		"timestamp": timestamp,
	})

	return err
}

// Создание связи MENTIONS (цитирование)
func (c *Neo4jClient) CreateMentionsRelation(ctx context.Context, fromNewsID, toNewsID string, strength int, context string) error {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (from:News {id: $from_id})
        MATCH (to:News {id: $to_id})
        MERGE (from)-[:MENTIONS {strength: $strength, context: $context}]->(to)
    `

	_, err := session.Run(ctx, query, map[string]interface{}{
		"from_id":  fromNewsID,
		"to_id":    toNewsID,
		"strength": strength,
		"context":  context,
	})

	return err
}

// ============================================
// ГРАФОВЫЕ ЗАПРОСЫ ДЛЯ ЗАДАНИЙ
// ============================================

// Запрос 1: Похожие новости через общие теги
func (c *Neo4jClient) GetSimilarNews(ctx context.Context, newsID string, limit int, minWeight float64) ([]map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (n:News {id: $news_id})-[:TAGGED_WITH {weight: $min_weight}]->(t:Tag)<-[:TAGGED_WITH]-(similar:News)
        WHERE n.id <> similar.id
        RETURN similar.id AS news_id,
               similar.title AS title,
               collect(DISTINCT t.name) AS common_tags,
               count(DISTINCT t) AS common_tags_count
        ORDER BY common_tags_count DESC
        LIMIT $limit
    `

	result, err := session.Run(ctx, query, map[string]interface{}{
		"news_id":    newsID,
		"min_weight": minWeight,
		"limit":      limit,
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		results = append(results, map[string]interface{}{
			"news_id":           record.Values[0],
			"title":             record.Values[1],
			"common_tags":       record.Values[2],
			"common_tags_count": record.Values[3],
		})
	}

	return results, nil
}

// Запрос 2: Цепочка цитирований (переменная длина пути)
func (c *Neo4jClient) GetCitationChain(ctx context.Context, startNewsID string, minHops, maxHops int) ([]map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	// Исправление: подставляем значения через fmt.Sprintf
	query := fmt.Sprintf(`
		MATCH path = (start:News {id: $start_id})-[:MENTIONS*%d..%d]->(end:News)
		RETURN end.id AS end_news_id,
			   end.title AS end_title,
			   length(path) AS chain_length,
			   [rel IN relationships(path) | rel.strength] AS strengths,
			   [node IN nodes(path) | node.id] AS path_nodes
		ORDER BY chain_length ASC
		LIMIT 10
	`, minHops, maxHops)

	result, err := session.Run(ctx, query, map[string]interface{}{
		"start_id": startNewsID,
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		results = append(results, map[string]interface{}{
			"end_news_id":  record.Values[0],
			"end_title":    record.Values[1],
			"chain_length": record.Values[2],
			"strengths":    record.Values[3],
			"path_nodes":   record.Values[4],
		})
	}

	return results, nil
}

// Запрос 3: Общие соседи (авторы, которые пишут на одни темы)
func (c *Neo4jClient) GetCommonNeighbors(ctx context.Context, author1, author2 string) ([]map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (a1:Author {name: $author1})-[:WRITTEN_BY]->(n:News)-[:TAGGED_WITH]->(t:Tag)<-[:TAGGED_WITH]-(:News)<-[:WRITTEN_BY]-(a2:Author {name: $author2})
        RETURN t.name AS common_tag,
               count(DISTINCT n) AS shared_news_count
        ORDER BY shared_news_count DESC
        LIMIT 20
    `

	result, err := session.Run(ctx, query, map[string]interface{}{
		"author1": author1,
		"author2": author2,
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		results = append(results, map[string]interface{}{
			"common_tag":        record.Values[0],
			"shared_news_count": record.Values[1],
		})
	}

	return results, nil
}

// Запрос 4: Агрегация - топ авторов по количеству новостей
func (c *Neo4jClient) GetTopAuthors(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (a:Author)<-[:WRITTEN_BY]-(n:News)
        RETURN a.name AS author,
               a.type AS author_type,
               count(n) AS news_count,
               collect(n.title)[0..5] AS sample_titles
        ORDER BY news_count DESC
        LIMIT $limit
    `

	result, err := session.Run(ctx, query, map[string]interface{}{
		"limit": limit,
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		results = append(results, map[string]interface{}{
			"author":        record.Values[0],
			"author_type":   record.Values[1],
			"news_count":    record.Values[2],
			"sample_titles": record.Values[3],
		})
	}

	return results, nil
}

// Запрос 5: Поиск по фильтрам (одношаговый)
func (c *Neo4jClient) FilterNews(ctx context.Context, language string, category string, fromDate time.Time) ([]map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (n:News)-[:BELONGS_TO]->(c:Category)
        WHERE n.language = $language
          AND c.name = $category
          AND n.published_at >= $from_date
        RETURN n.id AS news_id,
               n.title AS title,
               n.published_at AS published_at,
               c.name AS category
        ORDER BY n.published_at DESC
        LIMIT 50
    `

	result, err := session.Run(ctx, query, map[string]interface{}{
		"language":  language,
		"category":  category,
		"from_date": fromDate,
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		results = append(results, map[string]interface{}{
			"news_id":      record.Values[0],
			"title":        record.Values[1],
			"published_at": record.Values[2],
			"category":     record.Values[3],
		})
	}

	return results, nil
}

// Запрос 6: Комбинированный запрос (цепочка + агрегация + фильтр)
func (c *Neo4jClient) GetComplexRecommendations(ctx context.Context, userID string, minHops, maxHops int, limit int) ([]map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (u:User {id: $user_id})-[:LIKED]->(liked:News)
		MATCH path = (liked)-[:TAGGED_WITH*2..4]->(t:Tag)<-[:TAGGED_WITH]-(rec:News)
		WHERE rec.id <> liked.id
		  AND rec.published_at >= datetime() - duration({days: 7})
		WITH rec, 
			 count(DISTINCT liked) AS liked_sources_count,
			 collect(DISTINCT t.name) AS path_tags,
			 reduce(s = 0.0, rel IN relationships(path) | s + COALESCE(rel.weight, 0)) / length(path) AS avg_weight
		RETURN rec.id AS news_id,
			   rec.title AS title,
			   liked_sources_count,
			   path_tags,
			   avg_weight
		ORDER BY liked_sources_count DESC, avg_weight DESC
		LIMIT $limit
	`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"user_id": userID,
		"limit":   limit,
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		results = append(results, map[string]interface{}{
			"news_id":             record.Values[0],
			"title":               record.Values[1],
			"liked_sources_count": record.Values[2],
			"path_tags":           record.Values[3],
			"avg_weight":          record.Values[4],
		})
	}

	return results, nil
}

// Запрос 7: Статистика по графу (общее количество)
func (c *Neo4jClient) GetGraphStats(ctx context.Context) (map[string]interface{}, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.Database,
	})
	defer session.Close(ctx)

	query := `
        MATCH (n:News) WITH count(n) AS news_count
        OPTIONAL MATCH (c:Category) WITH news_count, count(c) AS category_count
        OPTIONAL MATCH (a:Author) WITH news_count, category_count, count(a) AS author_count
        OPTIONAL MATCH (t:Tag) WITH news_count, category_count, author_count, count(t) AS tag_count
        OPTIONAL MATCH (u:User) WITH news_count, category_count, author_count, tag_count, count(u) AS user_count
        OPTIONAL MATCH ()-[r]->() WITH news_count, category_count, author_count, tag_count, user_count, count(r) AS rel_count
        RETURN news_count, category_count, author_count, tag_count, user_count, rel_count
    `

	result, err := session.Run(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	if result.Next(ctx) {
		record := result.Record()
		return map[string]interface{}{
			"news_count":      record.Values[0],
			"category_count":  record.Values[1],
			"author_count":    record.Values[2],
			"tag_count":       record.Values[3],
			"user_count":      record.Values[4],
			"relations_count": record.Values[5],
		}, nil
	}

	return map[string]interface{}{
		"news_count": 0, "category_count": 0, "author_count": 0,
		"tag_count": 0, "user_count": 0, "relations_count": 0,
	}, nil
}
