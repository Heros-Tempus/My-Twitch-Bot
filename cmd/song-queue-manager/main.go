package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load(".env")

	dbConString := os.Getenv("POSTGRES_CON_STRING")
	log.Printf("PostgreSQL connection string: %s", dbConString)
	dbConn, err := sql.Open("postgres", dbConString)
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer func() {
		log.Println("Closing PostgreSQL connection...")
		dbConn.Close()
	}()
	log.Println("Connected to PostgreSQL")

	service := NewService(dbConn)

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	log.Printf("RabbitMQ connection string: %s", rabbitConString)

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

	rabbitClient := &RabbitClient{ch: ch}
	app := &App{
		rabbit:  rabbitClient,
		service: service,
		isIdle:  true,
	}

	err = setupRabbitMQSubscriptions(con, ch, app)
	if err != nil {
		log.Fatal("Error setting up RabbitMQ subscriptions:", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Bot is running. Waiting for shutdown signal...")

	<-ctx.Done()

	log.Println("Shutdown signal received. Initiating graceful shutdown...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	log.Println("Executing StopAndWipe...")
	if err := app.service.StopAndWipe(shutdownCtx); err != nil {
		log.Printf("Failed to complete StopAndWipe during shutdown: %v", err)
	} else {
		log.Println("StopAndWipe completed successfully.")
	}
}
