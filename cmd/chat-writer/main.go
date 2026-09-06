package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
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

	if err := pubsub.DeclareExchange(ch, "twitch", "topic"); err != nil {
		log.Fatal("Error declaring Twitch exchange:", err)
	}

	app := newApp()
	err = pubsub.SubscribeJSON(con, "twitch", "auth.refreshed.writer", "auth.refreshed.writer", pubsub.SimpleQueueTypeDurable, app.handleToken)
	if err != nil {
		log.Fatal("Error subscribing to token messages:", err)
	}
	err = pubsub.SubscribeJSON(con, "twitch", "chat-writer.outgoing", "twitch.chat.send", pubsub.SimpleQueueTypeDurable, app.handleChat)
	if err != nil {
		log.Fatal("Error subscribing to writer messages:", err)
	}

	<-ctx.Done()
	log.Println("Writer shut down gracefully.")
}
