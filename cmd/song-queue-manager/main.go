package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	app := &App{
		service: service,
		isIdle:  true,
	}

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	err = app.setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatal("Error setting up RabbitMQ subscriptions:", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Bot is running. Waiting for shutdown signal...")

	<-ctx.Done()

	log.Println("Shutdown signal received. Initiating graceful shutdown...")

	if app.rabbitChan != nil {
		log.Println("Closing RabbitMQ channel...")
		app.rabbitChan.Close()
	}
	if app.rabbitConn != nil {
		log.Println("Closing RabbitMQ connection...")
		app.rabbitConn.Close()
	}
	
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	log.Println("Executing StopAndWipe...")
	if err := app.service.StopAndWipe(shutdownCtx); err != nil {
		log.Printf("Failed to complete StopAndWipe during shutdown: %v", err)
	} else {
		log.Println("StopAndWipe completed successfully.")
	}
}
