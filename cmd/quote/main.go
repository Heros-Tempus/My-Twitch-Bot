package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	DB         *database.Queries
	RabbitConn *amqp.Connection
	RabbitChann     *amqp.Channel
	TokenCache *TokenCache
	OwnerID    string
}

func main() {
	_ = godotenv.Load(".env")
	ownerID := os.Getenv("OWNER_ID")
	if ownerID == "" {
		log.Fatal("ERROR: OWNER_ID environment variable is required")
	}

	dbConString := os.Getenv("POSTGRES_CON_STRING")
	dbConn, err := sql.Open("postgres", dbConString)
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer dbConn.Close()
	log.Println("Connected to PostgreSQL")

	app := &App{
		DB:         database.New(dbConn),
		TokenCache: &TokenCache{},
		OwnerID:    ownerID,
	}

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	err = app.setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatal("Error setting up RabbitMQ:", err)
	}

	log.Println("Quote service is running. Waiting for events...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	
	log.Println("\nReceived shutdown signal. Initiating graceful shutdown...")
	
	if app.RabbitChann != nil {
		app.RabbitChann.Close()
	}
	if app.RabbitConn != nil {
		app.RabbitConn.Close()
	}

	log.Println("Graceful shutdown complete. Exiting.")
}