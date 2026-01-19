package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// LyricsProviderLRCLIB is the provider ID for LRCLIB lyrics service
const LyricsProviderLRCLIB = 3

// httpClient is a shared HTTP client with timeout for external API calls
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// ApiResponse represents the standard API response format
type ApiResponse struct {
	Timings  string      `json:"timings"`
	Response interface{} `json:"response"`
}

func writeApiResponse(w http.ResponseWriter, data interface{}, startTime time.Time) {
	w.Header().Set("Content-Type", "application/json")
	timings := fmt.Sprintf("%.2f", time.Since(startTime).Seconds())
	response := ApiResponse{
		Timings:  timings,
		Response: map[string]interface{}{"body": data},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode API response: %v", err)
	}
}

func SearchWeather(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	query := r.URL.Query().Get("location")
	if query == "" {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "missing query parameter 'location'",
		}, startTime)
		return
	}

	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1", url.QueryEscape(query))
	resp, err := httpClient.Get(geoURL)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to fetch geolocation",
		}, startTime)
		return
	}
	defer resp.Body.Close()

	var geoResult struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Name      string  `json:"name"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geoResult); err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to parse geolocation response",
		}, startTime)
		return
	}

	if len(geoResult.Results) == 0 {
		writeApiResponse(w, map[string]interface{}{
			"status":  1,
			"message": "location not found",
		}, startTime)
		return
	}

	geo := geoResult.Results[0]
	weatherURL := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code,relative_humidity_2m,apparent_temperature,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,sunrise,sunset&timezone=auto", geo.Latitude, geo.Longitude)
	weatherResp, err := httpClient.Get(weatherURL)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to fetch weather",
		}, startTime)
		return
	}
	defer weatherResp.Body.Close()

	body, err := io.ReadAll(weatherResp.Body)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to read weather response",
		}, startTime)
		return
	}

	var weatherData map[string]interface{}
	if err := json.Unmarshal(body, &weatherData); err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to parse weather response",
		}, startTime)
		return
	}

	// Safe type assertions for weather data
	current, ok := weatherData["current"].(map[string]interface{})
	if !ok {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "unexpected weather response format (invalid current)",
		}, startTime)
		return
	}

	daily, ok := weatherData["daily"].(map[string]interface{})
	if !ok {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "unexpected weather response format (invalid daily)",
		}, startTime)
		return
	}

	dailyTime, ok := daily["time"].([]interface{})
	if !ok {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "unexpected weather response format (invalid daily time)",
		}, startTime)
		return
	}

	dailyWeatherCode, ok := daily["weather_code"].([]interface{})
	if !ok {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "unexpected weather response format (invalid daily weather_code)",
		}, startTime)
		return
	}

	dailyTempMax, ok := daily["temperature_2m_max"].([]interface{})
	if !ok {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "unexpected weather response format (invalid daily temperature_2m_max)",
		}, startTime)
		return
	}

	dailyTempMin, ok := daily["temperature_2m_min"].([]interface{})
	if !ok {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "unexpected weather response format (invalid daily temperature_2m_min)",
		}, startTime)
		return
	}

	dailySunrise, ok := daily["sunrise"].([]interface{})
	if !ok || len(dailySunrise) == 0 {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "unexpected weather response format (invalid daily sunrise)",
		}, startTime)
		return
	}

	dailySunset, ok := daily["sunset"].([]interface{})
	if !ok || len(dailySunset) == 0 {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "unexpected weather response format (invalid daily sunset)",
		}, startTime)
		return
	}

	conditionLabels := map[int]string{
		0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
		45: "Fog", 48: "Depositing rime fog",
		51: "Light drizzle", 53: "Moderate drizzle", 55: "Dense drizzle",
		61: "Slight rain", 63: "Moderate rain", 65: "Heavy rain",
		71: "Slight snow fall", 73: "Moderate snow fall", 75: "Heavy snow fall",
		80: "Slight rain showers", 81: "Moderate rain showers", 82: "Violent rain showers",
		95: "Thunderstorm", 96: "Thunderstorm with hail", 99: "Thunderstorm with heavy hail",
	}

	// Safe type assertion for weather code
	weatherCode := 0
	if wc, ok := current["weather_code"].(float64); ok {
		weatherCode = int(wc)
	}

	// Safe type assertions for sunrise/sunset
	sunriseStr, _ := dailySunrise[0].(string)
	sunsetStr, _ := dailySunset[0].(string)

	writeApiResponse(w, map[string]interface{}{
		"status": 0,
		"result": map[string]interface{}{
			"location": geo.Name,
			"current": map[string]interface{}{
				"icon": map[string]interface{}{
					"id": current["weather_code"],
				},
				"temperature": map[string]interface{}{
					"current":    current["temperature_2m"],
					"feels_like": current["apparent_temperature"],
					"max":        safeIndex(dailyTempMax, 0),
					"min":        safeIndex(dailyTempMin, 0),
				},
				"condition": map[string]interface{}{
					"label": conditionLabels[weatherCode],
				},
				"wind": map[string]interface{}{
					"speed": current["wind_speed_10m"],
				},
				"humidity": current["relative_humidity_2m"],
				"sun": map[string]interface{}{
					"sunrise": parseTime(sunriseStr),
					"sunset":  parseTime(sunsetStr),
				},
			},
			"forecast": buildForecast(dailyTime, dailyWeatherCode, dailyTempMax, dailyTempMin),
			"warnings": []interface{}{},
		},
	}, startTime)
}

func safeIndex(slice []interface{}, index int) interface{} {
	if index >= 0 && index < len(slice) {
		return slice[index]
	}
	return nil
}

func parseTime(s string) int64 {
	t, err := time.Parse("2006-01-02T15:04", s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

func buildForecast(dailyTime []interface{}, dailyWeatherCode []interface{}, dailyTempMax []interface{}, dailyTempMin []interface{}) []interface{} {
	var forecast []interface{}
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)

	for i := 0; i < 7 && i < len(dailyTime); i++ {
		timeStr, ok := dailyTime[i].(string)
		if !ok {
			continue
		}
		t, err := time.Parse("2006-01-02", timeStr)
		if err != nil {
			continue
		}
		dayName := t.Format("Mon")
		if i == 0 {
			dayName = "Today"
		} else if t.Year() == tomorrow.Year() && t.Month() == tomorrow.Month() && t.Day() == tomorrow.Day() {
			dayName = "Tomorrow"
		}
		forecast = append(forecast, map[string]interface{}{
			"day": dayName,
			"icon": map[string]interface{}{
				"id": safeIndex(dailyWeatherCode, i),
			},
			"temperature": map[string]interface{}{
				"max": safeIndex(dailyTempMax, i),
				"min": safeIndex(dailyTempMin, i),
			},
		})
	}
	return forecast
}

func SearchLyrics(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	query := r.URL.Query().Get("q")

	if query == "" {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "missing 'q' query parameter",
		}, startTime)
		return
	}

	// Use LRCLib search API (fuzzy matching)
	apiURL := fmt.Sprintf("https://lrclib.net/api/search?q=%s", url.QueryEscape(query))
	resp, err := httpClient.Get(apiURL)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to fetch lyrics",
		}, startTime)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to read lyrics response",
		}, startTime)
		return
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(body, &results); err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to parse lyrics response",
		}, startTime)
		return
	}

	if len(results) == 0 {
		writeApiResponse(w, map[string]interface{}{
			"status":  1,
			"message": "lyrics not found",
		}, startTime)
		return
	}

	// Use the first result
	result := results[0]

	// Safe type assertion for lyrics
	lyrics, _ := result["plainLyrics"].(string)

	if lyrics == "" {
		writeApiResponse(w, map[string]interface{}{
			"status":  1,
			"message": "lyrics not found",
		}, startTime)
		return
	}

	// Safe type assertions for track info
	trackName := ""
	if v, ok := result["trackName"].(string); ok {
		trackName = v
	} else if v, ok := result["name"].(string); ok {
		trackName = v
	}

	artistName, _ := result["artistName"].(string)
	albumName, _ := result["albumName"].(string)

	writeApiResponse(w, map[string]interface{}{
		"status":          0,
		"lyrics":          lyrics,
		"lyrics_provider": LyricsProviderLRCLIB,
		"track": map[string]interface{}{
			"title":  trackName,
			"artist": artistName,
			"metadata": []map[string]interface{}{
				{"id": "Album", "value": albumName},
			},
		},
	}, startTime)
}

func SearchUrbanDictionary(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	q := r.URL.Query().Get("q")
	if q == "" {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "missing query parameter 'q'",
		}, startTime)
		return
	}

	apiURL := fmt.Sprintf("https://api.urbandictionary.com/v0/define?term=%s", url.QueryEscape(q))
	resp, err := httpClient.Get(apiURL)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to fetch definition",
		}, startTime)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to read response body",
		}, startTime)
		return
	}

	var udData map[string]interface{}
	if err := json.Unmarshal(body, &udData); err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to parse response",
		}, startTime)
		return
	}

	results := []interface{}{}
	if list, ok := udData["list"].([]interface{}); ok {
		for _, item := range list {
			if entry, ok := item.(map[string]interface{}); ok {
				results = append(results, map[string]interface{}{
					"title":       entry["word"],
					"link":        entry["permalink"],
					"description": entry["definition"],
					"author":      entry["author"],
					"date":        entry["written_on"],
					"example":     entry["example"],
					"score": map[string]interface{}{
						"likes":    entry["thumbs_up"],
						"dislikes": entry["thumbs_down"],
					},
				})
			}
		}
	}

	status := 0
	message := ""
	if len(results) == 0 {
		status = 1
		message = "no definitions found"
	}

	writeApiResponse(w, map[string]interface{}{
		"status":  status,
		"message": message,
		"results": results,
	}, startTime)
}
