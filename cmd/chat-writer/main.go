package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")

	app := newApp()

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	if err := app.setupRabbitMQ(rabbitConString); err != nil {
		log.Fatal("Error setting up RabbitMQ:", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Chat Writer is running. Waiting for shutdown signal...")

	<-ctx.Done()
	log.Println("\nShutdown signal received. Initiating graceful shutdown...")

	if app.rabbitChan != nil {
		log.Println("Closing RabbitMQ channel...")
		app.rabbitChan.Close()
	}
	if app.rabbitConn != nil {
		log.Println("Closing RabbitMQ connection...")
		app.rabbitConn.Close()
	}

	log.Println("Writer shut down gracefully.")
}