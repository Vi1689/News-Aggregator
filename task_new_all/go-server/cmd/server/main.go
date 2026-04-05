package main

import (
	"context"
	"log"
	"net/http"
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

	// Создаем контекст с отменой для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	if err := neo4jClient.SetupConstraints(ctx); err != nil {
		log.Printf("Warning: failed to setup constraints: %v", err)
	}

	// Инициализация ClickHouse (опционально)
	var clickhouseClient *database.ClickHouseClient
	if cfg.EnableKafkaConsumerClickHouse {
		clickhouseClient, err = database.NewClickHouseClient(
			cfg.ClickHouseHost,
			cfg.ClickHousePort,
			cfg.ClickHouseUser,
			cfg.ClickHousePassword,
			cfg.ClickHouseDB,
		)
		if err != nil {
			log.Printf("Warning: failed to connect to ClickHouse: %v", err)
			log.Println("ClickHouse features will be disabled")
			clickhouseClient = nil
		} else {
			defer clickhouseClient.Close()
		}
	}

	// Инициализация Kafka Producer
	var kafkaProducer *kafka.Producer
	if cfg.EnableKafkaProducer {
		kafkaProducer = kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaTopicEvents)
		defer kafkaProducer.Close()
	}

	// Канал для ошибок consumer'ов
	consumerErrors := make(chan error, 2)

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

		// Запуск consumer в горутине с обработкой ошибок
		go func() {
			log.Println("Starting Neo4j Kafka consumer...")
			neo4jConsumer.Start(ctx)
			consumerErrors <- nil
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
			log.Println("Starting ClickHouse Kafka consumer...")
			clickhouseConsumer.Start(ctx)
			consumerErrors <- nil
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

	// ClickHouse API (Задание 3) - только если клиент инициализирован
	if clickhouseClient != nil {
		chAPI := router.Group("/api/clickhouse")
		{
			chAPI.GET("/top-news", h.GetTopNews)
			chAPI.GET("/popular-tags", h.GetPopularTags)
			chAPI.GET("/user-activity", h.GetUserActivity)
			chAPI.GET("/author-stats", h.GetAuthorStats)
			chAPI.GET("/retention", h.GetUserRetention)
			chAPI.GET("/engagement", h.GetEngagementByHour)
			chAPI.GET("/likes-by-category", h.GetLikesByCategory)
			chAPI.GET("/top-readers", h.GetTopReaders)
		}
	}

	// HTTP сервер
	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	// Запуск HTTP сервера в горутине
	go func() {
		log.Printf("HTTP server listening on :%s", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Ожидание сигнала или ошибки consumer'а
	select {
	case <-quit:
		log.Println("Received shutdown signal...")
	case err := <-consumerErrors:
		if err != nil {
			log.Printf("Consumer error: %v", err)
		}
	}

	log.Println("Shutting down server...")

	// Отменяем контекст для остановки consumer'ов
	cancel()

	// Таймаут для shutdown HTTP сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Server exited")
}
