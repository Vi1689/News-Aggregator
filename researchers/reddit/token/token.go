package token

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	AccessFile = "/usr/local/etc/reddit-researcher/access_data"
	TokenCache = "/usr/local/etc/reddit-researcher/token_cache.json"
	// AccessFile  = "access_data"
	// TokenCache  = "token_cache.json"
	TokenURL    = "https://www.reddit.com/api/v1/access_token"
	CacheBuffer = 60 // запас в секундах
)

type AccessData struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	UserAgent    string
}

type CachedToken struct {
	AccessToken string  `json:"access_token"`
	ExpiresAt   float64 `json:"expires_at"`
}

// ---------- Чтение access_data ----------
func ReadAccessData() (*AccessData, error) {
	content, err := os.ReadFile(AccessFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения %s: %v", AccessFile, err)
	}

	data := &AccessData{}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "client_id":
			data.ClientID = value
		case "client_secret":
			data.ClientSecret = value
		case "username":
			data.Username = value
		case "password":
			data.Password = value
		case "user_agent":
			data.UserAgent = value
		}
	}

	if data.ClientID == "" || data.ClientSecret == "" ||
		data.Username == "" || data.Password == "" || data.UserAgent == "" {
		return nil, errors.New("не все обязательные поля указаны в access_data")
	}
	return data, nil
}

// ---------- Работа с кэшем ----------
func LoadCachedToken() (string, bool) {
	file, err := os.ReadFile(TokenCache)
	if err != nil {
		return "", false
	}
	var t CachedToken
	if err := json.Unmarshal(file, &t); err != nil {
		return "", false
	}
	if time.Now().Unix() < int64(t.ExpiresAt) {
		return t.AccessToken, true
	}
	return "", false
}

func SaveToken(token string, expiresIn int) error {
	data := CachedToken{
		AccessToken: token,
		ExpiresAt:   float64(time.Now().Unix() + int64(expiresIn-CacheBuffer)),
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(TokenCache, jsonData, 0644)
}

// ---------- Запрос нового токена ----------
func RequestNewToken(creds *AccessData) (string, error) {
	client := &http.Client{}

	form := url.Values{}
	form.Add("grant_type", "password")
	form.Add("username", creds.Username)
	form.Add("password", creds.Password)

	req, err := http.NewRequest("POST", TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(creds.ClientID, creds.ClientSecret)
	req.Header.Set("User-Agent", creds.UserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка запроса токена: %s\n%s", resp.Status, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.AccessToken == "" {
		return "", errors.New("не удалось получить access_token")
	}

	if err := SaveToken(result.AccessToken, result.ExpiresIn); err != nil {
		fmt.Println("⚠️ Ошибка сохранения токена:", err)
	}

	return result.AccessToken, nil
}

// ---------- Публичная функция ----------
func GetAccessToken() (string, error) {
	if token, ok := LoadCachedToken(); ok {
		fmt.Println("✅ Используем кэшированный токен")
		return token, nil
	}

	fmt.Println("🔄 Запрашиваем новый токен...")
	creds, err := ReadAccessData()
	if err != nil {
		return "", err
	}
	return RequestNewToken(creds)
}
