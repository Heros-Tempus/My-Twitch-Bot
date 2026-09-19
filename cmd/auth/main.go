package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load(".env")

	dbConString := os.Getenv("POSTGRES_CON_STRING")
	db, err := sql.Open("postgres", dbConString)
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer func() {
		log.Println("Closing PostgreSQL connection...")
		db.Close()
	}()
	log.Println("Connected to PostgreSQL")

	app := newApp(database.New(db))

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	if err := app.setupRabbitMQ(rabbitConString); err != nil {
		log.Fatal("Error setting up RabbitMQ:", err)
	}

	botID := os.Getenv("BOT_ID")
	app.loadOAuth(botID)

	if app.oauth.Token == "" {
		log.Fatal("Critical: OAuth initialization failed")
	}

	app.publishOAuth(nil)
	log.Println("OAuth service initialized successfully.")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Auth service is running. Waiting for shutdown signal...")

	// Blocks until ctx is cancelled
	app.run(ctx)

	log.Println("\nShutdown signal received. Initiating graceful shutdown...")

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
