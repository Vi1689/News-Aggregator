package vk

import (
	"math/rand"

	"github.com/google/uuid"
)

// Структура для группы (расширенная с администраторами)
type VKGroup struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	ScreenName   string     `json:"screen_name"`
	MembersCount int        `json:"members_count"`
	Contacts     []struct { // Администраторы/контакты группы
		UserID int    `json:"user_id"`
		Desc   string `json:"desc"`
	} `json:"contacts"`
}

// Структура ответа от VK API
type VKGroupsResponse struct {
	Response struct {
		Count int       `json:"count"`
		Items []VKGroup `json:"items"`
	} `json:"response"`
}

// Структура ответа от VK API для groups.search
type VKGroupsSearchResponse struct {
	Response struct {
		Count int       `json:"count"`
		Items []VKGroup `json:"items"`
	} `json:"response"`
}

// Функция для генерации count случайных групп
func GetRandGroups(count int, offset int) ([]VKGroup, error) {
	var vkResp VKGroupsResponse

	vkResp.Response.Count = count
	for range count {
		var newGroup VKGroup
		newGroup.ID = rand.Intn(100)
		newGroup.Name = uuid.New().String()
		newGroup.ScreenName = uuid.New().String()
		newGroup.MembersCount = rand.Intn(10000)

		contacts_num := rand.Intn(10)
		newGroup.Contacts = make([]struct {
			UserID int    "json:\"user_id\""
			Desc   string "json:\"desc\""
		}, contacts_num)
		for j := range contacts_num {
			newGroup.Contacts[j].UserID = rand.Intn(10000)
			newGroup.Contacts[j].Desc = uuid.New().String()
		}
		vkResp.Response.Items = append(vkResp.Response.Items, newGroup)
	}

	return vkResp.Response.Items, nil
}
