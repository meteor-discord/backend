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

		// Google routes
		r.Post("/google/translate/text", handleNotImplemented)
		r.Get("/google/vision/labels", handleNotImplemented)
		r.Post("/google/vision/ocr", handleNotImplemented)
		r.Get("/google/vision/safety", handleNotImplemented)

		// Omni routes
		r.Get("/omni/anime", handleNotImplemented)
		r.Get("/omni/anime-supplemental", handleNotImplemented)
		r.Get("/omni/manga", handleNotImplemented)
		r.Get("/omni/movie", handleNotImplemented)

		// Search routes
		r.Get("/search/duckduckgo", handler.SearchDuckDuckGo)
		r.Get("/search/duckduckgo-images", handler.SearchDuckDuckGoImages)
		r.Get("/search/google-maps", handler.SearchMaps)
		r.Get("/search/google-maps-supplemental", handler.SearchMapsSupplemental)
		r.Get("/search/google-news", handler.SearchNews)
		r.Get("/search/google-news-supplemental", handler.SearchNewsSupplemental)
		r.Get("/search/lyrics", handler.SearchLyrics)
		r.Get("/search/quora", handleNotImplemented)
		r.Get("/search/quora-result", handleNotImplemented)
		r.Get("/search/reverse-image", handleNotImplemented)
		r.Get("/search/booru", handleNotImplemented)
		r.Get("/search/urbandictionary", handler.SearchUrbanDictionary)
		r.Get("/search/wikihow", handleNotImplemented)
		r.Get("/search/wolfram-alpha", handleNotImplemented)
		r.Get("/search/wolfram-supplemental", handleNotImplemented)
		r.Get("/search/youtube", handleNotImplemented)

		// TTS routes
		r.Get("/tts/imtranslator", handleNotImplemented)
		r.Get("/tts/moonbase", handleNotImplemented)
		r.Get("/tts/playht", handleNotImplemented)
		r.Get("/tts/tiktok", handleNotImplemented)

		// Utils routes
		r.Get("/utils/dictionary", handleNotImplemented)
		r.Get("/utils/emojipedia", handleNotImplemented)
		r.Get("/utils/emoji-search", handleNotImplemented)
		r.Get("/utils/garfield", handleNotImplemented)
		r.Get("/utils/otter", handleNotImplemented)
		r.Get("/utils/perspective", handleNotImplemented)
		r.Get("/utils/unicode-metadata", handleNotImplemented)
		r.Get("/utils/webshot", handler.Webshot)

		// LLM routes
		r.Get("/llm/_private:bard", handleNotImplemented)
		r.Get("/parrot/google:gemini", handleNotImplemented)
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
