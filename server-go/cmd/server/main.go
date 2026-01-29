// [file name]: main.go
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

	// Инициализация MongoDB с репликацией
	mongoURI := getEnv("MONGODB_URI", "mongodb://news_app:app_password@mongodb-primary:27017,mongodb-secondary1:27017,mongodb-secondary2:27017/news_aggregator?authSource=news_aggregator&replicaSet=rs0&readPreference=secondaryPreferred&w=majority")
	mongoManager, err := mongo.NewMongoManager(mongoURI)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}
	defer mongoManager.Close()

	// Запуск health check для PostgreSQL в фоне
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := pool.HealthCheck(); err != nil {
				log.Printf("PostgreSQL health check error: %v", err)
			}
		}
	}()

	// Запуск health check для MongoDB репликации
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			health, err := mongoManager.CheckReplicaSetHealth(ctx)
			cancel()
			
			if err != nil {
				log.Printf("MongoDB replica set health check failed: %v", err)
			} else {
				if ok, okBool := health["ok"].(float64); okBool && ok == 1 {
					log.Printf("✓ MongoDB replica set healthy. Members: %v", health["total_members"])
				} else {
					log.Printf("⚠ MongoDB replica set unhealthy: %v", health)
				}
			}
		}
	}()

	// Периодическое обновление материализованных представлений
	go func() {
		// Ждем запуска сервера
		time.Sleep(2 * time.Minute)
		
		ticker := time.NewTicker(30 * time.Minute) // Обновляем каждые 30 минут
		defer ticker.Stop()
		
		for range ticker.C {
			log.Println("Refreshing materialized views...")
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			err := mongoManager.MaterializeTopPostsView(ctx)
			cancel()
			
			if err != nil {
				log.Printf("Failed to refresh materialized views: %v", err)
			} else {
				log.Println("Materialized views refreshed successfully")
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
	serverErrors := make(chan error, 1)
	go func() {
		log.Println("Server starting on :8080")
		log.Printf("MongoDB URI: %s", getMongoURIMasked(mongoURI))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Server failed: %v", err)
	case sig := <-quit:
		log.Printf("Received signal %v. Shutting down server...", sig)
		
		// Даем время для завершения текущих операций
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		// Останавливаем сервер
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
		
		// Дополнительное время для очистки ресурсов
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		
		// Закрываем соединения
		mongoManager.Close()
		cacheManager.Close()
		pool.Close()
		
		select {
		case <-cleanupCtx.Done():
			log.Println("Cleanup timeout exceeded")
		default:
			log.Println("Cleanup completed")
		}
	}

	log.Println("Server exited gracefully")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Helper функция для маскирования пароля в логах
func getMongoURIMasked(uri string) string {
	// Маскируем пароль в URI для безопасности в логах
	const userPassMarker = "news_app:"
	const atMarker = "@"
	
	startIdx := 0
	for {
		userIdx := startIdx
		if userIdx >= len(uri) {
			break
		}
		
		userIdx = indexAt(uri, userPassMarker, userIdx)
		if userIdx == -1 {
			break
		}
		
		atIdx := indexAt(uri, atMarker, userIdx+len(userPassMarker))
		if atIdx == -1 {
			break
		}
		
		// Маскируем пароль
		maskedPassword := "****"
		uri = uri[:userIdx+len(userPassMarker)] + maskedPassword + uri[atIdx:]
		startIdx = userIdx + len(userPassMarker) + len(maskedPassword) + len(atMarker)
	}
	
	return uri
}

func indexAt(s, substr string, start int) int {
	if start < 0 || start >= len(s) {
		return -1
	}
	
	idx := -1
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			idx = i
			break
		}
	}
	return idx
}