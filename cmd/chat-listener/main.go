package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := newApp()
	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	if err := app.setupRabbitMQ(rabbitConString); err != nil {
		log.Fatalf("Error setting up RabbitMQ: %v", err)
	}

	log.Println("Waiting for first token...")
	select {
	case <-app.firstTokenReceived:
		log.Println("First Token Received.")
	case <-ctx.Done():
		log.Println("Shutting down before token received.")
		cleanupRabbitMQ(app)
		return
	}

	initialBackoff := 2 * time.Second
	maxBackoff := 2 * time.Minute
	currentBackoff := initialBackoff

	for {
		if ctx.Err() != nil {
			log.Println("Bot is shutting down...")
			break
		}
		
		tkn, valid := app.getToken()
		if !valid {
			log.Println("Token is currently invalid or revoked. Waiting before retry...")
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
			}
			continue
		}
		
		connCtx, cancelConn := context.WithCancel(ctx)
		app.setConnectionCancel(cancelConn)

		connectionStartTime := time.Now()
		
		err := ListenToTwitch(connCtx, tkn, app.RouteCommand, app.HandleRevocation, app.invalidateForRefresh)

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

	cleanupRabbitMQ(app)
	log.Println("Graceful shutdown complete.")
}

func cleanupRabbitMQ(app *App) {
	if app.rabbitChan != nil {
		log.Println("Closing RabbitMQ channel...")
		app.rabbitChan.Close()
	}
	if app.rabbitConn != nil {
		log.Println("Closing RabbitMQ connection...")
		app.rabbitConn.Close()
	}
}