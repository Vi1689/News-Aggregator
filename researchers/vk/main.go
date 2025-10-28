package main

import (
	"fmt"
	"os"
	"time"

	"researcher-vk/internal/config"
	"researcher-vk/internal/sendRequests"
	"researcher-vk/internal/vk"
	_ "researcher-vk/internal/vkToken"
)

var groupOffset int

func MakeGroupsSlice(accessToken string, conf config.ResearcherConfig) ([]vk.VKGroup, error) {
	if len(conf.Preferred_channels) == 0 {
		return vk.GetTopPopularGroups(accessToken, conf.Channel_limit, groupOffset)
	}
	groupOffset += conf.Channel_limit
	return vk.GetGroupsByFullNames(accessToken, conf.Preferred_channels, conf.Channel_limit)
}

func main() {
	timer1 := time.NewTimer(time.Duration(10) * time.Second)
	<-timer1.C

	// автоматизация получения токена vk
	/*go func() {
		vkToken.GetValidToken()
	}()*/
	// вк = говно

	accessTokenFileName := "/usr/local/etc/vk-researcher/access_token"
	//accessTokenFileName := "access_token"
	accessToken, err := os.ReadFile(accessTokenFileName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Добавить источник VK в начале (один раз)
	sourceID, err := sendRequests.AddVKSource()
	if err != nil {
		fmt.Printf("Failed to add VK source: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Printf("sourceID = %d\n\n", sourceID)
	}

	groupOffset = 0
	for {
		fmt.Printf("New Scan\n")
		//accessToken := vkToken.GetAccessToken()
		/*if len(accessToken) == 0 {
			fmt.Printf("Открой браузер и перейди по ссылке: %s\n", vkToken.GetAuthUrl())
			fmt.Println("После авторизации скопируй code из URL и вставь сюда:")
			timer1 := time.NewTimer(time.Duration(10) * time.Second)
			<-timer1.C
			continue
		}*/

		ConfigFileName := "/usr/local/etc/vk-researcher/test_conf.xml"
		//ConfigFileName := "test_conf.xml"

		xmlData, err := os.ReadFile(ConfigFileName)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", xmlData)

		var conf config.ResearcherConfig
		conf, err = config.ParseConfigFile(string(xmlData))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = config.ValidateConfig(conf)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// сгенерировать список групп
		var groupsSlice []vk.VKGroup
		groupsSlice, err = MakeGroupsSlice(string(accessToken), conf)
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

			postsSlice, err := vk.GetGroupPosts(string(accessToken), group.ID, conf.Post_limit)
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
				/*commentsSlice, err := vk.GetCommentsWithThreads(string(accessToken), post.AuthorID, post.ID, conf.Comment_limit)
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
				}*/

				MediaSlice, _ := vk.GetMediaFromPosts(string(accessToken), post.AuthorID, conf.Media_limit)
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

		timer1 := time.NewTimer(time.Duration(conf.Research_period) * time.Second)
		<-timer1.C
	}

}
