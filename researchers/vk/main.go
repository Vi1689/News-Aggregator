package main

import (
	"fmt"
	"os"
	"time"

	"researcher-vk/internal/config"
	"researcher-vk/internal/vk"
)

func MakeGroupsSlice(accessToken string, conf config.ResearcherConfig) ([]vk.VKGroup, error) {
	if len(conf.Preferred_channels) == 0 {
		return vk.GetTopPopularGroups(accessToken, conf.Channel_limit)
	}
	return vk.GetGroupsByFullNames(accessToken, conf.Preferred_channels, conf.Channel_limit)
}

func main() {

	accessTokenFileName := "access_token"
	accessToken, err := os.ReadFile(accessTokenFileName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for {
		fmt.Printf("New Scan")
		//ConfigFileName := "../../config/researchers.xml"
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
		var groupSlice []vk.VKGroup
		groupSlice, err = MakeGroupsSlice(string(accessToken), conf)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		for i, group := range groupSlice {
			fmt.Printf("%d. %s (ID: %d, Участников: %d)\n", i+1, group.Name, group.ID, group.MembersCount)
			// получить post_limit постов
			postsSlice, _ := vk.GetGroupPosts(string(accessToken), group.ID, conf.Post_limit)
			// TODO: отправить запросы на из добавление на сервер
			for j, post := range postsSlice {
				fmt.Printf("%d. %s [%d] %d\n", j+1, post.Text, post.Comments, post.Likes)
				// получить комментарии
			}

			// получить медиа
		}

		timer1 := time.NewTimer(time.Duration(conf.Research_period) * time.Minute)
		<-timer1.C
	}

}
