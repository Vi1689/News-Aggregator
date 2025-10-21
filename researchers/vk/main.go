package main

import (
	"fmt"
	"os"
	"time"

	"researcher-vk/internal/config"
	"researcher-vk/internal/vk"
	"researcher-vk/internal/vkToken"
	_ "researcher-vk/internal/vkToken"
)

func MakeGroupsSlice(accessToken string, conf config.ResearcherConfig) ([]vk.VKGroup, error) {
	if len(conf.Preferred_channels) == 0 {
		return vk.GetTopPopularGroups(accessToken, conf.Channel_limit)
	}
	return vk.GetGroupsByFullNames(accessToken, conf.Preferred_channels, conf.Channel_limit)
}

func main() {

	// автоматизация получения токена vk
	go func() {
		vkToken.GetValidToken()
	}()

	for {
		fmt.Printf("New Scan\n")
		accessToken := vkToken.GetAccessToken()
		if len(accessToken) == 0 {
			fmt.Printf("Открой браузер и перейди по ссылке: %s\n", vkToken.GetAuthUrl())
			fmt.Println("После авторизации скопируй code из URL и вставь сюда:")
			timer1 := time.NewTimer(time.Duration(10) * time.Second)
			<-timer1.C
			continue
		}
		//ConfigFileName := "../../config/researchers.xml"
		//ConfigFileName := "/usr/local/etc/vk-researcher/test_conf.xml"
		ConfigFileName := "test_conf.xml"

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

		for i, group := range groupsSlice {
			fmt.Printf("%d. %s (ID: %d, Участников: %d)\n", i+1, group.Name, group.ID, group.MembersCount)
			// получить post_limit постов
			postsSlice, _ := vk.GetGroupPosts(string(accessToken), group.ID, conf.Post_limit)

			// TODO: отправить запросы на из добавление на сервер
			for j, post := range postsSlice {
				fmt.Printf("%d. %s [%d] %d\n", j+1, post.Text, post.Comments, post.Likes)
				// получить комментарии
				commentsSlice, _ := vk.GetCommentsWithThreads(string(accessToken), post.AuthorID, post.ID, conf.Comment_limit)
				// отправить на сервер
				for k, comment := range commentsSlice {
					fmt.Printf("=================================%d. %s %s\n", k+1, comment.AuthorName, comment.Text)
				}
				MediaSlice, _ := vk.GetMediaFromPosts(string(accessToken), post.AuthorID, conf.Media_limit)
				// отправить на севрер
				for k, mediaInfo := range MediaSlice {
					fmt.Printf("---------------------%d. %s %s\n", k+1, mediaInfo.Type, mediaInfo.URL)
				}
			}

			// получить медиа
		}

		timer1 := time.NewTimer(time.Duration(conf.Research_period) * time.Minute)
		<-timer1.C
	}

}
