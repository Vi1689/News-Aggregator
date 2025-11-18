package main

import (
	"fmt"
	"os"

	"researcher-test/internal/sendRequests"
	"researcher-test/internal/vk"
	_ "researcher-test/internal/vkToken"
)

func main() {
	// Добавить источник VK в начале (один раз)
	sourceID, err := sendRequests.AddVKSource()
	if err != nil {
		fmt.Printf("Failed to add VK source: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Printf("sourceID = %d\n\n", sourceID)
	}

	for range 100 {
		// сгенерировать список групп
		var groupsSlice []vk.VKGroup

		groupsSlice, err = vk.GetRandGroups(1000, 0)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		for _, group := range groupsSlice {
			// запрос на добавление группы
			channelID, err := sendRequests.AddVKChannel(group, sourceID)
			if err != nil {
				fmt.Printf("Failed to add channel for group %s: %v\n", group.Name, err)
				continue // Пропустить группу, если ошибка
			}

			postsSlice, err := vk.GetRandPosts(1000, 1)
			if err != nil {
				fmt.Printf("Failed to get posts: %v\n", err)
				continue
			}

			// TODO: отправить запросы на из добавление на сервер
			for j, post := range postsSlice {
				// запрос на добавление постов
				postID, err := sendRequests.AddVKPost(post, channelID, nil)
				if err != nil {
					fmt.Printf("Failed to add post [%d]%d(%d): %v\n", j, post.ID, postID, err)
					continue
				}
				// получить комментарии
				commentsSlice, err := vk.GetRandComments(1000, 10)
				if err != nil {
					fmt.Printf("Failed to get comments: %v\n", err)
					continue
				}
				// отправить на сервер
				for _, comment := range commentsSlice {
					// запрос на добавление комментария
					err := sendRequests.AddVKComment(comment, postID, nil)
					if err != nil {
						fmt.Printf("Failed to add comment: %v\n", err)
					}
				}

				MediaSlice := vk.GetRandMedia(100)
				fmt.Printf("=================================len(MediaSlice)=%d\n", len(MediaSlice))
				// запрос на добавление медиа
				for k, mediaInfo := range MediaSlice {
					fmt.Printf("---------------------%d. %s %s\n", k+1, mediaInfo.Type, mediaInfo.URL)
					err := sendRequests.AddVKMedia(mediaInfo, postID)
					if err != nil {
						fmt.Printf("Failed to add media: %v\n", err)
					}
				}
			}
		}
	}
}
