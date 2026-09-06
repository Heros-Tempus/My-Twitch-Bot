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
	"github.com/joho/godotenv"
)

type Token struct {
	BotAccountID string
	OwnerID      string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}

type ChatMessage struct {
	Message string `json:"message"`
	User    string `json:"user"`
}

type App struct {
	tokenMu     sync.RWMutex
	token       Token
	tokenChange chan struct{}
}

func newApp() *App {
	return &App{
		tokenChange: make(chan struct{}),
	}
}

var ErrTokenExpired = errors.New("token expired")

func main() {

	_ = godotenv.Load(".env")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	con, err := pubsub.ConnectWithBackoff(rabbitConString, 5)
	if err != nil {
		log.Fatal("Error connecting to RabbitMQ:", err)
	}
	defer func() {
		log.Println("Closing RabbitMQ connection...")
		con.Close()
	}()

	ch, err := con.Channel()
	if err != nil {
		log.Fatal("Error opening RabbitMQ channel:", err)
	}
	defer func() {
		log.Println("Closing RabbitMQ channel...")
		ch.Close()
	}()

	if err := pubsub.DeclareExchange(ch, "twitch", "topic"); err != nil {
		log.Fatal("Error declaring Twitch exchange:", err)
	}

	app := newApp()
	err = pubsub.SubscribeJSON(con, "twitch", "auth.refreshed.writer", "auth.refreshed.writer", pubsub.SimpleQueueTypeDurable, app.handleToken)
	if err != nil {
		log.Fatal("Error subscribing to token messages:", err)
	}
	err = pubsub.SubscribeJSON(con, "twitch", "chat-writer.outgoing", "twitch.chat.send", pubsub.SimpleQueueTypeDurable, app.handleChat)
	if err != nil {
		log.Fatal("Error subscribing to writer messages:", err)
	}

	<-ctx.Done()
	log.Println("Writer shut down gracefully.")
}

func SendChatMessage(t Token, text string) error {
	url := "https://api.twitch.tv/helix/chat/messages"

	payload := map[string]string{
		"broadcaster_id": t.OwnerID,
		"sender_id":      t.BotAccountID,
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

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrTokenExpired
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send message, status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (a *App) handleToken(payload Token) pubsub.AckType {
	a.tokenMu.Lock()
	a.token = payload

	close(a.tokenChange)
	a.tokenChange = make(chan struct{})
	a.tokenMu.Unlock()

	return pubsub.AckTypeAck
}

func (a *App) handleChat(payload ChatMessage) pubsub.AckType {
	a.tokenMu.RLock()
	token := a.token
	tokenChange := a.tokenChange
	a.tokenMu.RUnlock()
	log.Printf("Handling chat message from %s: %s", payload.User, payload.Message)
	if token.Token == "" {
		<-tokenChange
		return pubsub.AckTypeNackRequeue
	}

	err := SendChatMessage(token, payload.Message)
	if err == nil {
		return pubsub.AckTypeAck
	}
	log.Printf("Failed to send chat message: %v", err)
	if errors.Is(err, ErrTokenExpired) {
		<-tokenChange
		return pubsub.AckTypeNackRequeue
	}
	return pubsub.AckTypeNackRequeue
}
