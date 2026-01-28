package sendRequests

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RequestLogger struct {
	mu        sync.Mutex
	file      *os.File
	logPath   string
	debugFile *os.File
	sampled   map[string]int
}

var (
	logger *RequestLogger
	once   sync.Once
)

// InitLogger инициализирует логгер и отладочный файл
func InitLogger(logDir string) error {
	var initErr error
	once.Do(func() {
		// Для отладки создаем файл в текущей директории
		currentDir, err := os.Getwd()
		if err != nil {
			currentDir = "." // fallback
		}
		
		// Создаем отладочный файл в текущей директории (где запускается программа)
		debugPath := filepath.Join(currentDir, fmt.Sprintf("debug_%s.txt", time.Now().Format("2006-01-02")))
		
		// Если указана директория для логов, создаем ее
		if logDir != "" {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				initErr = fmt.Errorf("failed to create log directory: %v", err)
				return
			}
			logPath := filepath.Join(logDir, fmt.Sprintf("researcher_%s.log", time.Now().Format("2006-01-02")))
			file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				initErr = fmt.Errorf("failed to open log file: %v", err)
				return
			}
			
			logger = &RequestLogger{
				file:      file,
				logPath:   logPath,
				sampled:   make(map[string]int),
			}
		} else {
			logger = &RequestLogger{
				file:      nil, // Не создаем файл лога если не указана директория
				logPath:   "",
				sampled:   make(map[string]int),
			}
		}
		
		// Создаем отладочный файл
		debugFile, err := os.OpenFile(debugPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			if logger.file != nil {
				logger.file.Close()
			}
			initErr = fmt.Errorf("failed to open debug file: %v", err)
			return
		}
		
		logger.debugFile = debugFile
		
		// Пишем заголовок в отладочный файл
		debugFile.WriteString(fmt.Sprintf("=== Debug Log Started at %s ===\n", 
			time.Now().Format("2006-01-02 15:04:05")))
		debugFile.WriteString(fmt.Sprintf("Current directory: %s\n", currentDir))
		debugFile.WriteString(fmt.Sprintf("Debug file: %s\n", debugPath))
		if logger.file != nil {
			debugFile.WriteString(fmt.Sprintf("Log file: %s\n", logger.logPath))
		}
		debugFile.WriteString("========================================\n\n")
	})
	return initErr
}

// LogRequest логирует запрос с семплированием (1 из 10 постов)
func (rl *RequestLogger) LogRequest(endpoint string, data interface{}, success bool, response interface{}, errorMsg string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Семплирование: логируем только 1 из 10 запросов для постов
	if endpoint == "/posts" {
		sampleKey := fmt.Sprintf("%s:%d", endpoint, time.Now().Minute())
		
		count, _ := rl.sampled[sampleKey]
		if count < 9 {
			rl.sampled[sampleKey] = count + 1
			return
		}
		delete(rl.sampled, sampleKey)
	}

	// Формируем запись лога
	logEntry := fmt.Sprintf("[%s] %s | Success: %v", 
		time.Now().Format("2006-01-02 15:04:05"),
		endpoint,
		success)

	if !success && errorMsg != "" {
		logEntry += fmt.Sprintf(" | Error: %s", errorMsg)
	}

	// Добавляем краткую информацию о данных
	if endpoint == "/posts" {
		if postData, ok := data.(map[string]interface{}); ok {
			if title, ok := postData["title"].(string); ok {
				titleShort := title
				if len(titleShort) > 50 {
					titleShort = titleShort[:50] + "..."
				}
				logEntry += fmt.Sprintf(" | Title: %s", titleShort)
			}
		}
	}

	if response != nil {
		if respMap, ok := response.(map[string]interface{}); ok {
			if id, ok := respMap["post_id"]; ok && endpoint == "/posts" {
				logEntry += fmt.Sprintf(" | PostID: %v", id)
			}
		}
	}

	// Пишем в файл и в stdout
	logEntry += "\n"
	fmt.Print(logEntry) // В консоль
	if rl.file != nil {
		rl.file.WriteString(logEntry)
	}
}

func (rl *RequestLogger) DebugLog(module string, message string, data ...interface{}) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry := fmt.Sprintf("[%s] [%s] %s", 
		time.Now().Format("2006-01-02 15:04:05"),
		module,
		fmt.Sprintf(message, data...))

	entry += "\n"
	
	// Всегда пишем в консоль (для Docker logs)
	fmt.Printf("[DEBUG] %s", entry)
	
	// Если есть отладочный файл, пишем и в него
	if rl.debugFile != nil {
		rl.debugFile.WriteString(entry)
		rl.debugFile.Sync()
	}
}

// Close закрывает файлы логгера
func (rl *RequestLogger) Close() {
	rl.mu.Lock()
	defer rl.mu.Unlock() // ИСПРАВЛЕНО: было rl.unlock

	if rl.file != nil {
		rl.file.Close()
		rl.file = nil
	}
	
	if rl.debugFile != nil {
		rl.debugFile.WriteString(fmt.Sprintf("\n=== Debug Log Ended at %s ===\n", 
			time.Now().Format("2006-01-02 15:04:05")))
		rl.debugFile.Close()
		rl.debugFile = nil
	}
}

// GetLogger возвращает экземпляр логгера
func GetLogger() *RequestLogger {
	return logger
}

// GetDebugLogger возвращает экземпляр логгера для отладки
func GetDebugLogger() *RequestLogger {
	return logger
}