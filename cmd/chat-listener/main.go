package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

type token struct {
	BotAccountID string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}

type tokenContainer struct {
	mu            sync.RWMutex
	hasValidToken bool
	isInvalidated bool
	token         token
}

func (t *tokenContainer) Get() (token, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.token, t.hasValidToken
}

func (t *tokenContainer) Invalidate() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hasValidToken = false
	t.isInvalidated = true
}

func (t *tokenContainer) UpdateIfNewer(msg token) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isInvalidated {
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
	firstTokenOnce.Do(func() { close(firstTokenReceived) })

	if msg.Token == "" {
		tokenStore.Invalidate()
		log.Printf("Receieved error signal from Auth, shutting down")
		return pubsub.AckTypeAck
	}

	updated := tokenStore.UpdateIfNewer(msg)
	if !updated {
		log.Printf("Received token older than current token")
	} else {
		log.Printf("OAuth refreshed for bot account %s", msg.BotAccountID)
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

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	con, ch, err := setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatalf("Error setting up RabbitMQ: %v", err)
	}
	defer con.Close()
	defer ch.Close()
	fmt.Println("Waiting for first token...")
	<-firstTokenReceived
	fmt.Println("First Token Received.")

}
