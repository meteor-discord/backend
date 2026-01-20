package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const (
	screenshotTimeout   = 30 * time.Second
	screenshotWidth     = 1024
	screenshotHeight    = 1024
	screenshotAssetBase = "https://cdn.roxyproxy.de/assets/errors/"
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
