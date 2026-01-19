package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

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
	json.NewEncoder(w).Encode(response)
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
	resp, err := http.Get(geoURL)
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
	weatherResp, err := http.Get(weatherURL)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to fetch weather",
		}, startTime)
		return
	}
	defer weatherResp.Body.Close()

	body, _ := io.ReadAll(weatherResp.Body)

	var weatherData map[string]interface{}
	json.Unmarshal(body, &weatherData)

	current := weatherData["current"].(map[string]interface{})
	daily := weatherData["daily"].(map[string]interface{})
	dailyTime := daily["time"].([]interface{})
	dailyWeatherCode := daily["weather_code"].([]interface{})
	dailyTempMax := daily["temperature_2m_max"].([]interface{})
	dailyTempMin := daily["temperature_2m_min"].([]interface{})
	dailySunrise := daily["sunrise"].([]interface{})
	dailySunset := daily["sunset"].([]interface{})

	conditionLabels := map[int]string{
		0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
		45: "Fog", 48: "Depositing rime fog",
		51: "Light drizzle", 53: "Moderate drizzle", 55: "Dense drizzle",
		61: "Slight rain", 63: "Moderate rain", 65: "Heavy rain",
		71: "Slight snow fall", 73: "Moderate snow fall", 75: "Heavy snow fall",
		80: "Slight rain showers", 81: "Moderate rain showers", 82: "Violent rain showers",
		95: "Thunderstorm", 96: "Thunderstorm with hail", 99: "Thunderstorm with heavy hail",
	}

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
					"max":        dailyTempMax[0],
					"min":        dailyTempMin[0],
				},
				"condition": map[string]interface{}{
					"label": conditionLabels[int(current["weather_code"].(float64))],
				},
				"wind": map[string]interface{}{
					"speed": current["wind_speed_10m"],
				},
				"humidity": current["relative_humidity_2m"],
				"sun": map[string]interface{}{
					"sunrise": parseTime(dailySunrise[0].(string)),
					"sunset":  parseTime(dailySunset[0].(string)),
				},
			},
			"forecast": buildForecast(dailyTime, dailyWeatherCode, dailyTempMax, dailyTempMin),
			"warnings": []interface{}{},
		},
	}, startTime)
}

func parseTime(s string) int64 {
	t, _ := time.Parse("2006-01-02T15:04", s)
	return t.UnixMilli()
}

func buildForecast(dailyTime []interface{}, dailyWeatherCode []interface{}, dailyTempMax []interface{}, dailyTempMin []interface{}) []interface{} {
	var forecast []interface{}
	now := time.Now()
	for i := 0; i < 7 && i < len(dailyTime); i++ {
		t, _ := time.Parse("2006-01-02", dailyTime[i].(string))
		dayName := t.Format("Mon")
		if i == 0 {
			dayName = "Today"
		} else if t.Day() == now.Day() {
			dayName = "Tomorrow"
		}
		forecast = append(forecast, map[string]interface{}{
			"day": dayName,
			"icon": map[string]interface{}{
				"id": dailyWeatherCode[i],
			},
			"temperature": map[string]interface{}{
				"max": dailyTempMax[i],
				"min": dailyTempMin[i],
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
	resp, err := http.Get(apiURL)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to fetch lyrics",
		}, startTime)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var results []map[string]interface{}
	json.Unmarshal(body, &results)

	if len(results) == 0 {
		writeApiResponse(w, map[string]interface{}{
			"status":  1,
			"message": "lyrics not found",
		}, startTime)
		return
	}

	// Use the first result
	result := results[0]

	lyrics := ""
	if result["plainLyrics"] != nil {
		lyrics = result["plainLyrics"].(string)
	}

	if lyrics == "" {
		writeApiResponse(w, map[string]interface{}{
			"status":  1,
			"message": "lyrics not found",
		}, startTime)
		return
	}

	trackName := ""
	if result["trackName"] != nil {
		trackName = result["trackName"].(string)
	} else if result["name"] != nil {
		trackName = result["name"].(string)
	}

	artistName := ""
	if result["artistName"] != nil {
		artistName = result["artistName"].(string)
	}

	albumName := ""
	if result["albumName"] != nil {
		albumName = result["albumName"].(string)
	}

	writeApiResponse(w, map[string]interface{}{
		"status":          0,
		"lyrics":          lyrics,
		"lyrics_provider": 3, // LRCLIB
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
	resp, err := http.Get(apiURL)
	if err != nil {
		writeApiResponse(w, map[string]interface{}{
			"status":  2,
			"message": "failed to fetch definition",
		}, startTime)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var udData map[string]interface{}
	json.Unmarshal(body, &udData)

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
