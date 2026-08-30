package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func setupRabbitMQ(uri string) (*amqp.Channel, error) {
	con, err := pubsub.ConnectWithBackoff(uri, 5)
	if err != nil {
		return nil, fmt.Errorf("Error connecting to RabbitMQ: %w", err)
	}
	log.Println("Connected to RabbitMQ")

	ch, err := con.Channel()
	if err != nil {
		con.Close()
		return nil, fmt.Errorf("Error opening RabbitMQ channel: %w", err)
	}

	err = pubsub.SubscribeJSON(con, "twitch", "auth", "auth.refreshed.listener", pubsub.SimpleQueueTypeDurable, getOAuth)
	if err != nil {
		con.Close()
		ch.Close()
		return nil, fmt.Errorf("Error subscribing to RabbitMQ: %w", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, "twitch", "twitch.chat", "twitch.chat.commands.gc")
	if err != nil {
		con.Close()
		ch.Close()
		return nil, fmt.Errorf("Error declaring and binding queue: %w", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, "twitch", "twitch.chat", "twitch.chat.commands.recipe")
	if err != nil {
		con.Close()
		ch.Close()
		return nil, fmt.Errorf("Error declaring and binding queue: %w", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, "twitch", "twitch.chat", "twitch.chat.commands.tts")
	if err != nil {
		con.Close()
		ch.Close()
		return nil, fmt.Errorf("Error declaring and binding queue: %w", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, "twitch", "twitch.chat", "twitch.chat")
	if err != nil {
		con.Close()
		ch.Close()
		return nil, fmt.Errorf("Error declaring and binding queue: %w", err)
	}
	return ch, nil
}
