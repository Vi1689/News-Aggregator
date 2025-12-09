package Reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"researcher-reddit/token"
	"time"
)

// ---------------------------
// Media structure
// ---------------------------
type Media struct {
	Type string `json:"type"` // "image", "video", "link", "document", etc.
	URL  string `json:"url"`
}

// ---------------------------
// Safe HTTP helper
// ---------------------------
// MakeRedditRequest выполняет GET-запрос с токеном и защищённо возвращает *http.Response.
// Никогда не возвращает ненадёжный resp с nil body без ошибки.
func MakeRedditRequest(urlStr, tokenStr string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "bearer "+tokenStr)
	req.Header.Set("User-Agent", "MyRedditApp/0.1 by Old_Supermarket6173")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("empty response")
	}
	// Не закрываем тело здесь — вызывающий код должен закрыть после чтения.
	return resp, nil
}

// ---------------------------
// FetchPostMedia
// ---------------------------
// Получает сырое тело поста и распаривает все вложенные медиа.
// Возвращает:
// - combinedRaw — JSON-массив, где каждый элемент — один сырой ответ (обычно один элемент)
// - medias — распарсенные медиа
// - err
//
// Поведение retry: при сетевых ошибках/429/5xx — ждать 2s и повторять, максимум 30 попыток.
// Если превышено — возвращаются любые частичные результаты (combinedRaw может быть nil или содержать последний ответ).
func FetchPostMedia(accessToken, nameSubreddit, postID string, maxCountMedia int) ([]byte, []Media, error) {
	if accessToken == "" {
		return nil, nil, errors.New("empty access token")
	}
	if nameSubreddit == "" {
		return nil, nil, errors.New("empty subreddit name")
	}
	if postID == "" {
		return nil, nil, errors.New("empty postID")
	}

	// URL — без markdown
	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/comments/%s?raw_json=1", nameSubreddit, postID)

	var rawResponses [][]byte
	var parsedMedias []Media
	attempts := 0

	for {
		attempts++
		resp, err := MakeRedditRequest(url, accessToken)
		if err != nil {
			// retryable network error
			if attempts > 30 {
				// возвращаем то, что есть
				break
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// читаем тело
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if attempts > 30 {
				break
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// статус
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			// rate limit or server error
			if attempts > 30 {
				// собрать комбинированный и вернуть частично
				rawResponses = append(rawResponses, body)
				break
			}
			// сохраняем raw чтобы не терять (необязательно)
			rawResponses = append(rawResponses, body)
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode == 401 {
			// unauthorized
			accessToken, err = token.GetAccessToken()
			if err != nil {
				_ = append(rawResponses, body)
				combined, _ := json.Marshal([]json.RawMessage{json.RawMessage(body)})
				return combined, nil, fmt.Errorf("ошибка обновления токена: %w", err)
			}
			rawResponses = append(rawResponses, body)
			continue
		}
		if resp.StatusCode == 404 {
			_ = append(rawResponses, body)
			combined, _ := json.Marshal([]json.RawMessage{json.RawMessage(body)})
			return combined, nil, fmt.Errorf("post not found (404)")
		}
		// прочие 2xx -> parse
		rawResponses = append(rawResponses, body)

		// Попробуем распарсить body — но сначала безопасно определить форму корня
		// Reddit обычно возвращает массив [ postObj, commentsObj ], но иногда разные ответы приходят как объект.
		// Попробуем сначала как массив, иначе как объект.
		var parsedAsArray []interface{}
		var parsedAsObject map[string]interface{}
		var rootIsArray bool
		if err := json.Unmarshal(body, &parsedAsArray); err == nil && len(parsedAsArray) > 0 {
			rootIsArray = true
		} else if err2 := json.Unmarshal(body, &parsedAsObject); err2 == nil {
			rootIsArray = false
		} else {
			// не получилось распарсить ни как массив, ни как объект — считаем это ошибкой парсинга
			combined, _ := json.Marshal(toRawMessages(rawResponses))
			return combined, nil, fmt.Errorf("can't parse reddit response JSON")
		}

		var postMap map[string]interface{}

		if rootIsArray {
			// expected shape: [ postListing, commentsListing ]
			arr := parsedAsArray
			if len(arr) == 0 {
				// пусто — продолжаем/выход
				break
			}
			// first element может быть listing or object with data.children
			if first, ok := arr[0].(map[string]interface{}); ok {
				// ideally first["data"]["children"][0]["data"] is the post
				if dataIf, ok := first["data"].(map[string]interface{}); ok {
					if childrenIf, ok := dataIf["children"].([]interface{}); ok && len(childrenIf) > 0 {
						if firstChild, ok := childrenIf[0].(map[string]interface{}); ok {
							if pd, ok := firstChild["data"].(map[string]interface{}); ok {
								postMap = pd
							}
						}
					} else {
						// fallback: maybe arr[0] is already the post data
						postMap = first
					}
				} else {
					// fallback
					postMap = first
				}
			}
		} else {
			// root is object — try to find post inside data.children[0].data
			obj := parsedAsObject
			if dataIf, ok := obj["data"].(map[string]interface{}); ok {
				if childrenIf, ok := dataIf["children"].([]interface{}); ok && len(childrenIf) > 0 {
					if firstChild, ok := childrenIf[0].(map[string]interface{}); ok {
						if pd, ok := firstChild["data"].(map[string]interface{}); ok {
							postMap = pd
						}
					}
				} else {
					// fallback — maybe obj is the post itself
					postMap = obj
				}
			} else {
				// fallback — treat whole object as post
				postMap = obj
			}
		}

		if postMap == nil {
			// не получилось извлечь post data — возвращаем raw
			combined, _ := json.Marshal(toRawMessages(rawResponses))
			return combined, nil, fmt.Errorf("cannot find post data in response")
		}

		// Парсим медиа из postMap
		parsedMedias = collectMediaFromPostMap(postMap, maxCountMedia)
		// Удаляем дубликаты по URL (редко, но бывает)
		parsedMedias = dedupeMedia(parsedMedias)

		// Готово — выходим
		break
	}

	combined, _ := json.Marshal(toRawMessages(rawResponses))
	return combined, parsedMedias, nil
}

// ---------------------------
// collectMediaFromPostMap — универсальный парсер медиа для одного post data map
// ---------------------------
func collectMediaFromPostMap(post map[string]interface{}, maxCount int) []Media {
	res := make([]Media, 0)

	// 1) gallery_data + media_metadata
	if gd, ok := post["gallery_data"].(map[string]interface{}); ok {
		if items, ok := gd["items"].([]interface{}); ok {
			if mediaMeta, ok := post["media_metadata"].(map[string]interface{}); ok {
				for _, it := range items {
					if im, ok := it.(map[string]interface{}); ok {
						if mediaID, ok := im["media_id"].(string); ok {
							if mmRaw, ok := mediaMeta[mediaID].(map[string]interface{}); ok {
								// type mime is mmRaw["m"], sizes mmRaw["s"] -> {"u": "..."}
								typ := ""
								if t, ok := mmRaw["m"].(string); ok {
									typ = t
								}
								if s, ok := mmRaw["s"].(map[string]interface{}); ok {
									if u, ok := s["u"].(string); ok {
										u = html.UnescapeString(u)
										res = append(res, Media{Type: typOrKind(typ, "image"), URL: u})
										if maxCount > 0 && len(res) >= maxCount {
											return res
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// 2) preview.images
	if preview, ok := post["preview"].(map[string]interface{}); ok {
		if images, ok := preview["images"].([]interface{}); ok {
			for _, im := range images {
				if imMap, ok := im.(map[string]interface{}); ok {
					if src, ok := imMap["source"].(map[string]interface{}); ok {
						if u, ok := src["url"].(string); ok {
							u = html.UnescapeString(u)
							res = append(res, Media{Type: "image", URL: u})
							if maxCount > 0 && len(res) >= maxCount {
								return res
							}
						}
					}
					// sometimes "resolutions" contain other useful urls - skip for now
				}
			}
		}
	}

	// 3) media.reddit_video or secure_media.reddit_video
	if media, ok := post["media"].(map[string]interface{}); ok {
		if rv, ok := media["reddit_video"].(map[string]interface{}); ok {
			if u, ok := rv["fallback_url"].(string); ok {
				res = append(res, Media{Type: "video", URL: u})
				if maxCount > 0 && len(res) >= maxCount {
					return res
				}
			}
		}
	}
	if sm, ok := post["secure_media"].(map[string]interface{}); ok {
		if rv, ok := sm["reddit_video"].(map[string]interface{}); ok {
			if u, ok := rv["fallback_url"].(string); ok {
				res = append(res, Media{Type: "video", URL: u})
				if maxCount > 0 && len(res) >= maxCount {
					return res
				}
			}
		}
	}

	// 4) media (external), e.g., oembed, reddit_video_preview etc.
	if media, ok := post["media"].(map[string]interface{}); ok {
		// embedded oembed may contain "thumbnail_url"
		if oembed, ok := media["oembed"].(map[string]interface{}); ok {
			if thumb, ok := oembed["thumbnail_url"].(string); ok {
				res = append(res, Media{Type: "image", URL: html.UnescapeString(thumb)})
				if maxCount > 0 && len(res) >= maxCount {
					return res
				}
			}
			// if media has other URLs, try to collect link
			if urlS, ok := media["type"].(string); ok && urlS != "" {
				// nothing special
				_ = urlS
			}
		}
	}

	// 5) url_overridden_by_dest (direct link to resource)
	if uod, ok := post["url_overridden_by_dest"].(string); ok && uod != "" {
		res = append(res, Media{Type: "link", URL: html.UnescapeString(uod)})
		if maxCount > 0 && len(res) >= maxCount {
			return res
		}
	}

	// 6) url field (often i.redd.it or external)
	if u, ok := post["url"].(string); ok && u != "" {
		res = append(res, Media{Type: "link", URL: html.UnescapeString(u)})
		if maxCount > 0 && len(res) >= maxCount {
			return res
		}
	}

	// 7) crosspost_parent_list — sometimes contains post data with media
	if cpList, ok := post["crosspost_parent_list"].([]interface{}); ok {
		for _, cp := range cpList {
			if cpMap, ok := cp.(map[string]interface{}); ok {
				more := collectMediaFromPostMap(cpMap, maxCount-len(res))
				res = append(res, more...)
				if maxCount > 0 && len(res) >= maxCount {
					return res
				}
			}
		}
	}

	return res
}

// ---------------------------
// Утилиты
// ---------------------------

// типизация: если mime содержит image/..., считаем image, если video/..., video, иначе fallback kind
func typOrKind(mime, fallback string) string {
	if mime == "" {
		return fallback
	}
	if len(mime) >= 6 && mime[:6] == "image/" {
		return "image"
	}
	if len(mime) >= 6 && mime[:6] == "video/" {
		return "video"
	}
	return fallback
}

// удаляем дубликаты по URL (сохраняя первый тип)
func dedupeMedia(in []Media) []Media {
	seen := make(map[string]bool)
	out := make([]Media, 0, len(in))
	for _, m := range in {
		if m.URL == "" {
			continue
		}
		if seen[m.URL] {
			continue
		}
		seen[m.URL] = true
		out = append(out, m)
	}
	return out
}

// преобразует [][]byte -> []json.RawMessage
func toRawMessages(raws [][]byte) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(raws))
	for _, r := range raws {
		out = append(out, json.RawMessage(r))
	}
	return out
}
