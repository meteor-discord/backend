package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	SearchResultTypeUnknown        = 0
	SearchResultTypeSearchResult   = 1
	SearchResultTypeKnowledgeGraph = 2
	SearchResultTypeDoodle         = 3
	SearchResultTypeEntity         = 4
	SearchResultTypeCalculator     = 5
	SearchResultTypeUnitConverter  = 6
	SearchResultTypeDictionary     = 7
	SearchResultTypeMaps           = 8
	SearchResultTypeFunboxCoinFlip = 10
	SearchResultTypeColorPicker    = 11
	SearchResultTypeDataGeneric    = 20
	SearchResultTypeDataFinance    = 21
	SearchResultTypeDataDictionary = 22
	SearchResultTypeDataTranslate  = 23
	SearchResultTypeDataWeather    = 24
	SearchResultTypePivotImages    = 100

	NewsCardTypeArticle    = 1
	NewsCardTypeCollection = 2
)

var ddgClient = &http.Client{
	Timeout: 15 * time.Second,
}

func getDDGHeaders() map[string]string {
	return map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
	}
}

func fetchDDGHTML(targetURL string) (*goquery.Document, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range getDDGHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := ddgClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	return goquery.NewDocumentFromReader(resp.Body)
}

func fetchDDGRaw(targetURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range getDDGHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := ddgClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func writeSearchJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeSearchJSONError(w http.ResponseWriter, status int, message string) {
	writeSearchJSON(w, map[string]interface{}{
		"status":  status,
		"message": message,
	})
}

func SearchDuckDuckGo(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeSearchJSONError(w, StatusError, "missing 'q' query parameter")
		return
	}

	nsfw := r.URL.Query().Get("nsfw") == "true"

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	if !nsfw {
		searchURL += "&kp=1"
	} else {
		searchURL += "&kp=-2"
	}

	doc, err := fetchDDGHTML(searchURL)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to fetch search results")
		return
	}

	var results []map[string]interface{}

	doc.Find("div.result, div.results_links").Each(func(i int, s *goquery.Selection) {
		if len(results) >= 20 {
			return
		}

		link := s.Find("a.result__a").First()
		href, exists := link.Attr("href")
		if !exists || href == "" {
			return
		}

		if strings.Contains(href, "duckduckgo.com/l/") {
			if u, err := url.Parse(href); err == nil {
				if uddg := u.Query().Get("uddg"); uddg != "" {
					href = uddg
				}
			}
		}

		title := strings.TrimSpace(link.Text())
		if title == "" {
			return
		}

		displayLink := s.Find("a.result__url").First().Text()
		displayLink = strings.TrimSpace(displayLink)
		if displayLink == "" {
			if parsedURL, err := url.Parse(href); err == nil {
				displayLink = parsedURL.Host
			}
		}

		snippet := strings.TrimSpace(s.Find("a.result__snippet").First().Text())

		results = append(results, map[string]interface{}{
			"type": SearchResultTypeSearchResult,
			"result": map[string]interface{}{
				"url":          href,
				"title":        title,
				"display_link": displayLink,
				"snippet":      snippet,
			},
		})
	})

	if len(results) == 0 {
		writeSearchJSONError(w, StatusNotFound, "no results found")
		return
	}

	writeSearchJSON(w, map[string]interface{}{
		"status":  StatusSuccess,
		"results": results,
		"doodle":  nil,
	})
}

func SearchDuckDuckGoImages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeSearchJSONError(w, StatusError, "missing 'q' query parameter")
		return
	}

	nsfw := r.URL.Query().Get("nsfw") == "true"

	tokenURL := fmt.Sprintf("https://duckduckgo.com/?q=%s&iax=images&ia=images", url.QueryEscape(query))

	tokenResp, err := fetchDDGRaw(tokenURL)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to get search token")
		return
	}

	vqdPattern := regexp.MustCompile(`vqd=["']?([^"'&]+)`)
	vqdMatch := vqdPattern.FindSubmatch(tokenResp)
	if len(vqdMatch) < 2 {
		vqdPattern2 := regexp.MustCompile(`vqd=(\d+-\d+(?:-\d+)?)`)
		vqdMatch = vqdPattern2.FindSubmatch(tokenResp)
	}

	if len(vqdMatch) < 2 {
		writeSearchJSONError(w, StatusError, "failed to get search token")
		return
	}

	vqd := string(vqdMatch[1])

	safeSearch := "1"
	if nsfw {
		safeSearch = "-1"
	}

	imageURL := fmt.Sprintf(
		"https://duckduckgo.com/i.js?l=us-en&o=json&q=%s&vqd=%s&f=,,,,,&p=%s",
		url.QueryEscape(query), vqd, safeSearch,
	)

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to create request")
		return
	}

	for k, v := range getDDGHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Referer", "https://duckduckgo.com/")

	resp, err := ddgClient.Do(req)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to fetch image results")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to read response")
		return
	}

	var imageData struct {
		Results []struct {
			Title     string `json:"title"`
			Image     string `json:"image"`
			Thumbnail string `json:"thumbnail"`
			URL       string `json:"url"`
			Source    string `json:"source"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &imageData); err != nil {
		writeSearchJSONError(w, StatusError, "failed to parse image results")
		return
	}

	var results []map[string]interface{}
	for _, img := range imageData.Results {
		if len(results) >= 20 {
			break
		}
		results = append(results, map[string]interface{}{
			"title":     img.Title,
			"url":       img.URL,
			"image":     img.Image,
			"thumbnail": img.Thumbnail,
			"source":    img.Source,
			"width":     img.Width,
			"height":    img.Height,
		})
	}

	if len(results) == 0 {
		writeSearchJSONError(w, StatusNotFound, "no image results found")
		return
	}

	writeSearchJSON(w, map[string]interface{}{
		"status":  StatusSuccess,
		"results": results,
	})
}

func SearchMaps(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeSearchJSONError(w, StatusError, "missing 'q' query parameter")
		return
	}

	nominatimURL := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=5&addressdetails=1",
		url.QueryEscape(query),
	)

	req, err := http.NewRequest("GET", nominatimURL, nil)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to create request")
		return
	}

	req.Header.Set("User-Agent", "MeteorDiscordBot/1.0")

	resp, err := ddgClient.Do(req)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to fetch location")
		return
	}
	if resp.StatusCode != http.StatusOK {
		// Ensure the body is closed before returning on non-OK responses.
		resp.Body.Close()
		writeSearchJSONError(w, StatusError, fmt.Sprintf("failed to fetch location: status code %d", resp.StatusCode))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to read response")
		return
	}

	var locations []struct {
		PlaceID     int    `json:"place_id"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
		Type        string `json:"type"`
		Class       string `json:"class"`
		Address     struct {
			Road     string `json:"road"`
			City     string `json:"city"`
			Town     string `json:"town"`
			Village  string `json:"village"`
			State    string `json:"state"`
			Country  string `json:"country"`
			Postcode string `json:"postcode"`
		} `json:"address"`
	}

	if err := json.Unmarshal(body, &locations); err != nil {
		writeSearchJSONError(w, StatusError, "failed to parse location data")
		return
	}

	if len(locations) == 0 {
		writeSearchJSONError(w, StatusNotFound, "location not found")
		return
	}

	loc := locations[0]

	mapURL := fmt.Sprintf(
		"https://staticmap.openstreetmap.de/staticmap.php?center=%s,%s&zoom=14&size=800x400&maptype=mapnik&markers=%s,%s,red-pushpin",
		loc.Lat, loc.Lon, loc.Lat, loc.Lon,
	)

	city := loc.Address.City
	if city == "" {
		city = loc.Address.Town
	}
	if city == "" {
		city = loc.Address.Village
	}

	place := map[string]interface{}{
		"title": loc.DisplayName,
		"address": map[string]interface{}{
			"full":     loc.DisplayName,
			"city":     city,
			"state":    loc.Address.State,
			"country":  loc.Address.Country,
			"postcode": loc.Address.Postcode,
		},
		"coordinates": map[string]interface{}{
			"lat": loc.Lat,
			"lon": loc.Lon,
		},
		"url":          fmt.Sprintf("https://www.openstreetmap.org/?mlat=%s&mlon=%s#map=15/%s/%s", loc.Lat, loc.Lon, loc.Lat, loc.Lon),
		"display_type": loc.Type,
		"style": map[string]interface{}{
			"color": "#4285F4",
			"icon": map[string]interface{}{
				"url": "https://maps.gstatic.com/mapfiles/place_api/icons/v1/png_71/geocode-71.png",
			},
		},
	}

	var places []map[string]interface{}
	if len(locations) > 1 {
		for _, l := range locations {
			lCity := l.Address.City
			if lCity == "" {
				lCity = l.Address.Town
			}
			if lCity == "" {
				lCity = l.Address.Village
			}

			places = append(places, map[string]interface{}{
				"place": map[string]interface{}{
					"name":    l.DisplayName,
					"address": l.DisplayName,
					"city":    lCity,
					"lat":     l.Lat,
					"lon":     l.Lon,
				},
			})
		}
	}

	response := map[string]interface{}{
		"status": StatusSuccess,
		"assets": map[string]interface{}{
			"map": mapURL,
		},
		"place": place,
	}

	if len(places) > 1 {
		response["places"] = places
	}

	writeSearchJSON(w, response)
}

func SearchMapsSupplemental(w http.ResponseWriter, r *http.Request) {
	writeSearchJSON(w, map[string]interface{}{
		"status":  StatusSuccess,
		"message": "supplemental data not available",
	})
}

func SearchNews(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		query = "top stories"
	}

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s+news&kl=us-en", url.QueryEscape(query))

	doc, err := fetchDDGHTML(searchURL)
	if err != nil {
		writeSearchJSONError(w, StatusError, "failed to fetch news results")
		return
	}

	var cards []map[string]interface{}

	doc.Find("div.result, div.results_links").Each(func(i int, s *goquery.Selection) {
		if len(cards) >= 15 {
			return
		}

		link := s.Find("a.result__a").First()
		href, exists := link.Attr("href")
		if !exists || href == "" {
			return
		}

		if strings.Contains(href, "duckduckgo.com/l/") {
			if u, err := url.Parse(href); err == nil {
				if uddg := u.Query().Get("uddg"); uddg != "" {
					href = uddg
				}
			}
		}

		title := strings.TrimSpace(link.Text())
		if title == "" {
			return
		}

		snippet := strings.TrimSpace(s.Find("a.result__snippet").First().Text())

		displayURL := strings.TrimSpace(s.Find("a.result__url").First().Text())

		publisher := "News"
		if displayURL != "" {
			if u, err := url.Parse("https://" + displayURL); err == nil {
				publisher = u.Host
			} else {
				publisher = displayURL
			}
		}

		cards = append(cards, map[string]interface{}{
			"type":  NewsCardTypeArticle,
			"title": title,
			"url":   href,
			"publisher": map[string]interface{}{
				"name": publisher,
				"icon": fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=64", publisher),
			},
			"description": snippet,
		})
	})

	if len(cards) == 0 {
		writeSearchJSONError(w, StatusNotFound, "no news results found")
		return
	}

	writeSearchJSON(w, map[string]interface{}{
		"status": StatusSuccess,
		"cards":  cards,
	})
}

func SearchNewsSupplemental(w http.ResponseWriter, r *http.Request) {
	writeSearchJSON(w, map[string]interface{}{
		"status":  StatusSuccess,
		"message": "supplemental data not available",
	})
}
