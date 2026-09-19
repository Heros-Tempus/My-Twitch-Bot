package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func extractVideoID(input string) string {
	if strings.Contains(input, "v=") {
		parts := strings.Split(input, "v=")
		if len(parts) > 1 {
			return strings.Split(parts[1], "&")[0]
		}
	}
	if strings.Contains(input, "youtu.be/") {
		parts := strings.Split(input, "youtu.be/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "?")[0]
		}
	}
	return input
}

func isYouTubeVideoAvailable(input string) bool {
	videoID := extractVideoID(input)

	u := url.URL{
		Scheme: "https",
		Host:   "www.youtube.com",
		Path:   "/watch",
	}
	q := u.Query()
	q.Set("v", videoID)
	u.RawQuery = q.Encode()
	watchURL := u.String()

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", watchURL, nil)
	if err != nil {
		log.Printf("Failed to create request for %s: %v", watchURL, err)
		return false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Network error checking track %s: %v", watchURL, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("oEmbed rejected %s (Status: %d)", watchURL, resp.StatusCode)
		return false
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body for %s: %v", watchURL, err)
		return false
	}

	html := string(bodyBytes)

	if strings.Contains(html, `"status":"UNPLAYABLE"`) || strings.Contains(html, `"status":"LOGIN_REQUIRED"`) {
		log.Printf("Video %s is flagged as unplayable.", watchURL)
		return false
	}

	if strings.Contains(html, `"reason":"Video unavailable"`) {
		log.Printf("Video %s is flagged as unavailable.", watchURL)
		return false
	}

	return true
}

func buildAttribution(t Track) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s - %s", t.Artist, t.Track))

	if t.SourceMedia != "" {
		parts = append(parts, fmt.Sprintf("Source media: (%s)", t.SourceMedia))
	}
	if t.Album != "" {
		parts = append(parts, fmt.Sprintf("Album: [%s]", t.Album))
	}
	if t.OriginalComposer != "" {
		parts = append(parts, fmt.Sprintf("Composed by: %s", t.OriginalComposer))
	}

	return strings.Join(parts, "\n")
}
