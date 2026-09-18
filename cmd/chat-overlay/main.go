package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	
	ownerID := os.Getenv("OWNER_ID")
	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	
	log.Println("Booting chat-overlay service...")
	app := NewApp()

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

	if err := app.setupRabbitMQ(rabbitConString); err != nil {
		log.Fatalf("Error setting up RabbitMQ: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Waiting for initial OAuth token from auth service...")

	select {
	case <-app.TokenReady:
		log.Println("Token received. Ready to process chat.")
	case <-ctx.Done():
		log.Println("Shutdown signal received during boot. Exiting.")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		if app.rabbitChan != nil {
			app.rabbitChan.Close()
		}
		if app.rabbitConn != nil {
			app.rabbitConn.Close()
		}
		return
	}

	if err := app.subscribeToChat(); err != nil {
		log.Fatalf("Failed to subscribe to chat: %v", err)
	}

	<-ctx.Done()
	log.Println("\nShutdown signal received. Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	if app.rabbitChan != nil {
		log.Println("Closing RabbitMQ channel...")
		app.rabbitChan.Close()
	}
	if app.rabbitConn != nil {
		log.Println("Closing RabbitMQ connection...")
		app.rabbitConn.Close()
	}

	log.Println("Graceful shutdown complete.")
}