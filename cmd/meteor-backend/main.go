package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/meteor-discord/backend/internal/handler"
	authmw "github.com/meteor-discord/backend/internal/middleware"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}

	log.Println("Starting meteor-backend server...")

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	r.Group(func(r chi.Router) {
		r.Use(authmw.Auth(apiKey))

		r.Get("/google/generativeai/edit-image", handleNotImplemented)
		r.Get("/google/generativeai/gemini", handleNotImplemented)
		r.Get("/google/generativeai/imagen", handleNotImplemented)
		r.Post("/google/perspective/analyze", handleNotImplemented)
		r.Post("/google/speech/recognize", handleNotImplemented)
		r.Post("/google/speech/multirecognize", handleNotImplemented)
		r.Post("/google/translate/text", handleNotImplemented)
		r.Post("/google/translate/multi", handleNotImplemented)
		r.Get("/google/vision/colors", handleNotImplemented)
		r.Get("/google/vision/faces", handleNotImplemented)
		r.Get("/google/vision/labels", handleNotImplemented)
		r.Post("/google/vision/ocr", handleNotImplemented)
		r.Get("/google/vision/safety", handleNotImplemented)
		r.Get("/google/vision/webdetection", handleNotImplemented)

		r.Get("/image/inhouse/pride", handleNotImplemented)
		r.Get("/image/deepai/deepdream", handleNotImplemented)
		r.Get("/image/deepai/imageedit", handleNotImplemented)
		r.Get("/image/deepai/superresolution", handleNotImplemented)
		r.Get("/image/deepai/text2image", handleNotImplemented)
		r.Get("/image/deepai/waifu2x", handleNotImplemented)
		r.Get("/image/emogen/generate", handleNotImplemented)

		r.Get("/photofunia/retro-wave", handleNotImplemented)
		r.Get("/photofunia/yacht", handleNotImplemented)

		r.Get("/search/bing", handleNotImplemented)
		r.Get("/search/bing-images", handleNotImplemented)
		r.Get("/search/duckduckgo", handleNotImplemented)
		r.Get("/search/google", handleNotImplemented)
		r.Get("/search/google-images", handleNotImplemented)
		r.Get("/search/google-maps", handleNotImplemented)
		r.Get("/search/google-maps-supplemental", handleNotImplemented)
		r.Get("/search/google-news", handleNotImplemented)
		r.Get("/search/google-news-supplemental", handleNotImplemented)
		r.Get("/search/lyrics", handler.SearchLyrics)
		r.Get("/search/quora", handleNotImplemented)
		r.Get("/search/quora-result", handleNotImplemented)
		r.Get("/search/reddit", handleNotImplemented)
		r.Get("/search/reverse-image", handleNotImplemented)
		r.Get("/search/booru", handleNotImplemented)
		r.Get("/search/urbandictionary", handler.SearchUrbanDictionary)
		r.Get("/search/weather", handler.SearchWeather)
		r.Get("/search/wikihow", handleNotImplemented)
		r.Get("/search/wolfram-alpha", handleNotImplemented)
		r.Get("/search/wolfram-supplemental", handleNotImplemented)
		r.Get("/search/youtube", handleNotImplemented)

		r.Get("/tts/imtranslator", handleNotImplemented)
		r.Get("/tts/moonbase", handleNotImplemented)
		r.Get("/tts/playht", handleNotImplemented)
		r.Get("/tts/polly", handleNotImplemented)
		r.Get("/tts/sapi4", handleNotImplemented)
		r.Get("/tts/tiktok", handleNotImplemented)

		r.Get("/utils/dictionary-v2", handleNotImplemented)
		r.Get("/utils/emojipedia", handleNotImplemented)
		r.Get("/utils/emoji-search", handleNotImplemented)
		r.Get("/utils/garfield", handleNotImplemented)
		r.Get("/utils/gpt", handleNotImplemented)
		r.Get("/utils/grok", handleNotImplemented)
		r.Get("/utils/inferkit", handleNotImplemented)
		r.Get("/utils/mapkit", handleNotImplemented)
		r.Get("/utils/otter", handleNotImplemented)
		r.Get("/utils/perspective", handleNotImplemented)
		r.Get("/utils/screenshot", handler.Screenshot)
		r.Get("/utils/text-generator", handleNotImplemented)
		r.Get("/utils/unicode-metadata", handleNotImplemented)
		r.Get("/utils/weather", handler.SearchWeather)
		r.Get("/utils/webshot", handler.Webshot)
	})

	port := ":8081"
	log.Printf("Server listening on %s", port)

	go func() {
		if err := http.ListenAndServe(port, r); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	time.Sleep(500 * time.Millisecond)
	log.Println("Server stopped")
}

func handleNotImplemented(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	w.Header().Set("Content-Type", "application/json")
	response := handler.ApiResponse{
		Timings:  fmt.Sprintf("%.2f", time.Since(startTime).Seconds()),
		Response: map[string]interface{}{"body": map[string]interface{}{"status": 2, "message": "not implemented"}},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode not implemented response: %v", err)
	}
}
