package Reddit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"researcher-reddit/token"
	"time"
)

// -------------------------------
// Структура поста (по требованиям)
// -------------------------------
type Post struct {
	Title      string  `json:"title"`
	Text       string  `json:"selftext"`
	AuthorName string  `json:"author"`
	Comments   int     `json:"num_comments"`
	Votes      int     `json:"score"`
	Date       float64 `json:"created_utc"`
	URL        string  `json:"url"`
	ID         string  `json:"id"`
}

// ---------------------------------------------------
// Пользовательская функция (точно такая, как просил)
// ---------------------------------------------------
func makeRedditRequestPost(url, tokenStr string) (*http.Response, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "bearer "+tokenStr)
	req.Header.Set("User-Agent", "MyRedditApp/0.1 by Old_Supermarket6173")
	return client.Do(req)
}

// ---------------------------------------------------
// Парсер одного HTTP-ответа Reddit -> []Post
// ---------------------------------------------------
func ParsePostsFromBody(body []byte, nameSubreddit string) ([]Post, error) {
	var parsed struct {
		Data struct {
			Children []struct {
				Data Post `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	posts := make([]Post, 0, len(parsed.Data.Children))
	for _, ch := range parsed.Data.Children {
		posts = append(posts, ch.Data)
	}

	for i := range posts {
		posts[i].URL = "https://www.reddit.com/r/" + nameSubreddit + "/comments/" + posts[i].ID
	}

	return posts, nil
}

// for _, post := range posts {
// 	post.URL = "https://www.reddit.com/r/" + nameSubreddit + "/comments/" + post.ID
// }

// ---------------------------------------------------
// Парсер объединённого raw JSON (массив ответов) -> []Post
// ---------------------------------------------------
func ParsePostsFromCombined(combinedRaw []byte, nameSubreddit string) ([]Post, error) {
	// combinedRaw должен быть JSON-массивом, где каждый элемент - обычный ответ от Reddit
	var rawResponses []json.RawMessage
	if err := json.Unmarshal(combinedRaw, &rawResponses); err != nil {
		return nil, err
	}

	allPosts := make([]Post, 0)
	for _, raw := range rawResponses {
		posts, err := ParsePostsFromBody(raw, nameSubreddit)
		if err != nil {
			// если парсинг одного ответа не удался, пропускаем его
			continue
		}
		allPosts = append(allPosts, posts...)
	}
	return allPosts, nil
}

// ---------------------------------------------------
// Вспомогательная: превращает срез raw ответов в единый JSON-массив
// ---------------------------------------------------
func combineRawResponses(raws [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, r := range raws {
		if i > 0 {
			buf.WriteByte(',')
		}
		// r может быть уже JSON-объектом
		buf.Write(r)
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// ---------------------------------------------------
// Основная функция: FetchPosts
//
// Параметры:
// - accessToken string
// - nameSubreddit string
// - dateFrom *time.Time (nil / zero означает не задано)
// - dateBy *time.Time (nil / zero означает не задано)
// - maxCountPost int (<=0 означает "без ограничений")
//
// Возвращает:
// - []byte : объединённый JSON (массив) всех сырых ответов от Reddit
// - []Post : отфильтрованные посты (по created_utc согласно логике)
// - error  : nil в нормальном случае; при превышении 30 попыток при 429 возвращает найденное и nil
// ---------------------------------------------------
func FetchPosts(
	accessToken string,
	nameSubreddit string,
	dateFrom *time.Time,
	dateBy *time.Time,
	maxCountPost int,
) ([]byte, []Post, error) {

	// Подготовка: лимит на запрос (Reddit ограничивает 100)
	var reqLimit int
	if maxCountPost < 100 {
		reqLimit = maxCountPost
	} else {
		reqLimit = 100
	}

	// Срез для хранения сырых ответов (каждый элемент - один JSON-ответ от Reddit)
	rawResponses := make([][]byte, 0)

	// Срез для итоговых отфильтрованных постов
	filteredPosts := make([]Post, 0)

	// Курсор пагинации (full name t3_<id>)
	var afterFullname string // пустой — первая страница

	// Счётчик суммарных retry-ожиданий (для поведения "более 30 попыток")
	totalRetries := 0

	// Вспомогательная функция проверки, попадает ли пост в диапазон
	inRange := func(p Post) bool {
		created := time.Unix(int64(p.Date), 0)
		// оба не заданы — всё в диапазоне
		if (dateFrom == nil || dateFrom.IsZero()) && (dateBy == nil || dateBy.IsZero()) {
			return true
		}
		// только dateFrom задан
		if dateFrom != nil && !dateFrom.IsZero() && (dateBy == nil || dateBy.IsZero()) {
			return !created.Before(*dateFrom) // created >= dateFrom
		}
		// только dateBy задан
		if dateBy != nil && !dateBy.IsZero() && (dateFrom == nil || dateFrom.IsZero()) {
			return !created.After(*dateBy) // created <= dateBy
		}
		// оба заданы
		if dateFrom != nil && !dateFrom.IsZero() && dateBy != nil && !dateBy.IsZero() {
			return (!created.Before(*dateFrom)) && (!created.After(*dateBy)) // dateFrom <= created <= dateBy
		}
		return true
	}

	// Если maxCountPost <= 0 интерпретируем как "без ограничений"
	unlimited := maxCountPost <= 0

	// Safety: если subreddit пустой — сразу ошибка
	if nameSubreddit == "" {
		return nil, nil, errors.New("empty subreddit name")
	}

	// Основной цикл: запрашиваем страницы, пока не получили достаточно или пока Reddit даёт данные
	for {
		// Формируем URL
		// Используем /new с сортировкой по времени (новые сначала)
		url := fmt.Sprintf("https://oauth.reddit.com/r/%s/new?limit=%d", nameSubreddit, reqLimit)
		if afterFullname != "" {
			// Параметр after ожидает fullname вида t3_<id>
			url = url + "&after=" + afterFullname
		}

		// Выполнение запроса с retry-политикой при 429 / временных ошибках
		var resp *http.Response
		var err error

		for {
			resp, err = makeRedditRequestPost(url, accessToken)
			if err != nil {
				// сетевые ошибки считаем retryable
				totalRetries++
				if totalRetries > 30 {
					// превышен лимит попыток — возвращаем то, что есть
					combinedRaw, _ := combineRawResponses(rawResponses)
					return combinedRaw, filteredPosts, nil
				}
				time.Sleep(2 * time.Second)
				continue
			}

			// получили ответ — проверяем статус
			if resp.StatusCode == 429 {
				// rate limited — ждать и повторить
				totalRetries++
				resp.Body.Close()
				if totalRetries > 30 {
					combinedRaw, _ := combineRawResponses(rawResponses)
					return combinedRaw, filteredPosts, nil
				}
				time.Sleep(2 * time.Second)
				continue
			}

			// для других статус-кодов выходим из retry-loop и обработаем их далее
			break
		}

		// читаем тело
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			// считаем это временной ошибкой: retry общую логику
			totalRetries++
			if totalRetries > 30 {
				combinedRaw, _ := combineRawResponses(rawResponses)
				return combinedRaw, filteredPosts, nil
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// проверка статуса (например 404, 401, 500)
		if resp.StatusCode != http.StatusOK {
			// Для 401 возможно токен протух — возвращаем ошибку, чтобы внешний код попытался обновить токен
			if resp.StatusCode == 401 {
				accessToken, err = token.GetAccessToken()
				if err != nil {
					combinedRaw, _ := combineRawResponses(rawResponses)
					return combinedRaw, filteredPosts, fmt.Errorf("unauthorized (401) — check token")
				}
				continue
			}
			// Для 404 — сабреддит не найден — завершаем с ошибкой
			if resp.StatusCode == 404 {
				combinedRaw, _ := combineRawResponses(rawResponses)
				return combinedRaw, filteredPosts, fmt.Errorf("subreddit not found (404): %s", nameSubreddit)
			}
			// Для других статусов — возвращаем пока что то, что есть (как частичный результат)
			combinedRaw, _ := combineRawResponses(rawResponses)
			return combinedRaw, filteredPosts, fmt.Errorf("reddit returned status %d", resp.StatusCode)
		}

		// Успешный ответ: сохраняем сырой ответ
		rawResponses = append(rawResponses, body)

		// Парсим текущую страницу в посты
		postsPage, perr := ParsePostsFromBody(body, nameSubreddit)
		if perr != nil {
			// если парсинг одной страницы не удался — пропускаем её, но продолжаем пагинацию
			// (сохраняли raw, так что пользователь сможет посмотреть)
			// Для безопасности — не увеличиваем afterFullname, чтобы не застрять: пробуем взять last id если есть
			if len(postsPage) == 0 {
				// не стали получать данные — пытаемся дальше (но если нет данных вообще, то останавливаем)
				// если страница пуста — это конец
				break
			}
		}

		// Фильтрация и добавление постов из этой страницы
		for _, p := range postsPage {
			if inRange(p) {
				filteredPosts = append(filteredPosts, p)
				// проверка лимита
				if !unlimited && len(filteredPosts) >= maxCountPost {
					combinedRaw, _ := combineRawResponses(rawResponses)
					return combinedRaw, filteredPosts[:maxCountPost], nil
				}
			}
		}

		// Подготовка курсора для следующей страницы:
		// Если на странице нет постов — это конец
		if len(postsPage) == 0 {
			break
		}
		// берем ID последнего поста в странице (самого старого в этой выборке)
		lastPost := postsPage[len(postsPage)-1]
		if lastPost.ID == "" {
			// нет id — останавливаемся
			break
		}
		nextAfter := "t3_" + lastPost.ID
		// если nextAfter такой же как предыдущий afterFullname — цикл не двигается — завершение
		if nextAfter == afterFullname {
			break
		}
		afterFullname = nextAfter

		// reset retry counter после успешного запроса
		// (но мы считаем суммарные попытки отдельно; решение: не сбрасывать totalRetries,
		//  чтобы соблюдалась глобальная политка "более 30 попыток" — это то, что просил пользователь)
		// если хочешь, можно reset локальный retries per request.
	}

	// По выходу собираем объединённый raw JSON и возвращаем найденные посты
	if len(rawResponses) == 0 && len(filteredPosts) == 0 {
		return nil, nil, errors.New("no data fetched")
	}
	combinedRaw, cerr := combineRawResponses(rawResponses)
	if cerr != nil {
		// если по какой-то причине не удалось скомбинировать, всё равно возвращаем найденное
		return nil, filteredPosts, nil
	}
	return combinedRaw, filteredPosts, nil
}

// ---------------------------------------------------
// Утилита для печати постов (для main.go)
// ---------------------------------------------------
func PrintPosts(posts []Post) {
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
