package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"news-aggregator/internal/cache"
	"news-aggregator/internal/handlers"
	"news-aggregator/internal/mongo"
	"news-aggregator/internal/pgpool"
)

func main() {
	log.Println("Starting News Aggregator Server...")

	// Инициализация PostgreSQL pool
	connStrings := []string{
		"host=db-master port=5432 dbname=news_db user=news_user password=news_pass sslmode=disable",
		"host=db-replica port=5432 dbname=news_db user=news_user password=news_pass sslmode=disable",
	}
	pool, err := pgpool.NewPgPool(connStrings, 4)
	if err != nil {
		log.Fatalf("Failed to initialize PgPool: %v", err)
	}
	defer pool.Close()

	// Инициализация Redis cache
	cacheManager := cache.NewCacheManager("redis:6379", "", 0)
	defer cacheManager.Close()

	// Инициализация MongoDB
	mongoURI := getEnv("MONGODB_URI", "mongodb://news_app:app_password@mongodb:27017/news_aggregator?authSource=news_aggregator")
	mongoManager, err := mongo.NewMongoManager(mongoURI)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}
	defer mongoManager.Close()

	// Запуск health check в фоне
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := pool.HealthCheck(); err != nil {
				log.Printf("Health check error: %v", err)
			}
		}
	}()

	// Настройка маршрутов
	handler := handlers.NewHandlers(pool, cacheManager, mongoManager)
	router := handler.SetupRoutes()

	// HTTP сервер
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Println("Server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
