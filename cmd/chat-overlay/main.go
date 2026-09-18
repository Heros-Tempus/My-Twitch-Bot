package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Booting chat-overlay service...")
	app := NewApp()

	ownerID := os.Getenv("OWNER_ID")
	rabbitConstring := os.Getenv("RABBIT_CON_STRING")
	fmt.Printf("RabbitMQ URI: %s\n", rabbitConstring)
	log.Println("Fetching 3rd Party Emotes...")
	if err := fetch7TVEmotes(ownerID, app.Cache); err != nil {
		log.Printf("Failed to fetch 7TV emotes: %v", err)
	}
	if err := fetchBTTVEmotes(ownerID, app.Cache); err != nil {
		log.Printf("Failed to fetch BTTV emotes: %v", err)
	}
	if err := fetchFFZEmotes(ownerID, app.Cache); err != nil {
		log.Printf("Failed to fetch FFZ emotes: %v", err)
	}

	conn, err := pubsub.ConnectWithBackoff(rabbitConstring, 5)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ channel: %v", err)
	}
	defer ch.Close()

	if err = pubsub.DeclareExchange(ch, pubsub.ExchangeBot, "topic"); err != nil {
		log.Fatalf("Failed to declare bot exchange: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", app.handleWebSocket)
	mux.HandleFunc("/chat.html", app.serveHTML)
	server := &http.Server{Addr: ":8080", Handler: mux}

	go func() {
		log.Println("Overlay server running on :8080 (HTML at /chat.html)")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	err = pubsub.SubscribeJSON(conn, pubsub.ExchangeBot, pubsub.QueueOverlayOAuth, pubsub.KeyTokenRefreshed, pubsub.SimpleQueueTypeDurable, app.handleAuthUpdate)
	if err != nil {
		log.Fatalf("Failed to subscribe to auth: %v", err)
	}
	err = pubsub.SubscribeJSON(conn, pubsub.ExchangeBot, pubsub.QueueOverlayAlerts, pubsub.KeyTokenFailed, pubsub.SimpleQueueTypeDurable, app.handleAuthFailure)
	if err != nil {
		log.Fatalf("Failed to subscribe to auth failures: %v", err)
	}

	log.Println("Waiting for initial OAuth token from auth service...")

	select {
	case <-app.TokenReady:
		log.Println("Token received. Ready to process chat.")
	case <-ctx.Done():
		log.Println("Shutdown signal received during boot. Exiting.")
		server.Shutdown(context.Background())
		return
	}

	err = pubsub.SubscribeJSON(conn, pubsub.ExchangeBot, pubsub.QueueOverlayMessages, pubsub.KeyChatOverlay, pubsub.SimpleQueueTypeDurable, app.handleIncomingMessage)
	if err != nil {
		log.Fatalf("Failed to subscribe to chat: %v", err)
	}

	<-ctx.Done()
	log.Println("\nShutdown signal received. Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Graceful shutdown complete.")
}
