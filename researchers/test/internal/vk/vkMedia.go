package vk

import (
	"fmt"
	"math/rand"
	"time"
)

// Структура для медиа (упрощённая)
type VKMedia struct {
	Type string `json:"type"` // "photo", "video", "audio" и т.д.
	URL  string `json:"url"`  // URL медиа (если доступен)
}

// Структура для фото (упрощённая)
type VKPhoto struct {
	Sizes []VKPhotoSize `json:"sizes"`
}
type VKPhotoSize struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Структура для видео (упрощённая)
type VKVideo struct {
	Player string `json:"player"` // URL плеера
}

// Структура для аудио (упрощённая)
type VKAudio struct {
	URL string `json:"url"`
}

func GetRandMedia(count int) []VKMedia {
	rand.Seed(time.Now().UnixNano())

	media := make([]VKMedia, count)
	for i := 0; i < count; i++ {
		media[i] = generateRandomMedia()
	}

	return media
}

// Генерация случайного медиа объекта
func generateRandomMedia() VKMedia {
	mediaTypes := []string{"photo", "video", "audio"}
	mediaType := mediaTypes[rand.Intn(len(mediaTypes))]

	switch mediaType {
	case "photo":
		return generateRandomPhoto()
	case "video":
		return generateRandomVideo()
	case "audio":
		return generateRandomAudio()
	default:
		return VKMedia{Type: "unknown", URL: ""}
	}
}

// Генерация случайного фото
func generateRandomPhoto() VKMedia {
	photoSizes := []VKPhotoSize{
		{Type: "s", URL: generateRandomImageURL(100, 100)},
		{Type: "m", URL: generateRandomImageURL(320, 240)},
		{Type: "x", URL: generateRandomImageURL(604, 453)},
		{Type: "y", URL: generateRandomImageURL(807, 605)},
		{Type: "z", URL: generateRandomImageURL(1280, 720)},
		{Type: "w", URL: generateRandomImageURL(1920, 1080)},
	}

	// Выбираем несколько случайных размеров
	selectedSizes := make([]VKPhotoSize, rand.Intn(3)+2) // от 2 до 5 размеров
	for i := range selectedSizes {
		selectedSizes[i] = photoSizes[rand.Intn(len(photoSizes))]
	}

	return VKMedia{
		Type: "photo",
		URL:  selectedSizes[len(selectedSizes)-1].URL, // URL самого большого размера
	}
}

// Генерация случайного видео
func generateRandomVideo() VKMedia {
	videoPlayers := []string{
		"https://vk.com/video_ext.php?oid=-123456789&id=123456789&hash=abc123def456",
		"https://vk.com/video_ext.php?oid=987654321&id=123456789&hash=xyz789uvw012",
		"https://vk.com/video_ext.php?oid=-555555555&id=888888888&hash=hash123456",
		"https://vk.com/video_ext.php?oid=111111111&id=222222222&hash=testhash789",
	}

	return VKMedia{
		Type: "video",
		URL:  videoPlayers[rand.Intn(len(videoPlayers))],
	}
}

// Генерация случайного аудио
func generateRandomAudio() VKMedia {
	audioURLs := []string{
		"https://vk.com/music/audio/123456789_abcdef123",
		"https://vk.com/music/audio/987654321_xyz789uvw",
		"https://vk.com/music/audio/555555555_testaudio",
		"https://vk.com/music/audio/111111111_audiotrack",
	}

	return VKMedia{
		Type: "audio",
		URL:  audioURLs[rand.Intn(len(audioURLs))],
	}
}

// Генерация URL случайного изображения (используем сервисы-заглушки)
func generateRandomImageURL(width, height int) string {
	imageServices := []string{
		"https://picsum.photos/%d/%d?random=%d",
		"https://via.placeholder.com/%dx%d?text=Random+Image",
		"https://dummyimage.com/%dx%d/000/fff&text=Random",
		"https://placeimg.com/%d/%d/any",
	}

	service := imageServices[rand.Intn(len(imageServices))]
	return fmt.Sprintf(service, width, height, rand.Intn(1000))
}

// Функция для получения полной информации о фото (если нужно)
func GetPhotoInfo(media VKMedia) VKPhoto {
	if media.Type != "photo" {
		return VKPhoto{}
	}

	// Генерируем полный набор размеров для фото
	sizes := []VKPhotoSize{
		{Type: "s", URL: generateRandomImageURL(100, 100)},
		{Type: "m", URL: generateRandomImageURL(320, 240)},
		{Type: "x", URL: generateRandomImageURL(604, 453)},
		{Type: "y", URL: generateRandomImageURL(807, 605)},
		{Type: "z", URL: generateRandomImageURL(1280, 720)},
		{Type: "w", URL: generateRandomImageURL(1920, 1080)},
	}

	return VKPhoto{Sizes: sizes}
}

// Функция для получения полной информации о видео (если нужно)
func GetVideoInfo(media VKMedia) VKVideo {
	if media.Type != "video" {
		return VKVideo{}
	}

	return VKVideo{Player: media.URL}
}

// Функция для получения полной информации об аудио (если нужно)
func GetAudioInfo(media VKMedia) VKAudio {
	if media.Type != "audio" {
		return VKAudio{}
	}

	return VKAudio{URL: media.URL}
}
