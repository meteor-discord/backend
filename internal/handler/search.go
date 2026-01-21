package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	StatusSuccess  = 0
	StatusNotFound = 1
	StatusError    = 2
)

const LyricsProviderLRCLIB = 3

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

type ApiResponse struct {
	Timings  string      `json:"timings"`
	Response interface{} `json:"response"`
}

type apiError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type responseWriter struct {
	w         http.ResponseWriter
	startTime time.Time
}

func newResponseWriter(w http.ResponseWriter, startTime time.Time) *responseWriter {
	return &responseWriter{w: w, startTime: startTime}
}

func (rw *responseWriter) write(data interface{}) {
	rw.w.Header().Set("Content-Type", "application/json")
	timings := fmt.Sprintf("%.2f", time.Since(rw.startTime).Seconds())
	response := ApiResponse{
		Timings:  timings,
		Response: map[string]interface{}{"body": data},
	}
	json.NewEncoder(rw.w).Encode(response)
}

func (rw *responseWriter) writeError(status int, message string) {
	rw.write(apiError{Status: status, Message: message})
}

func fetchJSON(url string, target interface{}) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode failed: %w", err)
	}
	return nil
}

type geoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
}

type geoResponse struct {
	Results []geoLocation `json:"results"`
}

type weatherCurrent struct {
	Temperature      float64 `json:"temperature_2m"`
	ApparentTemp     float64 `json:"apparent_temperature"`
	WeatherCode      int     `json:"weather_code"`
	RelativeHumidity int     `json:"relative_humidity_2m"`
	WindSpeed        float64 `json:"wind_speed_10m"`
}

type weatherDaily struct {
	Time        []string  `json:"time"`
	WeatherCode []int     `json:"weather_code"`
	TempMax     []float64 `json:"temperature_2m_max"`
	TempMin     []float64 `json:"temperature_2m_min"`
	Sunrise     []string  `json:"sunrise"`
	Sunset      []string  `json:"sunset"`
}

type weatherResponse struct {
	Current weatherCurrent `json:"current"`
	Daily   weatherDaily   `json:"daily"`
}

var conditionLabels = map[int]string{
	0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
	45: "Fog", 48: "Depositing rime fog",
	51: "Light drizzle", 53: "Moderate drizzle", 55: "Dense drizzle",
	61: "Slight rain", 63: "Moderate rain", 65: "Heavy rain",
	71: "Slight snow fall", 73: "Moderate snow fall", 75: "Heavy snow fall",
	80: "Slight rain showers", 81: "Moderate rain showers", 82: "Violent rain showers",
	95: "Thunderstorm", 96: "Thunderstorm with hail", 99: "Thunderstorm with heavy hail",
}

func SearchWeather(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	location := r.URL.Query().Get("location")
	if location == "" {
		rw.writeError(StatusError, "missing query parameter 'location'")
		return
	}

	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1", url.QueryEscape(location))
	var geo geoResponse
	if err := fetchJSON(geoURL, &geo); err != nil {
		rw.writeError(StatusError, "failed to fetch geolocation")
		return
	}

	if len(geo.Results) == 0 {
		rw.writeError(StatusNotFound, "location not found")
		return
	}

	loc := geo.Results[0]

	weatherURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code,relative_humidity_2m,apparent_temperature,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,sunrise,sunset&timezone=auto",
		loc.Latitude, loc.Longitude,
	)
	var weather weatherResponse
	if err := fetchJSON(weatherURL, &weather); err != nil {
		rw.writeError(StatusError, "failed to fetch weather")
		return
	}

	rw.write(map[string]interface{}{
		"status": StatusSuccess,
		"result": buildWeatherResult(loc, weather),
	})
}

func buildWeatherResult(loc geoLocation, weather weatherResponse) map[string]interface{} {
	daily := weather.Daily
	current := weather.Current

	return map[string]interface{}{
		"location": loc.Name,
		"current": map[string]interface{}{
			"icon": map[string]interface{}{
				"id": current.WeatherCode,
			},
			"temperature": map[string]interface{}{
				"current":    current.Temperature,
				"feels_like": current.ApparentTemp,
				"max":        safeFloatIndex(daily.TempMax, 0),
				"min":        safeFloatIndex(daily.TempMin, 0),
			},
			"condition": map[string]interface{}{
				"label": conditionLabels[current.WeatherCode],
			},
			"wind": map[string]interface{}{
				"speed": current.WindSpeed,
			},
			"humidity": current.RelativeHumidity,
			"sun": map[string]interface{}{
				"sunrise": parseTime(safeStringIndex(daily.Sunrise, 0)),
				"sunset":  parseTime(safeStringIndex(daily.Sunset, 0)),
			},
		},
		"forecast": buildForecast(daily),
		"warnings": []interface{}{},
	}
}

func buildForecast(daily weatherDaily) []interface{} {
	var forecast []interface{}
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)

	for i := 0; i < 7 && i < len(daily.Time); i++ {
		t, err := time.Parse("2006-01-02", daily.Time[i])
		if err != nil {
			continue
		}

		dayName := t.Format("Mon")
		if i == 0 {
			dayName = "Today"
		} else if sameDay(t, tomorrow) {
			dayName = "Tomorrow"
		}

		forecast = append(forecast, map[string]interface{}{
			"day": dayName,
			"icon": map[string]interface{}{
				"id": safeIntIndex(daily.WeatherCode, i),
			},
			"temperature": map[string]interface{}{
				"max": safeFloatIndex(daily.TempMax, i),
				"min": safeFloatIndex(daily.TempMin, i),
			},
		})
	}
	return forecast
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func safeFloatIndex(slice []float64, index int) *float64 {
	if index >= 0 && index < len(slice) {
		return &slice[index]
	}
	return nil
}

func safeIntIndex(slice []int, index int) *int {
	if index >= 0 && index < len(slice) {
		return &slice[index]
	}
	return nil
}

func safeStringIndex(slice []string, index int) string {
	if index >= 0 && index < len(slice) {
		return slice[index]
	}
	return ""
}

func parseTime(s string) int64 {
	t, err := time.Parse("2006-01-02T15:04", s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

type lyricsResult struct {
	TrackName   string `json:"trackName"`
	Name        string `json:"name"`
	ArtistName  string `json:"artistName"`
	AlbumName   string `json:"albumName"`
	PlainLyrics string `json:"plainLyrics"`
}

func SearchLyrics(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	query := r.URL.Query().Get("q")
	if query == "" {
		rw.writeError(StatusError, "missing 'q' query parameter")
		return
	}

	apiURL := fmt.Sprintf("https://lrclib.net/api/search?q=%s", url.QueryEscape(query))
	var results []lyricsResult
	if err := fetchJSON(apiURL, &results); err != nil {
		rw.writeError(StatusError, "failed to fetch lyrics")
		return
	}

	if len(results) == 0 {
		rw.writeError(StatusNotFound, "lyrics not found")
		return
	}

	result := results[0]
	if result.PlainLyrics == "" {
		rw.writeError(StatusNotFound, "lyrics not found")
		return
	}

	trackName := result.TrackName
	if trackName == "" {
		trackName = result.Name
	}

	rw.write(map[string]interface{}{
		"status":          StatusSuccess,
		"lyrics":          result.PlainLyrics,
		"lyrics_provider": LyricsProviderLRCLIB,
		"track": map[string]interface{}{
			"title":  trackName,
			"artist": result.ArtistName,
			"metadata": []map[string]interface{}{
				{"id": "Album", "value": result.AlbumName},
			},
		},
	})
}

type urbanEntry struct {
	Word       string `json:"word"`
	Permalink  string `json:"permalink"`
	Definition string `json:"definition"`
	Author     string `json:"author"`
	WrittenOn  string `json:"written_on"`
	Example    string `json:"example"`
	ThumbsUp   int    `json:"thumbs_up"`
	ThumbsDown int    `json:"thumbs_down"`
}

type urbanResponse struct {
	List []urbanEntry `json:"list"`
}

func SearchUrbanDictionary(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	query := r.URL.Query().Get("q")
	if query == "" {
		rw.writeError(StatusError, "missing query parameter 'q'")
		return
	}

	apiURL := fmt.Sprintf("https://api.urbandictionary.com/v0/define?term=%s", url.QueryEscape(query))
	var udResponse urbanResponse
	if err := fetchJSON(apiURL, &udResponse); err != nil {
		rw.writeError(StatusError, "failed to fetch definition")
		return
	}

	results := make([]map[string]interface{}, 0, len(udResponse.List))
	for _, entry := range udResponse.List {
		results = append(results, map[string]interface{}{
			"title":       entry.Word,
			"link":        entry.Permalink,
			"description": entry.Definition,
			"author":      entry.Author,
			"date":        entry.WrittenOn,
			"example":     entry.Example,
			"score": map[string]interface{}{
				"likes":    entry.ThumbsUp,
				"dislikes": entry.ThumbsDown,
			},
		})
	}

	status := StatusSuccess
	message := ""
	if len(results) == 0 {
		status = StatusNotFound
		message = "no definitions found"
	}

	rw.write(map[string]interface{}{
		"status":  status,
		"message": message,
		"results": results,
	})
}

type wikihowSearchEntry struct {
	Ns        int    `json:"ns"`
	Title     string `json:"title"`
	PageID    int    `json:"pageid"`
	Size      int    `json:"size"`
	WordCount int    `json:"wordcount"`
	Snippet   string `json:"snippet"`
	Timestamp string `json:"timestamp"`
}

type wikihowResponse struct {
	Query struct {
		Search []wikihowSearchEntry `json:"search"`
	} `json:"query"`
}

func SearchWikihow(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	query := r.URL.Query().Get("q")
	if query == "" {
		rw.writeError(StatusError, "missing 'q' query parameter")
		return
	}

	apiURL := fmt.Sprintf("https://www.wikihow.com/api.php?action=query&format=json&list=search&srsearch=%s", url.QueryEscape(query))
	var whResponse wikihowResponse
	if err := fetchJSON(apiURL, &whResponse); err != nil {
		rw.writeError(StatusError, "failed to fetch wikihow results")
		return
	}

	results := make([]map[string]interface{}, 0, len(whResponse.Query.Search))
	for _, entry := range whResponse.Query.Search {
		// Basic HTML tag stripping for snippet could be improved, but this is a start
		snippet := stripHTML(entry.Snippet)
		results = append(results, map[string]interface{}{
			"title":   entry.Title,
			"url":     fmt.Sprintf("https://www.wikihow.com/%s", strings.ReplaceAll(entry.Title, " ", "-")),
			"snippet": snippet,
		})
	}

	status := StatusSuccess
	if len(results) == 0 {
		status = StatusNotFound
	}

	rw.write(map[string]interface{}{
		"status":  status,
		"results": results,
	})
}

func stripHTML(s string) string {
	// Simple regex to strip HTML tags
	// In a real app, use a proper HTML parser
	// This strips anything between < and >
	var output strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			output.WriteRune(r)
		}
	}
	return output.String()
}

type invidiousVideo struct {
	Title         string `json:"title"`
	VideoID       string `json:"videoId"`
	Author        string `json:"author"`
	LengthSeconds int64  `json:"lengthSeconds"`
	ViewCount     int64  `json:"viewCount"`
	PublishedText string `json:"publishedText"`
}

func SearchYoutube(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	query := r.URL.Query().Get("q")
	if query == "" {
		rw.writeError(StatusError, "missing 'q' query parameter")
		return
	}

	// Using a public Invidious instance
	// Fallback instances could be implemented
	instance := "https://inv.tux.pizza"
	apiURL := fmt.Sprintf("%s/api/v1/search?q=%s&type=video", instance, url.QueryEscape(query))

	var videos []invidiousVideo
	if err := fetchJSON(apiURL, &videos); err != nil {
		rw.writeError(StatusError, "failed to fetch youtube results")
		return
	}

	results := make([]map[string]interface{}, 0, len(videos))
	for _, v := range videos {
		results = append(results, map[string]interface{}{
			"title":    v.Title,
			"url":      fmt.Sprintf("https://www.youtube.com/watch?v=%s", v.VideoID),
			"author":   v.Author,
			"duration": v.LengthSeconds,
			"views":    v.ViewCount,
			"date":     v.PublishedText,
		})
	}

	status := StatusSuccess
	if len(results) == 0 {
		status = StatusNotFound
	}

	rw.write(map[string]interface{}{
		"status":  status,
		"results": results,
	})
}
