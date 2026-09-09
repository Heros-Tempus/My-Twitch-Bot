package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
)

func refreshOath(token models.OAuthToken) (models.OAuthToken, error) {
	endpoint := "https://id.twitch.tv/oauth2/token"

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", token.Refresh)
	data.Set("user_id", token.OwnerID)
	data.Set("client_id", token.ClientID)
	data.Set("client_secret", token.ClientSecret)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return token, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return token, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return token, fmt.Errorf("twitch API returned unexpected status code: %d", resp.StatusCode)
	}

	var tokenResp TwitchTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return token, fmt.Errorf("failed to decode response: %w", err)
	}

	token.Token = tokenResp.AccessToken
	token.Refresh = tokenResp.RefreshToken
	token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	log.Printf("Successfully refreshed Oauth. Token active for another %v", time.Until(token.ExpiresAt).Round(time.Second))
	return token, nil
}
