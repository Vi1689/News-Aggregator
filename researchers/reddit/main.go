package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"researcher-reddit/Reddit"
	"researcher-reddit/config"
	"researcher-reddit/sendRequests"
	"researcher-reddit/token"
	"strings"
	"time"
)

func main() {
	timer1 := time.NewTimer(time.Duration(10) * time.Second)
	<-timer1.C

	// Добавить источник Reddit в начале (один раз)
	sourceID, err := sendRequests.AddRedditSource()
	if err != nil {
		fmt.Printf("Failed to add Reddit source: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Printf("sourceID = %d\n\n", sourceID)
	}

	for {
		fmt.Printf("New Scan\n")

		// --- Получаем токен ---
		tok, err := token.GetAccessToken()
		if err != nil {
			log.Fatalf("Ошибка получения токена: %v", err)
		}

		// ConfigFileName := "test_conf.xml"
		ConfigFileName := "/usr/local/etc/reddit-researcher/test_conf.xml"
		xmlData, err := os.ReadFile(ConfigFileName)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

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
		fmt.Printf("Конфигурация считана\n")

		var subs []Reddit.Subreddit
		_, subs, err = MakeGroupsSlice(tok, conf)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("Сабредиты получены\n")

		// df := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		// db := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)

		for _, sr := range subs {
			// запрос на добавление группы
			channelID, err := sendRequests.AddRedditChannel(sr, sourceID)
			if err != nil {
				fmt.Printf("Failed to add channel for group %s: %v\n", sr.DisplayName, err)
				continue // Пропустить группу, если ошибка
			} else {
				fmt.Printf("channelID = %d\n\n", channelID)
			}

			name := sr.DisplayName
			var posts []Reddit.Post
			_, posts, err = Reddit.FetchPosts(tok, name, nil, nil, conf.Post_limit)
			if err != nil {
				fmt.Println(err)
				continue
			}

			for j, post := range posts {
				postID, err := sendRequests.AddRedditPost(post, channelID, nil)
				if err != nil {
					fmt.Printf("Failed to add post [%d]%s(%d): %v\n", j, post.ID, postID, err)
					continue
				}

				var comments []Reddit.Comment
				_, comments, err = Reddit.FetchComments(tok, name, post.ID, conf.Comment_limit)
				if err != nil {
					fmt.Println(err)
					continue
				}

				for _, comment := range comments {
					// запрос на добавление комментария
					err := sendRequests.AddRedditComment(comment, postID, nil)
					if err != nil {
						fmt.Printf("Failed to add comment: %v\n", err)
					}
				}

				var medias []Reddit.Media
				_, medias, err = Reddit.FetchPostMedia(tok, name, post.ID, conf.Media_limit)
				if err != nil {
					fmt.Println(err)
					continue
				}

				for _, media := range medias {
					err := sendRequests.AddVKMedia(media, postID)
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

func MakeGroupsSlice(accessToken string, conf config.ResearcherConfig) ([]byte, []Reddit.Subreddit, error) {
	if len(conf.Preferred_channels) == 0 {
		return Reddit.GetTopPopularGroups(accessToken, conf.Channel_limit)
	}

	// Разбиваем строку на список названий (через запятую)
	nameList := strings.Split(conf.Preferred_channels, ",")
	for i := range nameList {
		nameList[i] = strings.TrimSpace(nameList[i]) // Убираем пробелы
	}
	return Reddit.FetchSubreddits(accessToken, nameList)
}

// printJSON красиво форматирует JSON-ответ
func PrintJSON(body []byte) {
	var pretty map[string]interface{}
	if err := json.Unmarshal(body, &pretty); err != nil {
		fmt.Println(string(body))
		return
	}
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Println(string(out))
}

// printSubreddits красиво выводит найденые сабредиты
func PrintSubreddits(subreddits []Reddit.Subreddit) {
	fmt.Printf("\n🔥 Топ популярных сабреддитов:\n")
	for i, sr := range subreddits {
		fmt.Printf("%d. %s (%d подписчиков)\n   r/%s\n   https://reddit.com%s\n\n",
			i+1, sr.Title, sr.Subscribers, sr.DisplayName, sr.URL)
	}
}

// ---------------------------------------------------
// Утилита для печати постов (для main.go)
// ---------------------------------------------------
func PrintPosts(posts []Reddit.Post) {
	for i, p := range posts {
		t := time.Unix(int64(p.Date), 0).UTC().Format("2006-01-02 15:04:05")
		fmt.Printf("%d) %s\n", i+1, p.Title)
		fmt.Printf("   Author: %s | Votes: %d | Comments: %d\n", p.AuthorName, p.Votes, p.Comments)
		fmt.Printf("   Date (UTC): %s\n", t)
		fmt.Printf("   URL: %s\n", p.URL)
		fmt.Printf("   ID: %s\n", p.ID)
		if p.Text != "" {
			// коротко показываем начало текста
			runes := []rune(p.Text)
			limit := 200
			if len(runes) < limit {
				limit = len(runes)
			}
			fmt.Printf("   Text (preview): %s\n", string(runes[:limit]))
		}
		fmt.Println()
	}
}

// -----------------------------
// PrintComments — красивая печать дерева (для main.go)
// -----------------------------
func PrintComments(comments []Reddit.Comment) {
	var printRec func(c Reddit.Comment, depth int)
	printRec = func(c Reddit.Comment, depth int) {
		prefix := strings.Repeat("  ", depth)
		t := time.Unix(int64(c.CreatedUTC), 0).UTC().Format("2006-01-02 15:04:05")
		fmt.Printf("%s- ID: %s | Author: %s | Date: %s\n", prefix, c.ID, c.AuthorName, t)
		if c.Text != "" {
			// print short preview
			r := []rune(c.Text)
			limit := 200
			if len(r) < limit {
				limit = len(r)
			}
			fmt.Printf("%s  %s\n", prefix, string(r[:limit]))
		}
		// print thread info
		if len(c.Thread.Items) > 0 {
			for _, ch := range c.Thread.Items {
				printRec(ch, depth+1)
			}
		}
	}
	for _, c := range comments {
		printRec(c, 0)
		fmt.Println()
	}
}

// функция для main: вывод распарсенных медиа
func PrintPostMedia(medias []Reddit.Media) {
	for i, m := range medias {
		fmt.Printf("%d) Type: %s, URL: %s\n", i+1, m.Type, m.URL)
	}
}
