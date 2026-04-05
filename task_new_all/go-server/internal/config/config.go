package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Сервер
	HTTPPort string

	// Neo4j
	Neo4jURI      string
	Neo4jUser     string
	Neo4jPassword string
	Neo4jDatabase string

	// Kafka
	KafkaBrokers                 []string
	KafkaTopicEvents             string
	KafkaTopicAnalytics          string
	KafkaTopicDLQ                string
	KafkaConsumerGroupNeo4j      string
	KafkaConsumerGroupClickHouse string

	// ClickHouse
	ClickHouseHost     string
	ClickHousePort     string
	ClickHouseUser     string
	ClickHousePassword string
	ClickHouseDB       string

	// Генерация
	GenerationNodes     int
	GenerationRelations int
	GenerationEvents    int
	GenerationBatchSize int
	GenerationDelayMs   int

	// Флаги
	EnableKafkaProducer           bool
	EnableKafkaConsumerNeo4j      bool
	EnableKafkaConsumerClickHouse bool
}

func Load() *Config {
	// Загружаем .env если есть
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	return &Config{
		// Сервер
		HTTPPort: getEnv("HTTP_PORT", "8080"),

		// Neo4j
		Neo4jURI:      getEnv("NEO4J_URI", "bolt://localhost:7687"),
		Neo4jUser:     getEnv("NEO4J_USER", "neo4j"),
		Neo4jPassword: getEnv("NEO4J_PASSWORD", "StrongPwd123"),
		Neo4jDatabase: getEnv("NEO4J_DATABASE", "newsgraph"),

		// Kafka
		KafkaBrokers:                 []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		KafkaTopicEvents:             getEnv("KAFKA_TOPIC_EVENTS", "news.events"),
		KafkaTopicAnalytics:          getEnv("KAFKA_TOPIC_ANALYTICS", "news.analytics"),
		KafkaTopicDLQ:                getEnv("KAFKA_TOPIC_DLQ", "news.dlq"),
		KafkaConsumerGroupNeo4j:      getEnv("KAFKA_CONSUMER_GROUP_NEO4J", "neo4j-writer-group"),
		KafkaConsumerGroupClickHouse: getEnv("KAFKA_CONSUMER_GROUP_CLICKHOUSE", "clickhouse-writer-group"),

		// ClickHouse
		ClickHouseHost:     getEnv("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:     getEnv("CLICKHOUSE_PORT", "8123"),
		ClickHouseUser:     getEnv("CLICKHOUSE_USER", "analyst"),
		ClickHousePassword: getEnv("CLICKHOUSE_PASSWORD", "analyst123"),
		ClickHouseDB:       getEnv("CLICKHOUSE_DB", "newsdb"),

		// Генерация
		GenerationNodes:     getEnvInt("GENERATION_NODES", 50),
		GenerationRelations: getEnvInt("GENERATION_RELATIONS", 120),
		GenerationEvents:    getEnvInt("GENERATION_EVENTS", 100000),
		GenerationBatchSize: getEnvInt("GENERATION_BATCH_SIZE", 1000),
		GenerationDelayMs:   getEnvInt("GENERATION_DELAY_MS", 10),

		// Флаги
		EnableKafkaProducer:           getEnvBool("ENABLE_KAFKA_PRODUCER", true),
		EnableKafkaConsumerNeo4j:      getEnvBool("ENABLE_KAFKA_CONSUMER_NEO4J", true),
		EnableKafkaConsumerClickHouse: getEnvBool("ENABLE_KAFKA_CONSUMER_CLICKHOUSE", false),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
