package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"news-aggregator/internal/config"
	"news-aggregator/internal/database"
	"news-aggregator/internal/handlers"
	"news-aggregator/internal/kafka"

	"github.com/gin-gonic/gin"
)

func main() {
	// Загрузка конфигурации
	cfg := config.Load()

	log.Println("Starting News Aggregator Service...")
	log.Printf("Config: Neo4j=%s, Kafka=%v, ClickHouse=%s:%s",
		cfg.Neo4jURI, cfg.KafkaBrokers, cfg.ClickHouseHost, cfg.ClickHousePort)

	// Инициализация Neo4j
	neo4jClient, err := database.NewNeo4jClient(
		cfg.Neo4jURI,
		cfg.Neo4jUser,
		cfg.Neo4jPassword,
		cfg.Neo4jDatabase,
	)
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer neo4jClient.Close()

	// Создание ограничений и индексов
	ctx := context.Background()
	if err := neo4jClient.SetupConstraints(ctx); err != nil {
		log.Printf("Warning: failed to setup constraints: %v", err)
	}

	// Инициализация ClickHouse
	clickhouseClient, err := database.NewClickHouseClient(
		cfg.ClickHouseHost,
		cfg.ClickHousePort,
		cfg.ClickHouseUser,
		cfg.ClickHousePassword,
		cfg.ClickHouseDB,
	)
	if err != nil {
		log.Printf("Warning: failed to connect to ClickHouse: %v", err)
	}
	if clickhouseClient != nil {
		defer clickhouseClient.Close()
	}

	// Инициализация Kafka Producer
	var kafkaProducer *kafka.Producer
	if cfg.EnableKafkaProducer {
		kafkaProducer = kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaTopicEvents)
		defer kafkaProducer.Close()
	}

	// Инициализация Kafka Consumer для Neo4j
	if cfg.EnableKafkaConsumerNeo4j {
		dlqProducer := kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaTopicDLQ)
		neo4jConsumer := kafka.NewNeo4jConsumer(
			cfg.KafkaBrokers,
			cfg.KafkaTopicEvents,
			cfg.KafkaConsumerGroupNeo4j,
			neo4jClient,
			dlqProducer,
		)
		defer neo4jConsumer.Close()

		// Запуск consumer в горутине
		go func() {
			neo4jConsumer.Start(context.Background())
		}()
	}

	// Инициализация Kafka Consumer для ClickHouse
	if cfg.EnableKafkaConsumerClickHouse && clickhouseClient != nil {
		dlqProducerForCH := kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaTopicDLQ)
		clickhouseConsumer := kafka.NewClickHouseConsumer(
			cfg.KafkaBrokers,
			cfg.KafkaTopicEvents,
			cfg.KafkaConsumerGroupClickHouse,
			clickhouseClient,
			dlqProducerForCH,
		)
		defer clickhouseConsumer.Close()

		go func() {
			clickhouseConsumer.Start(context.Background())
		}()
	}

	// Настройка HTTP сервера
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Инициализация хендлеров
	h := handlers.NewHandler(cfg, neo4jClient, clickhouseClient, kafkaProducer)

	// Health check
	router.GET("/health", h.HealthCheck)

	// API для генерации данных
	router.POST("/api/generate", h.GenerateData)

	// Neo4j API (Задание 1)
	neo4jAPI := router.Group("/api/neo4j")
	{
		neo4jAPI.GET("/similar/:id", h.GetSimilarNews)
		neo4jAPI.GET("/chain/:id", h.GetCitationChain)
		neo4jAPI.GET("/common-neighbors", h.GetCommonNeighbors)
		neo4jAPI.GET("/top-authors", h.GetTopAuthors)
		neo4jAPI.GET("/filter", h.FilterNews)
		neo4jAPI.GET("/recommend/:user_id", h.GetComplexRecommendations)
		neo4jAPI.GET("/stats", h.GetGraphStats)
	}

	// ClickHouse API (Задание 3)
	if clickhouseClient != nil {
		chAPI := router.Group("/api/clickhouse")
		{
			chAPI.GET("/top-news", h.GetTopNews)
			chAPI.GET("/popular-tags", h.GetPopularTags)
			chAPI.GET("/user-activity", h.GetUserActivity)
			chAPI.GET("/author-stats", h.GetAuthorStats)
			chAPI.GET("/retention", h.GetUserRetention)
			chAPI.GET("/engagement", h.GetEngagementByHour)
		}
	}

	// Запуск сервера
	srv := &gin.Engine{}
	go func() {
		log.Printf("HTTP server listening on :%s", cfg.HTTPPort)
		if err := router.Run(":" + cfg.HTTPPort); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	<-ctx.Done()
	log.Println("Server exited")
}
