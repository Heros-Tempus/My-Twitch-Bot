package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func setupRabbitMQ(uri string, oauthHandler func(Token) pubsub.AckType) (*amqp.Channel, error) {
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

	err = pubsub.DeclareExchange(ch, "twitch", "topic")
	if err != nil {
		con.Close()
		ch.Close()
		return nil, fmt.Errorf("Error declaring Twitch exchange: %w", err)
	}

	err = pubsub.SubscribeJSON(con, "twitch", "auth.refreshed.listener", "auth.refreshed.listener", pubsub.SimpleQueueTypeDurable, oauthHandler)
	if err != nil {
		con.Close()
		ch.Close()
		return nil, fmt.Errorf("Error subscribing to RabbitMQ: %w", err)
	}
	return ch, nil
}
