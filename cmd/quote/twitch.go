package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TwitchChannelResponse struct {
	Data []struct {
		GameName string `json:"game_name"`
	} `json:"data"`
}

func GetCurrentGame(ownerID, clientID, accessToken string) (string, error) {
	url := fmt.Sprintf("https://api.twitch.tv/helix/channels?broadcaster_id=%s", ownerID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Client-Id", clientID)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("twitch api returned status: %d", resp.StatusCode)
	}

	var result TwitchChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 || result.Data[0].GameName == "" {
		return "Just Chatting", nil
	}

	return result.Data[0].GameName, nil
}
