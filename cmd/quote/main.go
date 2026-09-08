package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
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

func (app *App) setupRabbitMQ(rabbitConString string) error {
	con, err := pubsub.ConnectWithBackoff(rabbitConString, 5)
	if err != nil {
		return fmt.Errorf("Error connecting to RabbitMQ: %w", err)
	}
	log.Println("Connected to RabbitMQ")

	ch, err := con.Channel()
	if err != nil {
		con.Close()
		return fmt.Errorf("Error opening RabbitMQ channel: %w", err)
	}
	app.Rabbit = ch
	err = pubsub.DeclareExchange(ch, "twitch", "topic")
	if err != nil {
		con.Close()
		ch.Close()
		return fmt.Errorf("Error declaring Twitch exchange: %w", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, "twitch", "auth.refreshed.quote", "auth.refreshed.quote")
	if err != nil {
		con.Close()
		ch.Close()
		return fmt.Errorf("Error declaring and binding auth refreshed quote queue: %w", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, "twitch", "twitch.chat.commands.quote", "twitch.chat.commands.quote")
	if err != nil {
		con.Close()
		ch.Close()
		return fmt.Errorf("Error declaring and binding twitch chat commands quote queue: %w", err)
	}
	err = pubsub.SubscribeJSON(con, "twitch", "auth.refreshed.quote", "auth.refreshed.quote", pubsub.SimpleQueueTypeDurable, app.HandleAuthMessage)
	if err != nil {
		con.Close()
		ch.Close()
		return fmt.Errorf("Error subscribing to auth refreshed quote messages: %w", err)
	}
	err = pubsub.SubscribeJSON(con, "twitch", "auth.refreshed.quote", "auth.refreshed.quote", pubsub.SimpleQueueTypeDurable, app.HandleAuthMessage)
	if err != nil {
		con.Close()
		ch.Close()
		return fmt.Errorf("Error subscribing to auth refreshed quote messages: %w", err)
	}
	err = pubsub.SubscribeJSON(con, "twitch", "twitch.chat.commands.quote", "twitch.chat.commands.quote", pubsub.SimpleQueueTypeDurable, app.HandleCommandMessage)
	if err != nil {
		con.Close()
		ch.Close()
		return fmt.Errorf("Error subscribing to quote commands: %w", err)
	}
	return nil
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
