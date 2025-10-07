package vk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Структура для группы (упрощённая)
type VKGroup struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ScreenName   string `json:"screen_name"`
	MembersCount int    `json:"members_count"`
}

// Структура ответа от VK API
type VKGroupsResponse struct {
	Response struct {
		Count int       `json:"count"`
		Items []VKGroup `json:"items"`
	} `json:"response"`
}

// Структура ответа от VK API для groups.getById
type VKGroupsByIdResponse struct {
	Response []VKGroup `json:"response"`
}

// Структура ответа от VK API для groups.search
type VKGroupsSearchResponse struct {
	Response struct {
		Count int       `json:"count"`
		Items []VKGroup `json:"items"`
	} `json:"response"`
}

// Функция для поиска n самых популярных групп VK
func GetTopPopularGroups(accessToken string, n int) ([]VKGroup, error) {
	// Базовый URL VK API для поиска групп
	baseURL := "https://api.vk.com/method/groups.search"

	// Параметры запроса
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("v", "5.131")                               // Версия API
	params.Set("q", "")                                    // Пустой запрос для поиска всех групп
	params.Set("type", "group")                            // Тип: группы
	params.Set("sort", "6")                                // Сортировка: по количеству участников (убывание)
	params.Set("count", strconv.Itoa(n))                   // Количество результатов
	params.Set("fields", "members_count,name,screen_name") // Поля для получения (включая members_count)

	// Формируем полный URL
	fullURL := baseURL + "?" + params.Encode()
	fmt.Printf("%s\n", fullURL)

	// Делаем HTTP GET запрос
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к VK API: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("VK API вернул ошибку: %s, тело: %s", resp.Status, string(body))
	}

	// Читаем и парсим JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	var vkResp VKGroupsResponse
	if err := json.Unmarshal(body, &vkResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
	}

	// Возвращаем найденные группы (уже отсортированы по популярности)
	return vkResp.Response.Items, nil
}

// Функция для получения групп по слайсу их screen_name
// Функция для получения групп по полному названию (изменённая с твоей оригинальной)
func GetGroupsByFullNames(accessToken string, names string, n int) ([]VKGroup, error) {
	if len(names) == 0 {
		return []VKGroup{}, nil // Пустой слайс, если вход пустой
	}

	// Разбиваем строку на список названий (через запятую)
	nameList := strings.Split(names, ",")
	for i := range nameList {
		nameList[i] = strings.TrimSpace(nameList[i]) // Убираем пробелы
	}

	var allGroups []VKGroup // Слайс для всех найденных групп

	// Проходим по каждому названию и делаем поиск
	for _, fullName := range nameList {
		if len(fullName) == 0 {
			continue // Пропускаем пустые
		}

		// Базовый URL VK API для поиска групп
		baseURL := "https://api.vk.com/method/groups.search"

		// Параметры запроса
		params := url.Values{}
		params.Set("access_token", accessToken)
		params.Set("v", "5.131")                               // Версия API
		params.Set("q", fullName)                              // Запрос по названию
		params.Set("type", "group")                            // Тип: группы
		params.Set("count", "20")                              // До 20 результатов на запрос
		params.Set("fields", "members_count,name,screen_name") // Поля

		// Формируем полный URL
		fullURL := baseURL + "?" + params.Encode()
		fmt.Printf("Запрос для '%s': %s\n", fullName, fullURL) // Отладка

		// Задаём таймаут
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		// Делаем HTTP GET запрос
		resp, err := client.Get(fullURL)
		if err != nil {
			return nil, fmt.Errorf("ошибка запроса к VK API для '%s': %w", fullName, err)
		}
		defer resp.Body.Close()

		// Проверяем статус ответа
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("VK API вернул ошибку для '%s': %s, тело: %s", fullName, resp.Status, string(body))
		}

		// Читаем и парсим JSON
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения ответа для '%s': %w", fullName, err)
		}

		var vkResp VKGroupsSearchResponse
		if err := json.Unmarshal(body, &vkResp); err != nil {
			return nil, fmt.Errorf("ошибка парсинга JSON для '%s': %w", fullName, err)
		}

		// Добавляем найденные группы в общий слайс
		allGroups = append(allGroups, vkResp.Response.Items[:n]...)
	}

	// Возвращаем все найденные группы
	fmt.Printf("Всего найдено групп: %d\n", len(allGroups))
	return allGroups, nil
}
