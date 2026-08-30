package twitch

/* import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func SendChatMessage(t Token, broadcasterID, text string) error {
	url := "https://api.twitch.tv/helix/chat/messages"

	payload := map[string]string{
		"broadcaster_id": broadcasterID,  // The channel we are echoing to
		"sender_id":      t.BotAccountID, // The bot sending the message
		"message":        text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal chat payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.Token)
	req.Header.Set("Client-Id", t.ClientID)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send chat message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send message, status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
 */