package Reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// -----------------------------
// Типы, которые мы возвращаем
// -----------------------------

type CommentThread struct {
	Count int       `json:"count"`
	Items []Comment `json:"items"`
}

type Comment struct {
	ID         string        `json:"id"`
	ParentID   string        `json:"parent_id"`
	Text       string        `json:"text"`
	AuthorName string        `json:"author"`
	Thread     CommentThread `json:"thread"`
	CreatedUTC float64       `json:"created_utc"`
}

// -----------------------------
// HTTP helpers
// -----------------------------

// MakeRedditRequest — твоя GET-функция (точно как просил)
func MakeRedditRequestComment(url, tokenStr string) (*http.Response, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "bearer "+tokenStr)
	req.Header.Set("User-Agent", "MyRedditApp/0.1 by Old_Supermarket6173")
	return client.Do(req)
}

// makeRedditPost — POST helper для /api/morechildren
func makeRedditPost(urlStr, tokenStr string, form url.Values) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "bearer "+tokenStr)
	req.Header.Set("User-Agent", "MyRedditApp/0.1 by Old_Supermarket6173")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client.Do(req)
}

// -----------------------------
// Внутренние структуры для парсинга Reddit JSON
// -----------------------------

type redditThing struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

type redditListing struct {
	Kind string `json:"kind"`
	Data struct {
		Children []redditThing `json:"children"`
	} `json:"data"`
}

type redditCommentData struct {
	ID         string          `json:"id"`
	ParentID   string          `json:"parent_id"`
	Body       string          `json:"body"`
	Author     string          `json:"author"`
	Replies    json.RawMessage `json:"replies"` // can be "" or listing
	CreatedUTC float64         `json:"created_utc"`
}

type redditMoreData struct {
	ID       string   `json:"id"`
	ParentID string   `json:"parent_id"`
	Children []string `json:"children"`
	Count    int      `json:"count"`
}

// morechildren response structure: { json: { data: { things: [ {kind, data}, ... ] } } }
type moreChildrenResp struct {
	JSON struct {
		Data struct {
			Things []redditThing `json:"things"`
		} `json:"data"`
	} `json:"json"`
}

// -----------------------------
// Парсеры
// -----------------------------

// ParseCommentsFromBody парсит один ответ Reddit /comments/{id}
// Возвращает список комментариев (flat slice) и карту parentID->childrenIDs (для обнаружения more)
func ParseCommentsFromBody(body []byte) ([]Comment, map[string][]string, error) {
	// Ответ от Reddit для comments/{id} — это массив length 2: [ postObj, commentsListing ]
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, nil, err
	}
	if len(arr) < 2 {
		return nil, nil, errors.New("unexpected comments response structure")
	}

	commentsListingRaw := arr[1]

	var listing redditListing
	if err := json.Unmarshal(commentsListingRaw, &listing); err != nil {
		return nil, nil, err
	}

	flat := make([]Comment, 0)
	moreMap := make(map[string][]string) // parentFullname -> []childIDs (from "more" nodes)

	// recursive walker
	var walk func(things []redditThing, parentFullname string)
	walk = func(things []redditThing, parentFullname string) {
		for _, th := range things {
			if th.Kind == "t1" {
				var data redditCommentData
				if err := json.Unmarshal(th.Data, &data); err != nil {
					// skip on parse error
					continue
				}
				c := Comment{
					ID:         data.ID,
					ParentID:   data.ParentID,
					Text:       data.Body,
					AuthorName: data.Author,
					CreatedUTC: data.CreatedUTC,
					Thread:     CommentThread{Count: 0, Items: nil}, // will fill later
				}
				flat = append(flat, c)

				// replies can be "" or a listing
				if len(data.Replies) > 0 {
					// if replies is not empty string — parse listing
					var repliesListing redditListing
					if err := json.Unmarshal(data.Replies, &repliesListing); err == nil {
						walk(repliesListing.Data.Children, "t1_"+data.ID) // parent Fullname for children is t1_<id>
					}
				}
			} else if th.Kind == "more" {
				var md redditMoreData
				if err := json.Unmarshal(th.Data, &md); err != nil {
					continue
				}
				// md.ParentID is parent fullname (t1_xxx or t3_post)
				// append children IDs under that parent
				if len(md.Children) > 0 {
					moreMap[md.ParentID] = append(moreMap[md.ParentID], md.Children...)
				}
			}
		}
	}

	walk(listing.Data.Children, "") // top-level parentFullname is post's fullname (t3_<id>) handled in nodes

	return flat, moreMap, nil
}

// ParseCommentsFromCombined принимает объединённый JSON (массив ответов) и парсит все комментарии в []Comment
func ParseCommentsFromCombined(combined []byte) ([]Comment, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(combined, &raws); err != nil {
		return nil, err
	}
	all := make([]Comment, 0)
	for _, r := range raws {
		flat, _, err := ParseCommentsFromBody(r)
		if err != nil {
			continue
		}
		all = append(all, flat...)
	}
	return all, nil
}

// -----------------------------
// Основная функция FetchComments
// -----------------------------
// Параметры:
// - accessToken, nameSubreddit, postID (post.ID), maxCountComment (<=0 — без ограничений)
//
// Возвращает:
// - combinedRaw []byte (JSON array of raw responses) — первый элемент main response, последующие - morechildren responses
// - []Comment — дерево комментариев с Thread заполненным
// - error
// -----------------------------
func FetchComments(accessToken, nameSubreddit, postID string, maxCountComment int) ([]byte, []Comment, error) {
	if postID == "" {
		return nil, nil, errors.New("empty postID")
	}
	if nameSubreddit == "" {
		return nil, nil, errors.New("empty subreddit name")
	}

	// retry policy
	totalRetries := 0

	// store raw responses: first the main comments response, then subsequent morechildren responses
	rawResponses := make([][]byte, 0)

	// flat map of comments by ID
	commentsMap := make(map[string]*Comment)

	// parent -> children ids mapping (to build tree later)
	childrenMap := make(map[string][]string) // parentFullname -> []childIDs

	// collected more IDs that we still need to fetch (childIDs)
	pendingMoreIDs := make([]string, 0)

	// helper to perform GET with retry policy (uses MakeRedditRequest)
	doGet := func(urlStr string) ([]byte, int, error) {
		var resp *http.Response
		var err error
		for {
			resp, err = MakeRedditRequestComment(urlStr, accessToken)
			if err != nil {
				totalRetries++
				if totalRetries > 30 {
					return nil, 0, fmt.Errorf("stopped after %d retries", totalRetries)
				}
				time.Sleep(2 * time.Second)
				continue
			}
			if resp.StatusCode == 429 {
				resp.Body.Close()
				totalRetries++
				if totalRetries > 30 {
					return nil, resp.StatusCode, fmt.Errorf("stopped after %d retries (429)", totalRetries)
				}
				time.Sleep(2 * time.Second)
				continue
			}
			// other statuses and success break loop
			break
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		return body, resp.StatusCode, err
	}

	// helper to perform POST with retry policy (uses makeRedditPost)
	doPost := func(urlStr string, form url.Values) ([]byte, int, error) {
		var resp *http.Response
		var err error
		for {
			resp, err = makeRedditPost(urlStr, accessToken, form)
			if err != nil {
				totalRetries++
				if totalRetries > 30 {
					return nil, 0, fmt.Errorf("stopped after %d retries", totalRetries)
				}
				time.Sleep(2 * time.Second)
				continue
			}
			if resp.StatusCode == 429 {
				resp.Body.Close()
				totalRetries++
				if totalRetries > 30 {
					return nil, resp.StatusCode, fmt.Errorf("stopped after %d retries (429)", totalRetries)
				}
				time.Sleep(2 * time.Second)
				continue
			}
			break
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		return body, resp.StatusCode, err
	}

	// 1) Получаем основной ответ /comments/{postID}
	urlMain := fmt.Sprintf("https://oauth.reddit.com/comments/%s?limit=500", postID)
	body, status, err := doGet(urlMain)
	if err != nil {
		// если превысили retry limit — вернуть найденное (пустое)
		if strings.Contains(err.Error(), "stopped after") {
			combinedRaw, _ := combineRawResponses(rawResponses)
			return combinedRaw, nil, nil
		}
		return nil, nil, err
	}
	// check statuses
	if status == 401 {
		return nil, nil, fmt.Errorf("unauthorized (401)")
	}
	if status == 404 {
		return nil, nil, fmt.Errorf("post not found (404)")
	}
	// сохраняем основной raw
	rawResponses = append(rawResponses, body)

	// парсим основной ответ в плоский список и собираем moreMap
	flat, moreMap, perr := ParseCommentsFromBody(body)
	if perr != nil {
		// если не смогли распарсить — возвращаем то, что есть
		combinedRaw, _ := combineRawResponses(rawResponses)
		return combinedRaw, nil, perr
	}

	// заполняем карты из flat
	for i := range flat {
		c := flat[i] // copy
		commentsMap[c.ID] = &c
		// parent relationship: keep mapping parentFullname -> childID
		childrenMap[c.ParentID] = append(childrenMap[c.ParentID], c.ID)
	}

	// добавляем все more IDs в очередь, сохраняя parent mapping
	for parentFullname, ids := range moreMap {
		for _, cid := range ids {
			pendingMoreIDs = append(pendingMoreIDs, cid)
			// record parent mapping — we will know parent when we insert comment (comment.ParentID)
			// But store mapping parentFullname->childIDs as well for safety:
			childrenMap[parentFullname] = append(childrenMap[parentFullname], cid)
		}
	}

	// If no pending more IDs, we can build tree and return
	// But we must still respect maxCountComment (unlimited if <=0)
	unlimited := maxCountComment <= 0

	// If we already have enough comments per maxCountComment, return
	if !unlimited && len(commentsMap) >= maxCountComment {
		combinedRaw, _ := combineRawResponses(rawResponses)
		// build tree from commentsMap and childrenMap
		commentsTree := buildCommentTree(commentsMap, childrenMap, postID, maxCountComment)
		return combinedRaw, commentsTree, nil
	}

	// 2) Обработка pending more IDs: нужно делать запросы к /api/morechildren
	// morechildren принимает параметр children (comma-separated ids), link_id=t3_<postID>
	// Лучше отправлять пакетами — допустим batch size 100
	const batchSize = 100

	// We'll process pendingMoreIDs until empty or until retry limit reached or until we hit maxCountComment (if limited)
	for i := 0; i < len(pendingMoreIDs); {
		// prepare batch
		end := i + batchSize
		if end > len(pendingMoreIDs) {
			end = len(pendingMoreIDs)
		}
		batch := pendingMoreIDs[i:end]
		i = end // advance

		// build form
		form := url.Values{}
		form.Set("api_type", "json")
		form.Set("link_id", "t3_"+postID)
		form.Set("children", strings.Join(batch, ","))

		// do POST to https://oauth.reddit.com/api/morechildren
		moreURL := "https://oauth.reddit.com/api/morechildren"
		moreBody, status, merr := doPost(moreURL, form)
		if merr != nil {
			// if retry exceeded, return current results
			if strings.Contains(merr.Error(), "stopped after") {
				combinedRaw, _ := combineRawResponses(rawResponses)
				commentsTree := buildCommentTree(commentsMap, childrenMap, postID, maxCountComment)
				return combinedRaw, commentsTree, nil
			}
			// otherwise skip this batch and continue
			continue
		}
		if status != 200 {
			// skip on non-200 but try to continue
			// If 401 -> token expired
			if status == 401 {
				combinedRaw, _ := combineRawResponses(rawResponses)
				commentsTree := buildCommentTree(commentsMap, childrenMap, postID, maxCountComment)
				return combinedRaw, commentsTree, fmt.Errorf("unauthorized (401) during morechildren")
			}
			// else skip this batch
			continue
		}

		// append raw response to rawResponses
		rawResponses = append(rawResponses, moreBody)

		// parse morechildren response
		var mcr moreChildrenResp
		if err := json.Unmarshal(moreBody, &mcr); err != nil {
			// malformed response — skip
			continue
		}
		// mcr.JSON.Data.Things are redditThing objects (likely t1 comments)
		for _, thing := range mcr.JSON.Data.Things {
			if thing.Kind == "t1" {
				var d redditCommentData
				if err := json.Unmarshal(thing.Data, &d); err != nil {
					continue
				}
				// create Comment
				c := Comment{
					ID:         d.ID,
					ParentID:   d.ParentID,
					Text:       d.Body,
					AuthorName: d.Author,
					CreatedUTC: d.CreatedUTC,
					Thread:     CommentThread{Count: 0, Items: nil},
				}
				// store in map if not exists
				if _, exists := commentsMap[c.ID]; !exists {
					commentsMap[c.ID] = &c
				}
				// ensure childrenMap contains parent -> child mapping (parentFullname)
				childrenMap[d.ParentID] = append(childrenMap[d.ParentID], d.ID)

				// if replies present in this returned thing, handle them
				if len(d.Replies) > 0 {
					var repliesListing redditListing
					if err := json.Unmarshal(d.Replies, &repliesListing); err == nil {
						// process nested replies similarly: convert to Comment and add
						// we'll reuse ParseCommentsFromBody logic by marshalling a small listing
						mini := redditListing{Kind: "Listing"}
						mini.Data.Children = repliesListing.Data.Children
						miniRaw, _ := json.Marshal(mini)
						subFlat, subMore, _ := ParseCommentsFromBody(append([]byte("[],"), miniRaw...)) // hack: not ideal; instead process directly
						// better approach: process repliesListing.Data.Children directly:
						for _, ch := range repliesListing.Data.Children {
							if ch.Kind == "t1" {
								var rd redditCommentData
								if err := json.Unmarshal(ch.Data, &rd); err == nil {
									cc := Comment{
										ID:         rd.ID,
										ParentID:   rd.ParentID,
										Text:       rd.Body,
										AuthorName: rd.Author,
										CreatedUTC: rd.CreatedUTC,
										Thread:     CommentThread{Count: 0, Items: nil},
									}
									if _, exists := commentsMap[cc.ID]; !exists {
										commentsMap[cc.ID] = &cc
									}
									childrenMap[rd.ParentID] = append(childrenMap[rd.ParentID], rd.ID)
								}
							} else if ch.Kind == "more" {
								var md redditMoreData
								if err := json.Unmarshal(ch.Data, &md); err == nil {
									childrenMap[md.ParentID] = append(childrenMap[md.ParentID], md.Children...)
									// append these children to pendingMoreIDs to process later
									pendingMoreIDs = append(pendingMoreIDs, md.Children...)
								}
							}
						}
						_ = subFlat
						_ = subMore
					}
				}
			} else if thing.Kind == "more" {
				// more inside morechildren response (rare) — extract children ids and enqueue
				var md redditMoreData
				if err := json.Unmarshal(thing.Data, &md); err == nil {
					for _, cid := range md.Children {
						pendingMoreIDs = append(pendingMoreIDs, cid)
						childrenMap[md.ParentID] = append(childrenMap[md.ParentID], cid)
					}
				}
			}
		}

		// check count limit
		if !unlimited && len(commentsMap) >= maxCountComment {
			combinedRaw, _ := combineRawResponses(rawResponses)
			commentsTree := buildCommentTree(commentsMap, childrenMap, postID, maxCountComment)
			return combinedRaw, commentsTree, nil
		}
	} // end for pendingMoreIDs batches

	// После всех morechildren: собираем финальное дерево комментариев
	combinedRaw, _ := combineRawResponses(rawResponses)
	commentsTree := buildCommentTree(commentsMap, childrenMap, postID, maxCountComment)
	return combinedRaw, commentsTree, nil
}

// -----------------------------
// Помощники для построения дерева комментариев
// -----------------------------

// buildCommentTree собирает дерево комментариев из commentsMap и childrenMap.
// parentFullnameForTop = "t3_<postID>"
func buildCommentTree(commentsMap map[string]*Comment, childrenMap map[string][]string, postID string, maxCount int) []Comment {
	// create result top-level list
	topLevel := make([]Comment, 0)

	// helper to compute total descendants count recursively
	var computeCount func(id string) int
	computeCount = func(id string) int {
		full := "t1_" + id
		childIDs := childrenMap[full]
		count := 0
		for _, cid := range childIDs {
			count++ // direct child
			// further descendants
			count += computeCount(cid)
		}
		return count
	}

	// build Comment objects with Thread.Items filled (direct children)
	// We'll also enforce maxCount: stop adding further items if reached
	addedCount := 0
	unlimited := maxCount <= 0

	// The top-level parent fullname for direct comments is "t3_<postID>"
	topParent := "t3_" + postID
	topChildren := childrenMap[topParent]

	for _, cid := range topChildren {
		cptr, ok := commentsMap[cid]
		if !ok {
			// if we have an ID but no parsed comment (maybe placeholder), skip
			continue
		}
		// build thread items (direct children)
		childFull := "t1_" + cid
		childIDs := childrenMap[childFull]
		items := make([]Comment, 0, len(childIDs))
		for _, ccid := range childIDs {
			if cp, ok2 := commentsMap[ccid]; ok2 {
				items = append(items, *cp)
			}
		}
		// compute descendants count
		count := 0
		for _, ccid := range childIDs {
			count += 1
			count += computeCount(ccid)
		}
		node := Comment{
			ID:         cptr.ID,
			ParentID:   cptr.ParentID,
			Text:       cptr.Text,
			AuthorName: cptr.AuthorName,
			CreatedUTC: cptr.CreatedUTC,
			Thread:     CommentThread{Count: count, Items: items},
		}
		topLevel = append(topLevel, node)
		addedCount++
		if !unlimited && addedCount >= maxCount {
			return topLevel
		}
	}

	return topLevel
}

// -----------------------------
// PrintComments — красивая печать дерева (для main.go)
// -----------------------------
func PrintComments(comments []Comment) {
	var printRec func(c Comment, depth int)
	printRec = func(c Comment, depth int) {
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
