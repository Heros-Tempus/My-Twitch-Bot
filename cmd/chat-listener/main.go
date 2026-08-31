package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/joho/godotenv"
)

var tokenStore = &TokenContainer{}
var firstTokenOnce sync.Once
var firstTokenReceived = make(chan struct{})

func GetValidToken() (Token, bool) {
	return tokenStore.Get()
}

func main() {
	_ = godotenv.Load(".env")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	ch, err := setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatalf("Error setting up RabbitMQ: %v", err)
	}
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

		ConnCancelMu.Lock()
		CurrentConnCancel = cancelConn
		ConnCancelMu.Unlock()

		connectionStartTime := time.Now()
		routeHandler := BuildCommandRouter(ch)
		revocationHandler := func(revocation SubscriptionRevocation) {
			err := pubsub.PublishJSON(ch, "auth.requests", "auth.refresh.request", revocation)
			if err != nil {
				log.Printf("Error publishing revocation: %v", err)
			}
		}
		err := ListenToTwitch(connCtx, tkn, routeHandler, revocationHandler)

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
