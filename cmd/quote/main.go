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
	Rabbit     *amqp.Channel
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
	log.Printf("PostgreSQL connection string: %s", dbConString)
	dbConn, err := sql.Open("postgres", dbConString)
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer dbConn.Close()
	log.Println("Connected to PostgreSQL")
	app := &App{
		DB:         database.New(dbConn),
		Rabbit:     nil,
		TokenCache: &TokenCache{},
		OwnerID:    ownerID,
	}
	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	log.Printf("RabbitMQ connection string: %s", rabbitConString)
	err = app.setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatal("Error setting up RabbitMQ:", err)
	}
	log.Println("Connected to RabbitMQ")

	log.Println("Quote service starting up...")

	log.Println("Quote service is running. Waiting for events...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("\nReceived shutdown signal. Initiating graceful shutdown...")

	if app.Rabbit != nil {
		log.Println("Closing RabbitMQ connections...")
		app.Rabbit.Close()
	}
	if dbConn != nil {
		log.Println("Closing Database connections...")
		dbConn.Close()
	}

	log.Println("Graceful shutdown complete. Exiting.")
}
