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
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/coder/websocket"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	currentConnCancel context.CancelFunc
	connCancelMu      sync.Mutex
)

type wsEnvelope struct {
	Metadata struct {
		MessageType string `json:"message_type"`
	} `json:"metadata"`
	Payload json.RawMessage `json:"payload"`
}

type sessionWelcome struct {
	Session struct {
		ID                      string `json:"id"`
		KeepaliveTimeoutSeconds int    `json:"keepalive_timeout_seconds"`
	} `json:"session"`
}

type sessionReconnect struct {
	Session struct {
		ID           string `json:"id"`
		ReconnectURL string `json:"reconnect_url"`
	} `json:"session"`
}

type chatNotification struct {
	Event struct {
		BroadcasterUserName string `json:"broadcaster_user_name"`
		ChatterUserName     string `json:"chatter_user_name"`
		Message             struct {
			Text string `json:"text"`
		} `json:"message"`
	} `json:"event"`
}

type token struct {
	BotAccountID string
	OwnerID      string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}

type tokenContainer struct {
	mu            sync.RWMutex
	hasValidToken bool
	isRevoked     bool
	token         token
}

func (t *tokenContainer) Get() (token, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.token, t.hasValidToken
}

func (t *tokenContainer) Revoke() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hasValidToken = false
	t.isRevoked = true
}

func (t *tokenContainer) InvalidateForRefresh() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.isRevoked {
		return false
	}
	if t.hasValidToken {
		t.hasValidToken = false
		return true
	}
	return false
}

func (t *tokenContainer) UpdateIfNewer(msg token) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isRevoked {
		return false
	}

	if t.hasValidToken && t.token.ExpiresAt.After(msg.ExpiresAt) {
		return false
	}

	t.token = msg
	t.hasValidToken = true
	return true
}

var tokenStore = &tokenContainer{}
var firstTokenOnce sync.Once
var firstTokenReceived = make(chan struct{})

func GetValidToken() (token, bool) {
	return tokenStore.Get()
}

func getOAuth(msg token) pubsub.AckType {
	if msg.Token == "" {
		tokenStore.Revoke()
		log.Println("Received error signal from Auth, shutting down")
		return pubsub.AckTypeAck
	}

	updated := tokenStore.UpdateIfNewer(msg)
	if !updated {
		log.Println("Received token older than current token")
	} else {
		log.Printf("OAuth refreshed for bot account %s", msg.BotAccountID)

		isFirstToken := false
		firstTokenOnce.Do(func() {
			close(firstTokenReceived)
			isFirstToken = true
		})

		if !isFirstToken {
			connCancelMu.Lock()
			if currentConnCancel != nil {
				log.Println("Token rotated. Signaling WebSocket to reconnect...")
				currentConnCancel()
			}
			connCancelMu.Unlock()
		}
	}

	return pubsub.AckTypeAck
}

func setupRabbitMQ(uri string) (*amqp.Connection, *amqp.Channel, error) {
	con, err := pubsub.ConnectWithBackoff(uri, 5)
	if err != nil {
		return nil, nil, fmt.Errorf("Error connecting to RabbitMQ: %w", err)
	}
	log.Println("Connected to RabbitMQ")

	ch, err := con.Channel()
	if err != nil {
		con.Close()
		return nil, nil, fmt.Errorf("Error opening RabbitMQ channel: %w", err)
	}

	err = pubsub.SubscribeJSON(con, "twitch.auth", "auth.refreshed.listener", "", pubsub.SimpleQueueTypeDurable, getOAuth)
	if err != nil {
		con.Close()
		ch.Close()
		return nil, nil, fmt.Errorf("Error subscribing to RabbitMQ: %w", err)
	}

	return con, ch, nil
}

func main() {
	_ = godotenv.Load(".env")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	con, ch, err := setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatalf("Error setting up RabbitMQ: %v", err)
	}
	defer con.Close()
	defer ch.Close()

	log.Println("Waiting for first token...")
	select {
	case <-firstTokenReceived:
		log.Println("First Token Received.")
	case <-ctx.Done():
		log.Println("Shutting down before token received.")
		return
	}
	log.Println("First Token Received.")

	initialBackoff := 2 * time.Second
	maxBackoff := 2 * time.Minute
	currentBackoff := initialBackoff

	for {
		if ctx.Err() != nil {
			log.Println("Bot is shutting down...")
			break
		}
		tkn, valid := GetValidToken()
		if !valid {
			log.Println("Token is currently invalid or revoked. Waiting before retry...")
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
			}
			continue
		}
		connCtx, cancelConn := context.WithCancel(ctx)

		connCancelMu.Lock()
		currentConnCancel = cancelConn
		connCancelMu.Unlock()

		connectionStartTime := time.Now()

		err := listenToTwitch(connCtx, tkn)

		cancelConn()

		if time.Since(connectionStartTime) > 60*time.Second {
			currentBackoff = initialBackoff
		}
		if ctx.Err() != nil {
			continue
		}

		log.Printf("Twitch listener disconnected: %v", err)
		log.Printf("Reconnecting in %v...", currentBackoff)

		select {
		case <-time.After(currentBackoff):
		case <-ctx.Done():
		}

		currentBackoff *= 2
		if currentBackoff > maxBackoff {
			currentBackoff = maxBackoff
		}
	}
}

func subscribeToChat(sessionID string, t token) error {
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
func listenToTwitch(ctx context.Context, t token) error {
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

		var env wsEnvelope
		if err := json.Unmarshal(message, &env); err != nil {
			log.Printf("Failed to unmarshal outer envelope: %v", err)
			continue
		}

		switch env.Metadata.MessageType {
		case "session_welcome":
			var welcome sessionWelcome
			if err := json.Unmarshal(env.Payload, &welcome); err != nil {
				continue
			}
			log.Printf("Received session_welcome. Session ID: %s", welcome.Session.ID)

			timeoutDuration = time.Duration(welcome.Session.KeepaliveTimeoutSeconds) * time.Second
			log.Printf("Keepalive timeout set to %v", timeoutDuration)

			if !isReconnecting {
				if err := subscribeToChat(welcome.Session.ID, t); err != nil {
					log.Printf("Subscription failed: %v", err)
				}
			} else {
				log.Println("Reconnection complete. Subscriptions migrated automatically.")
				isReconnecting = false
			}

		case "session_keepalive":
			continue

		case "notification":
			var notification chatNotification
			if err := json.Unmarshal(env.Payload, &notification); err != nil {
				log.Printf("Failed to unmarshal notification: %v", err)
				continue
			}

			fmt.Printf("[%s] %s: %s\n",
				notification.Event.BroadcasterUserName,
				notification.Event.ChatterUserName,
				notification.Event.Message.Text,
			)

		case "session_reconnect":
			var reconnect sessionReconnect
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

		default:
			log.Printf("Received unhandled message type: %s", env.Metadata.MessageType)
		}
	}
}
