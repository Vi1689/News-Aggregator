package vkToken

// получение vk токена
// для первоначального получения токена необходима авторизация в браузере, затем скрипт будет автоматически обновлять токен

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var refresh_token string
var access_token string
var auth_url string

var rw sync.RWMutex
var expiresAt time.Time

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

const (
	clientID     = "YOUR_CLIENT_ID"     // Замени на свой из VK Developers
	clientSecret = "YOUR_CLIENT_SECRET" // Замени на свой (секретно храни!)
	redirectURI  = "http://localhost:8080/callback"
)

func InitTokenStrings() {
	refresh_token = ""
	access_token = ""
}

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func GetAccessToken() string {
	rw.RLock()
	defer rw.RUnlock()
	return access_token
}

func GetAuthUrl() string {
	return auth_url
}

func setAccessToken(new_access_token string) {
	rw.Lock()
	defer rw.Unlock()
	access_token = new_access_token
}

func refreshAccessToken() error {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refresh_token)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	resp, err := http.PostForm("https://oauth.vk.com/access_token", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return err
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("failed to refresh token: %s", string(body))
	}

	expiresAt = time.Now().Add(time.Hour)
	setAccessToken(tokenResp.AccessToken)
	refresh_token = tokenResp.RefreshToken
	return nil
}

func getNewToken() error {
	state := generateState()
	authURL := fmt.Sprintf("https://oauth.vk.com/authorize?client_id=%s&redirect_uri=%s&scope=wall,groups&response_type=code&state=%s&v=5.131",
		clientID, redirectURI, state)

	fmt.Printf("Открой браузер и перейди по ссылке: %s\n", authURL)
	auth_url = authURL
	fmt.Println("После авторизации скопируй code из URL и вставь сюда:")

	var code string
	fmt.Scanln(&code)

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("code", code)

	resp, err := http.PostForm("https://oauth.vk.com/access_token", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return err
	}

	expiresAt = time.Now().Add(time.Hour)
	setAccessToken(tokenResp.AccessToken)
	refresh_token = tokenResp.RefreshToken
	return nil
}

func GetValidToken() error {
	if len(access_token) == 0 && len(refresh_token) == 0 {
		// первое получение токенов
		fmt.Println("Токен не найден. Получаем новый...")
		err := getNewToken()
		if err != nil {
			return err
		}
		return nil
	}

	// Проверяем, истёк ли токен
	if time.Now().After(expiresAt) {
		fmt.Println("Токен истёк. Обновляем...")
		if refresh_token == "" {
			// refresh_token нет — получаем новый через браузер
			err := getNewToken()
			if err != nil {
				return err
			}
		} else {
			// Обновляем через refresh_token
			err := refreshAccessToken()
			if err != nil {
				fmt.Println("Не удалось обновить через refresh_token. Получаем новый...")
				err = getNewToken()
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
