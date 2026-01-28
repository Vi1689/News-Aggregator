package main

import (
	"fmt"
	"os"
	"time"
	"strings" // ДОБАВИТЬ ЭТУ СТРОКУ

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
    // Инициализируем логгер
    if err := sendRequests.InitLogger(""); err != nil {
        fmt.Printf("Failed to init logger: %v\n", err)
    }
    
    defer func() {
        if logger := sendRequests.GetLogger(); logger != nil {
            logger.Close()
        }
    }()

    currentDir, _ := os.Getwd()
    fmt.Printf("Current directory: %s\n", currentDir)
    
    if debugLogger := sendRequests.GetDebugLogger(); debugLogger != nil {
        debugLogger.DebugLog("MAIN", "Application started in directory: %s", currentDir)
    }

    // Небольшая задержка для запуска
    time.Sleep(5 * time.Second)

    accessTokenFileName := "/usr/local/etc/vk-researcher/access_token"
    accessToken, err := os.ReadFile(accessTokenFileName)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    // Добавить источник VK
    sourceID, err := sendRequests.AddVKSource()
    if err != nil {
        fmt.Printf("Failed to add VK source: %v\n", err)
        os.Exit(1)
    } else {
        fmt.Printf("sourceID = %d\n\n", sourceID)
    }

    groupOffset = 0
    for {
        fmt.Printf("\n=== New Scan Cycle ===\n")
        
        if logger := sendRequests.GetDebugLogger(); logger != nil {
            logger.DebugLog("MAIN", "Starting new scan cycle")
        }

        ConfigFileName := "/usr/local/etc/vk-researcher/test_conf.xml"
        xmlData, err := os.ReadFile(ConfigFileName)

        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
		
        if logger := sendRequests.GetDebugLogger(); logger != nil {
            logger.DebugLog("CONFIG", "Config file loaded, size: %d bytes", len(xmlData))
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

        // Уменьшаем лимиты для избежания Flood Control
        if conf.Channel_limit > 10 {
            fmt.Printf("Reducing channel limit from %d to 10 to avoid flood control\n", conf.Channel_limit)
            conf.Channel_limit = 10
        }
        
        if conf.Post_limit > 50 {
            fmt.Printf("Reducing post limit from %d to 50 to avoid flood control\n", conf.Post_limit)
            conf.Post_limit = 50
        }

        // Получаем список групп
        var groupsSlice []vk.VKGroup
        groupsSlice, err = MakeGroupsSlice(string(accessToken), conf)
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }

        fmt.Printf("Found %d groups to process\n", len(groupsSlice))

        // Обрабатываем группы с задержками
        for i, group := range groupsSlice {
            fmt.Printf("\n[%d/%d] Processing group: %s (ID: %d, Members: %d)\n", 
                i+1, len(groupsSlice), group.Name, group.ID, group.MembersCount)
            
            // Задержка между группами
            if i > 0 {
                delay := 8 * time.Second
                fmt.Printf("Waiting %v before next group...\n", delay)
                time.Sleep(delay)
            }

            // Создаем автора для группы
            authorName := fmt.Sprintf("VK Group: %s", group.Name)
            authorID, err := sendRequests.AddVKAuthor(authorName)
            if err != nil {
                fmt.Printf("WARNING: Failed to create author for group %s: %v\n", group.Name, err)
                // Пропускаем группу если не можем создать автора
                continue
            }

            // Создаем канал
            channelID, err := sendRequests.AddVKChannel(group, sourceID)
            if err != nil {
                fmt.Printf("ERROR: Failed to add channel for group %s: %v\n", group.Name, err)
                continue
            }

            fmt.Printf("✓ Channel added with ID: %d, Author ID: %d\n", channelID, authorID)
            fmt.Printf("Getting posts for group: %s (ID: %d)\n", group.Name, group.ID)
            
            // Получаем посты с задержкой
            time.Sleep(1 * time.Second)
            
            postsSlice, err := vk.GetGroupPostsWithRetry(string(accessToken), group.ID, conf.Post_limit)
            if err != nil {
                fmt.Printf("ERROR in GetGroupPosts: %v\n", err)
                fmt.Printf("Skipping group %s due to API error\n", group.Name)
                
                // При ошибке Flood control увеличиваем задержку
                if err.Error() == "flood control" {
                    fmt.Printf("Flood control detected, increasing delay...\n")
                    time.Sleep(10 * time.Second)
                }
                continue
            }
            
            fmt.Printf("Got %d posts\n", len(postsSlice))
            
            if len(postsSlice) == 0 {
                fmt.Printf("No posts found for group %s\n", group.Name)
                continue
            }

            // Показываем информацию о первом посте
            if len(postsSlice) > 0 {
                fmt.Printf("First post preview: ID=%d, Date: %v, Likes: %d, Comments: %d\n", 
                    postsSlice[0].ID, 
                    time.Unix(postsSlice[0].Date, 0).Format("2006-01-02 15:04:05"),
                    postsSlice[0].Likes, 
                    postsSlice[0].Comments)
            }

            if logger := sendRequests.GetDebugLogger(); logger != nil {
                logger.DebugLog("POSTS", "Retrieved %d posts for group %s", len(postsSlice), group.Name)
            }

            // Ограничиваем количество постов для обработки
            postsToProcess := len(postsSlice)
            if postsToProcess > 20 {
                postsToProcess = 20
                fmt.Printf("Limiting to %d posts to avoid overloading\n", postsToProcess)
            }

            // Отправляем посты
            for j := 0; j < postsToProcess; j++ {
                post := postsSlice[j]
                fmt.Printf("Processing post %d/%d: ID=%d\n", 
                    j+1, postsToProcess, post.ID)
                
                // Задержка между постами
                if j > 0 {
                    time.Sleep(500 * time.Millisecond)
                }
                
                // Добавляем пост
                postID, err := sendRequests.AddVKPost(post, channelID, authorID, group.Name)
                if err != nil {
                    fmt.Printf("ERROR: Failed to add post [%d]%d: %v\n", j, post.ID, err)
                    
                    // При ошибке сервера делаем паузу
                    if strings.Contains(err.Error(), "server error") {
                        fmt.Printf("Server error, waiting 5 seconds...\n")
                        time.Sleep(5 * time.Second)
                    }
                    continue
                }

                fmt.Printf("✓ Post %d added successfully with postID: %d\n", post.ID, postID)

                // Получаем и добавляем медиа (если есть лимит)
                if conf.Media_limit > 0 && postID > 0 {
                    MediaSlice, _ := vk.GetMediaFromPosts(string(accessToken), post.AuthorID, 5) // Ограничим 5 медиа
                    
                    if len(MediaSlice) > 0 {
                        fmt.Printf("Found %d media items for post %d\n", len(MediaSlice), post.ID)
                        
                        for k, mediaInfo := range MediaSlice {
                            fmt.Printf("Adding media %d/%d: Type=%s\n", 
                                k+1, len(MediaSlice), mediaInfo.Type)
                            
                            err := sendRequests.AddVKMedia(mediaInfo, postID)
                            if err != nil {
                                fmt.Printf("ERROR: Failed to add media: %v\n", err)
                            }
                            time.Sleep(200 * time.Millisecond)
                        }
                    }
                }
            }
            
            fmt.Printf("✓ Group %s processed: %d posts added\n", group.Name, postsToProcess)
        }

        fmt.Printf("\n=== Scan cycle completed, waiting %d seconds ===\n", conf.Research_period)

        timer1 := time.NewTimer(time.Duration(conf.Research_period) * time.Second)
        <-timer1.C
    }
}