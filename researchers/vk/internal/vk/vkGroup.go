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

// Структура для группы
type VKGroup struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ScreenName   string `json:"screen_name"`
	MembersCount int    `json:"members_count"`
}

// Структура ответа
type VKGroupsResponse struct {
	Response struct {
		Count int       `json:"count"`
		Items []VKGroup `json:"items"`
	} `json:"response"`
}

// Поиск популярных групп
func GetTopPopularGroups(accessToken string, count int, offset int) ([]VKGroup, error) {
    // Ограничиваем количество
    if count > 20 {
        count = 20
        fmt.Printf("Reducing group count to %d\n", count)
    }
    
    baseURL := "https://api.vk.com/method/groups.search"
    params := url.Values{}
    params.Set("access_token", accessToken)
    params.Set("v", "5.199")
    params.Set("q", "новости")
    params.Set("type", "group")
    params.Set("sort", "6")
    params.Set("count", strconv.Itoa(count))
    params.Set("offset", strconv.Itoa(offset))
    params.Set("fields", "members_count,name,screen_name")

    fullURL := baseURL + "?" + params.Encode()
    
    // Добавляем задержку
    time.Sleep(500 * time.Millisecond)
    
    resp, err := http.Get(fullURL)
    if err != nil {
        return nil, fmt.Errorf("HTTP request error: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("VK API error %d: %s", resp.StatusCode, string(body))
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response error: %w", err)
    }

    var vkResp VKGroupsResponse
    if err := json.Unmarshal(body, &vkResp); err != nil {
        return nil, fmt.Errorf("JSON parse error: %w", err)
    }

    fmt.Printf("Found %d groups, got %d items\n", vkResp.Response.Count, len(vkResp.Response.Items))
    
    // Фильтруем группы без названия
    var filteredGroups []VKGroup
    for _, group := range vkResp.Response.Items {
        if group.Name != "" && group.MembersCount > 0 {
            // Убедимся, что screen_name не пустой
            if group.ScreenName == "" {
                group.ScreenName = strconv.Itoa(group.ID)
            }
            filteredGroups = append(filteredGroups, group)
        }
    }

    return filteredGroups, nil
}

// Получение групп по названиям
func GetGroupsByFullNames(accessToken string, names string, n int) ([]VKGroup, error) {
	if len(names) == 0 {
		return []VKGroup{}, nil
	}

	nameList := strings.Split(names, ",")
	for i := range nameList {
		nameList[i] = strings.TrimSpace(nameList[i])
	}

	var allGroups []VKGroup

	for _, fullName := range nameList {
		if len(fullName) == 0 {
			continue
		}

		// Задержка между запросами
		time.Sleep(1 * time.Second)
		
		baseURL := "https://api.vk.com/method/groups.search"
		params := url.Values{}
		params.Set("access_token", accessToken)
		params.Set("v", "5.199")
		params.Set("q", fullName)
		params.Set("type", "group")
		params.Set("count", "5")
		params.Set("fields", "members_count,name,screen_name,is_closed")

		fullURL := baseURL + "?" + params.Encode()
		
		// ДОБАВИМ ОТЛАДКУ
		fmt.Printf("DEBUG: Searching group: %s\n", fullName)
		
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		resp, err := client.Get(fullURL)
		if err != nil {
			fmt.Printf("DEBUG: Request error for '%s': %v\n", fullName, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("DEBUG: HTTP error %d for '%s'\n", resp.StatusCode, fullName)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("DEBUG: Read error for '%s': %v\n", fullName, err)
			continue
		}

		// ДОБАВИМ ПРЕВЬЮ ОТВЕТА
		fmt.Printf("DEBUG: Response for '%s': %s\n", fullName, string(body[:min(200, len(body))]))
		
		var vkResp struct {
			Response struct {
				Count int `json:"count"`
				Items []struct {
					ID           int    `json:"id"`
					Name         string `json:"name"`
					ScreenName   string `json:"screen_name"`
					MembersCount int    `json:"members_count"`
					IsClosed     int    `json:"is_closed"` // 0 - открытая, 1 - закрытая, 2 - частная
				} `json:"items"`
			} `json:"response"`
		}
		
		if err := json.Unmarshal(body, &vkResp); err != nil {
			fmt.Printf("DEBUG: JSON parse error for '%s': %v\n", fullName, err)
			continue
		}

		fmt.Printf("DEBUG: Found %d groups for '%s'\n", vkResp.Response.Count, fullName)
		
		// Добавляем найденные группы
		if len(vkResp.Response.Items) > 0 {
			for _, item := range vkResp.Response.Items {
				fmt.Printf("DEBUG: Group: ID=%d, Name='%s', ScreenName='%s', Members=%d, IsClosed=%d\n",
					item.ID, item.Name, item.ScreenName, item.MembersCount, item.IsClosed)
					
				// Проверяем что группа открытая
				if item.IsClosed == 0 {
					group := VKGroup{
						ID:           item.ID,
						Name:         item.Name,
						ScreenName:   item.ScreenName,
						MembersCount: item.MembersCount,
					}
					if group.ScreenName == "" {
						group.ScreenName = fullName
					}
					allGroups = append(allGroups, group)
					fmt.Printf("DEBUG: Added OPEN group: %s (ID: %d)\n", group.Name, group.ID)
					break // Берем первую открытую группу
				} else {
					fmt.Printf("DEBUG: Skipping CLOSED group: %s (IsClosed=%d)\n", item.Name, item.IsClosed)
				}
			}
		}
	}

	fmt.Printf("Found %d groups by names\n", len(allGroups))
	return allGroups, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}