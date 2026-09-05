package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

func SubscribeToChat(sessionID string, t Token) error {
	url := "https://api.twitch.tv/helix/eventsub/subscriptions"

	payload := map[string]interface{}{
		"type":    "channel.chat.message",
		"version": "1",
		"condition": map[string]string{
			"broadcaster_user_id": t.OwnerID,      // channel to read from
			"user_id":             t.BotAccountID, // user reading the channel
		},
		"transport": map[string]interface{}{
			"method":     "websocket",
			"session_id": sessionID,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.Token)
	req.Header.Set("Client-Id", t.ClientID)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send subscription request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to subscribe, status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Println("Successfully subscribed to channel.chat.message")
	return nil
}

func ListenToTwitch(ctx context.Context, t Token, onMessage func(string, string), onRevocation func(SubscriptionRevocation)) error {
	wsURL := "wss://eventsub.wss.twitch.tv/ws"

	log.Printf("Connecting to Twitch EventSub at %s...", wsURL)
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to dial Twitch EventSub: %w", err)
	}
	defer func() {
		if c != nil {
			c.Close(websocket.StatusNormalClosure, "client disconnecting")
		}
	}()

	log.Println("WebSocket connected, waiting for messages...")

	isReconnecting := false

	timeoutDuration := 60 * time.Second

	for {
		readCtx, cancel := context.WithTimeout(ctx, timeoutDuration+(2*time.Second))

		_, message, err := c.Read(readCtx)
		cancel()

		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("keepalive timeout exceeded, connection is dead")
			}
			return fmt.Errorf("websocket read error: %w", err)
		}

		var env WSEnvelope
		if err := json.Unmarshal(message, &env); err != nil {
			log.Printf("Failed to unmarshal outer envelope: %v", err)
			continue
		}

		switch env.Metadata.MessageType {
		case "session_welcome":
			var welcome SessionWelcome
			if err := json.Unmarshal(env.Payload, &welcome); err != nil {
				continue
			}
			log.Printf("Received session_welcome. Session ID: %s", welcome.Session.ID)

			timeoutDuration = time.Duration(welcome.Session.KeepaliveTimeoutSeconds) * time.Second
			log.Printf("Keepalive timeout set to %v", timeoutDuration)

			if !isReconnecting {
				if err := SubscribeToChat(welcome.Session.ID, t); err != nil {
					log.Printf("Subscription failed: %v", err)
				}
			} else {
				log.Println("Reconnection complete. Subscriptions migrated automatically.")
				isReconnecting = false
			}

		case "session_keepalive":
			continue

		case "notification":
			var notification ChatNotification
			if err := json.Unmarshal(env.Payload, &notification); err != nil {
				log.Printf("Failed to unmarshal notification: %v", err)
				continue
			}
			if notification.Event.ChatterUserID == t.BotAccountID {
				continue
			}

			/* echoText := fmt.Sprintf("Echoing @%s: %s",
				notification.Event.ChatterUserName,
				notification.Event.Message.Text,
			)

			log.Printf("Sending echo: %s", echoText)

			err := twitch.SendChatMessage(t, notification.Event.BroadcasterUserID, echoText)
			if err != nil {
				log.Printf("Error sending echo to Twitch: %v", err)
			} */

			onMessage(notification.Event.Message.Text, notification.Event.ChatterUserName)

		case "session_reconnect":
			var reconnect SessionReconnect
			if err := json.Unmarshal(env.Payload, &reconnect); err != nil {
				log.Printf("Failed to unmarshal session_reconnect: %v", err)
				continue
			}

			newURL := reconnect.Session.ReconnectURL
			log.Printf("Twitch requested reconnect. Connecting to: %s", newURL)

			newConn, _, err := websocket.Dial(ctx, newURL, nil)
			if err != nil {
				return fmt.Errorf("failed to dial reconnect url: %w", err)
			}

			c.Close(websocket.StatusNormalClosure, "reconnecting to new edge server")

			c = newConn
			isReconnecting = true

		case "revocation":
			var revocation SubscriptionRevocation
			if err := json.Unmarshal(env.Payload, &revocation); err != nil {
				log.Printf("Failed to unmarshal revocation: %v", err)
				continue
			}
			log.Printf("Subscription revoked. Type: %s, Status: %s", revocation.Subscription.Type, revocation.Subscription.Status)
			if tokenStore.InvalidateForRefresh() {
				onRevocation(revocation)
			}
			return fmt.Errorf("subscription revoked (status: %s)", revocation.Subscription.Status)
		default:
			log.Printf("Received unhandled message type: %s", env.Metadata.MessageType)
		}
	}
}
