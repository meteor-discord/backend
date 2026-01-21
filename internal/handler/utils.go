package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"golang.org/x/text/unicode/runenames"
)

const screenshotAssetBase = "https://bignutty.gitlab.io/webstorage4/v2/assets/screenshot/brand-update-2024/"

const (
	screenshotTimeout = 30 * time.Second

	screenshotWidth  = 1024
	screenshotHeight = 1024
)

var blockedDomains = []string{
	"pornhub.com",
	"xvideos.com",
	"xnxx.com",
	"xhamster.com",
	"redtube.com",
	"youporn.com",
	"tube8.com",
	"spankbang.com",
	"brazzers.com",
	"bangbros.com",
	"guns.lol",
}

type screenshotError struct {
	Error struct {
		ImageURL string `json:"image_url"`
		Message  string `json:"message"`
	} `json:"error"`
}

func isBlockedDomain(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	for _, blocked := range blockedDomains {
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return true
		}
	}
	return false
}

func normalizeURL(rawURL string) string {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "https://" + rawURL
	}
	return rawURL
}

func takeScreenshot(targetURL string) ([]byte, error) {
	l := launcher.New().
		Headless(true).
		Set("disable-gpu").
		Set("no-sandbox").
		Set("disable-dev-shm-usage")

	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}
	defer browser.Close()

	ctx, cancel := context.WithTimeout(context.Background(), screenshotTimeout)
	defer cancel()

	page, err := browser.Context(ctx).Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  screenshotWidth,
		Height: screenshotHeight,
	}); err != nil {
		return nil, fmt.Errorf("failed to set viewport: %w", err)
	}

	if err := page.Navigate(targetURL); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("failed to wait for page load: %w", err)
	}

	// this is to give the javascript time to load
	time.Sleep(1000 * time.Millisecond)

	screenshot, err := page.Screenshot(true, &proto.PageCaptureScreenshot{
		Format:  proto.PageCaptureScreenshotFormatPng,
		Quality: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	return screenshot, nil
}

func Webshot(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		w.Header().Set("Content-Type", "application/json")
		rw := newResponseWriter(w, startTime)
		rw.write(screenshotError{
			Error: struct {
				ImageURL string `json:"image_url"`
				Message  string `json:"message"`
			}{
				ImageURL: screenshotAssetBase + "scr_invalid_url.png",
				Message:  "missing 'url' query parameter",
			},
		})
		return
	}

	targetURL := normalizeURL(rawURL)

	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Host == "" {
		w.Header().Set("Content-Type", "application/json")
		rw := newResponseWriter(w, startTime)
		rw.write(screenshotError{
			Error: struct {
				ImageURL string `json:"image_url"`
				Message  string `json:"message"`
			}{
				ImageURL: screenshotAssetBase + "scr_invalid_url.png",
				Message:  "invalid URL format",
			},
		})
		return
	}

	nsfw := r.URL.Query().Get("nsfw")
	if nsfw != "true" && isBlockedDomain(targetURL) {
		w.Header().Set("Content-Type", "application/json")
		rw := newResponseWriter(w, startTime)
		rw.write(screenshotError{
			Error: struct {
				ImageURL string `json:"image_url"`
				Message  string `json:"message"`
			}{
				ImageURL: screenshotAssetBase + "scr_nsfw.png",
				Message:  "this website is blocked",
			},
		})
		return
	}

	screenshot, err := takeScreenshot(targetURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		rw := newResponseWriter(w, startTime)
		rw.write(screenshotError{
			Error: struct {
				ImageURL string `json:"image_url"`
				Message  string `json:"message"`
			}{
				ImageURL: screenshotAssetBase + "scr_unavailable.png",
				Message:  err.Error(),
			},
		})
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(screenshot)))
	w.Write(screenshot)
}

func Screenshot(w http.ResponseWriter, r *http.Request) {
	Webshot(w, r)
}

func GetGarfield(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	// Garfield started June 19, 1978
	start := time.Date(1978, 6, 19, 0, 0, 0, 0, time.UTC)
	end := time.Now()
	diff := end.Sub(start)

	// Random day
	randomDays := rand.Int63n(int64(diff.Hours() / 24))
	date := start.Add(time.Duration(randomDays) * 24 * time.Hour)
	dateStr := date.Format("2006/01/02")

	urlStr := fmt.Sprintf("https://www.gocomics.com/garfield/%s", dateStr)

	resp, err := httpClient.Get(urlStr)
	if err != nil {
		rw.writeError(StatusError, "failed to fetch garfield page")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		rw.writeError(StatusError, "failed to read garfield page")
		return
	}

	// Regex to find og:image
	re := regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
	matches := re.FindSubmatch(body)

	if len(matches) < 2 {
		rw.writeError(StatusNotFound, "garfield comic not found")
		return
	}

	comicURL := string(matches[1])

	rw.write(map[string]interface{}{
		"date":  date.Format("2006-01-02"),
		"comic": comicURL,
		"link":  urlStr,
	})
}

type redditChildData struct {
	URL string `json:"url"`
}

type redditChild struct {
	Data redditChildData `json:"data"`
}

type redditData struct {
	Children []redditChild `json:"children"`
}

type redditResponse struct {
	Data redditData `json:"data"`
}

func GetOtter(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	req, err := http.NewRequest("GET", "https://www.reddit.com/r/Otters/random.json", nil)
	if err != nil {
		rw.writeError(StatusError, "failed to create request")
		return
	}

	// Reddit requires a User-Agent
	req.Header.Set("User-Agent", "Meteor-Backend/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		rw.writeError(StatusError, "failed to fetch otter")
		return
	}
	defer resp.Body.Close()

	// Reddit random returns an array, where the first element contains the post
	var results []redditResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		rw.writeError(StatusError, "failed to decode reddit response")
		return
	}

	if len(results) == 0 || len(results[0].Data.Children) == 0 {
		rw.writeError(StatusNotFound, "no otter found")
		return
	}

	imageURL := results[0].Data.Children[0].Data.URL

	rw.write(map[string]interface{}{
		"url": imageURL,
	})
}

type dictPhonetic struct {
	Text  string `json:"text"`
	Audio string `json:"audio"`
}

type dictDefinition struct {
	Definition string   `json:"definition"`
	Example    string   `json:"example"`
	Synonyms   []string `json:"synonyms"`
	Antonyms   []string `json:"antonyms"`
}

type dictMeaning struct {
	PartOfSpeech string           `json:"partOfSpeech"`
	Definitions  []dictDefinition `json:"definitions"`
}

type dictEntry struct {
	Word      string         `json:"word"`
	Phonetic  string         `json:"phonetic"`
	Phonetics []dictPhonetic `json:"phonetics"`
	Meanings  []dictMeaning  `json:"meanings"`
	Origin    string         `json:"origin"`
}

func GetDictionary(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	word := r.URL.Query().Get("word")
	if word == "" {
		rw.writeError(StatusError, "missing 'word' query parameter")
		return
	}

	urlStr := fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%s", url.QueryEscape(word))

	// The API returns an array of entries
	var entries []dictEntry

	// Handle 404 specifically as it returns a different JSON structure sometimes
	resp, err := httpClient.Get(urlStr)
	if err != nil {
		rw.writeError(StatusError, "request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		rw.writeError(StatusNotFound, "word not found")
		return
	}

	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		rw.writeError(StatusError, "failed to decode dictionary response")
		return
	}

	rw.write(entries)
}

func GetUnicodeMetadata(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w, time.Now())

	charStr := r.URL.Query().Get("char")
	if charStr == "" {
		rw.writeError(StatusError, "missing 'char' query parameter")
		return
	}

	runes := []rune(charStr)
	if len(runes) == 0 {
		rw.writeError(StatusError, "empty character")
		return
	}

	rChar := runes[0]

	name := runenames.Name(rChar)

	// Basic categorization
	category := "Unknown"
	if unicode.IsControl(rChar) {
		category = "Control"
	} else if unicode.IsDigit(rChar) {
		category = "Digit"
	} else if unicode.IsLetter(rChar) {
		category = "Letter"
	} else if unicode.IsNumber(rChar) {
		category = "Number"
	} else if unicode.IsSpace(rChar) {
		category = "Space"
	} else if unicode.IsSymbol(rChar) {
		category = "Symbol"
	} else if unicode.IsPunct(rChar) {
		category = "Punctuation"
	} else if unicode.IsMark(rChar) {
		category = "Mark"
	}

	rw.write(map[string]interface{}{
		"char":      string(rChar),
		"name":      name,
		"codepoint": fmt.Sprintf("U+%04X", rChar),
		"decimal":   int(rChar),
		"hex":       fmt.Sprintf("%X", rChar),
		"category":  category,
		"html":      fmt.Sprintf("&#%d;", rChar),
	})
}