// connect/deploy.go
//
// Kafka Connect — Connector Deployment (Go)
// ==========================================
// Sends connector configurations to the Kafka Connect REST API.
// Run after the kafka-connect container is healthy.
//
// Usage:
//   go run ./connect/deploy.go

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const connectURL = "http://localhost:8083"

// ---------------------------------------------------------------------------
// Connector configs
// ---------------------------------------------------------------------------

var postgresSourceConfig = map[string]any{
	"name": "postgres-source-posts",
	"config": map[string]any{
		"connector.class": "io.confluent.connect.jdbc.JdbcSourceConnector",
		"connection.url":  "jdbc:postgresql://db-master:5432/news_db",
		"connection.user": "news_user",
		"connection.password": "news_pass",

		// Tracking mode: timestamp + incrementing — catches new AND updated rows
		"mode":                     "timestamp+incrementing",
		"incrementing.column.name": "post_id",
		"timestamp.column.name":    "created_at",

		// Join with authors so the event already has author_name
		"query": `
			SELECT p.post_id, p.title, a.name AS author_name,
			       p.channel_id, p.likes_count, p.comments_count, p.created_at
			FROM posts p LEFT JOIN authors a ON p.author_id = a.author_id`,

		"topic.prefix":  "kafka.pg.",
		"poll.interval.ms": "5000",
		"batch.max.rows":   "100",

		"key.converter":   "org.apache.kafka.connect.storage.StringConverter",
		"value.converter": "org.apache.kafka.connect.json.JsonConverter",
		"value.converter.schemas.enable": "false",

		// Transforms: stamp eventType so downstream consumers can filter easily
		"transforms": "addEventType,castTimestamp",
		"transforms.addEventType.type":          "org.apache.kafka.connect.transforms.InsertField$Value",
		"transforms.addEventType.static.field":  "eventType",
		"transforms.addEventType.static.value":  "POST_FROM_PG",
		"transforms.castTimestamp.type":          "org.apache.kafka.connect.transforms.TimestampConverter$Value",
		"transforms.castTimestamp.field":         "created_at",
		"transforms.castTimestamp.target.type":   "string",
		"transforms.castTimestamp.format":        "yyyy-MM-dd'T'HH:mm:ssZ",
	},
}

// Reads Kafka Streams hourly result and writes it back to PostgreSQL.
var postgresSinkConfig = map[string]any{
	"name": "postgres-sink-trending-tags",
	"config": map[string]any{
		"connector.class":     "io.confluent.connect.jdbc.JdbcSinkConnector",
		"connection.url":      "jdbc:postgresql://db-master:5432/news_db",
		"connection.user":     "news_user",
		"connection.password": "news_pass",

		"topics":             "news.tags.trending.hourly",
		"table.name.format":  "trending_tags_hourly",

		// UPSERT: idempotent write — safe to replay
		"insert.mode": "upsert",
		"pk.mode":     "record_value",
		"pk.fields":   "windowKey,tag",

		"auto.create": "true",
		"auto.evolve": "true",

		"key.converter":   "org.apache.kafka.connect.storage.StringConverter",
		"value.converter": "org.apache.kafka.connect.json.JsonConverter",
		"value.converter.schemas.enable": "false",

		"batch.size":          "50",
		"flush.synchronously": "true",
	},
}

// Persists normalised posts from Streams into MongoDB.
var mongoSinkConfig = map[string]any{
	"name": "mongodb-sink-posts-normalized",
	"config": map[string]any{
		"connector.class": "com.mongodb.kafka.connect.MongoSinkConnector",
		"connection.uri":  "mongodb://news_app:app_password@mongodb:27017",
		"database":        "news_aggregator",
		"collection":      "posts_normalized",

		"topics": "news.posts.normalized",

		"writemodel.strategy":    "com.mongodb.kafka.connect.sink.writemodel.strategy.ReplaceOneDefaultStrategy",
		"document.id.strategy":   "com.mongodb.kafka.connect.sink.processor.id.strategy.FullKey",

		"key.converter":   "org.apache.kafka.connect.storage.StringConverter",
		"value.converter": "org.apache.kafka.connect.json.JsonConverter",
		"value.converter.schemas.enable": "false",

		"bulk.write.ordered": "false",
		"max.batch.size":     "100",
	},
}

// ---------------------------------------------------------------------------
// Deployment helpers
// ---------------------------------------------------------------------------

func waitForConnect() {
	log.Println("[CONNECT] Waiting for Kafka Connect REST API...")
	for {
		resp, err := http.Get(connectURL + "/connectors")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Println("[CONNECT] Kafka Connect is ready.")
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func deployConnector(cfg map[string]any) error {
	name := cfg["name"].(string)

	// Delete existing connector (idempotent deploy)
	req, _ := http.NewRequest(http.MethodDelete, connectURL+"/connectors/"+name, nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	body, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}

	resp, err = http.Post(connectURL+"/connectors", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST %s: %w", name, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("connector %s: HTTP %d — %s", name, resp.StatusCode, respBody)
	}

	log.Printf("[CONNECT] Deployed: %s (HTTP %d)", name, resp.StatusCode)
	return nil
}

func listConnectors() {
	resp, err := http.Get(connectURL + "/connectors?expand=status")
	if err != nil {
		log.Printf("[CONNECT] list error: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("[CONNECT] Active connectors:\n%s", body)
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	waitForConnect()

	connectors := []map[string]any{
		postgresSourceConfig,
		postgresSinkConfig,
		mongoSinkConfig,
	}

	for _, cfg := range connectors {
		if err := deployConnector(cfg); err != nil {
			log.Printf("[CONNECT] ERROR: %v", err)
		}
	}

	log.Println("[CONNECT] All connectors deployed.")
	listConnectors()
}
