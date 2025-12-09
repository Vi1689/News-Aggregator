package Reddit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"researcher-reddit/token"
)

// Структура для парсинга сабредитов из ответа Reddit
type Subreddit struct {
	DisplayName string `json:"display_name"`
	Title       string `json:"title"`
	Subscribers int    `json:"subscribers"`
	URL         string `json:"url"`
}

// makeRedditRequest выполняет запрос с токеном
func MakeRedditRequestSubreddit(url, tokenStr string) (*http.Response, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "bearer "+tokenStr)
	req.Header.Set("User-Agent", "MyRedditApp/0.1 by Old_Supermarket6173")
	return client.Do(req)
}

func GetTopPopularGroups(accessToken string, count int) ([]byte, []Subreddit, error) {
	url := "https://oauth.reddit.com/subreddits/popular"
	if count != 0 {
		url = url + "?limit=" + fmt.Sprintf("%d", count)
	}
	fmt.Println(url)

	// --- Делаем запрос к Reddit ---
	resp, err := MakeRedditRequestSubreddit(url, accessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка при запросе: %w", err)
	}
	defer resp.Body.Close()

	// --- Проверяем, не истёк ли токен ---
	if resp.StatusCode == 401 {
		fmt.Println("⚠️ Токен устарел, обновляем...")
		resp.Body.Close()

		newTok, err := token.GetAccessToken()
		if err != nil {
			return nil, nil, fmt.Errorf("ошибка обновления токена: %w", err)
		}

		resp, err = MakeRedditRequestSubreddit(url, newTok)
		if err != nil {
			return nil, nil, fmt.Errorf("ошибка при повторном запросе: %w", err)
		}
		defer resp.Body.Close()
	}

	body, _ := io.ReadAll(resp.Body)

	subreddits, err := parseTopSubreddits(body)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	return body, subreddits, nil
}

func parseTopSubreddits(body []byte) ([]Subreddit, error) {
	var result struct {
		Data struct {
			Children []struct {
				Data Subreddit `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var subs []Subreddit
	for _, child := range result.Data.Children {
		subs = append(subs, child.Data)
	}
	return subs, nil
}

// FetchSubreddits делает запрос к Reddit API для списка сабреддитов.
// Возвращает: []byte (весь сырой JSON), []Subreddit (распарсенные данные), error.
func FetchSubreddits(accessToken string, names []string) ([]byte, []Subreddit, error) {
	client := &http.Client{}
	allSubreddits := []Subreddit{}
	allRaw := []byte{}

	for _, name := range names {
		url := fmt.Sprintf("https://oauth.reddit.com/r/%s/about", name)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			fmt.Printf("⚠️ Ошибка создания запроса для %s: %v\n", name, err)
			continue
		}

		req.Header.Set("Authorization", "bearer "+accessToken)
		req.Header.Set("User-Agent", "MyRedditApp/0.1 by YOUR_REDDIT_USERNAME")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("⚠️ Ошибка запроса к Reddit для %s: %v\n", name, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close() // закрываем тело сразу
		if err != nil {
			fmt.Printf("⚠️ Ошибка чтения ответа %s: %v\n", name, err)
			continue
		}

		// Проверка на статус ответа
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("⚠️ Пропускаем %s: статус %d\n", name, resp.StatusCode)
			continue
		}

		// Добавляем в общий сырой массив JSON
		allRaw = append(allRaw, body...)

		// Парсим сабреддит
		subreddit, err := ParseSubreddit(body)
		if err != nil {
			fmt.Printf("⚠️ Ошибка парсинга %s: %v\n", name, err)
			continue
		}

		allSubreddits = append(allSubreddits, subreddit)
	}

	// Если вообще ничего не удалось получить
	if len(allSubreddits) == 0 {
		return allRaw, nil, fmt.Errorf("не удалось получить ни одного сабреддита")
	}

	return allRaw, allSubreddits, nil
}

// ParseSubreddit парсит JSON одного сабреддита в структуру Subreddit
func ParseSubreddit(body []byte) (Subreddit, error) {
	var parsed struct {
		Data Subreddit `json:"data"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return Subreddit{}, err
	}

	return parsed.Data, nil
}
