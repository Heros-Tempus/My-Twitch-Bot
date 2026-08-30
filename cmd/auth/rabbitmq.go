package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func notifyRabbit(chann *amqp.Channel, token Oauth, err error) {
	if err != nil {
		if pubErr := pubsub.PublishJSON(chann, "twitch.auth", "auth.failed", Oauth{}); pubErr != nil {
			log.Println("Error publishing auth failed message to RabbitMQ:", pubErr)
			return
		}
	}
	fmt.Printf("Publishing OAuth token to RabbitMQ: %+v\n", token)
	if pubErr := pubsub.PublishJSON(chann, "twitch.auth", "auth.refreshed.#", token); pubErr != nil {
		log.Println("Error publishing OAuth token to RabbitMQ:", pubErr)
	}
	log.Println("Successfully published OAuth token to RabbitMQ.")
}

func setupRefreshListener(con *amqp.Connection) (<-chan struct{}, error) {
	refreshChan := make(chan struct{}, 1)
	err := pubsub.SubscribeJSON(
		con,
		"auth.requests",
		"auth.refresh.listener",
		"auth.refresh.request",
		pubsub.SimpleQueueTypeTransient,
		func(msg struct{}) pubsub.AckType {
			select {
			case refreshChan <- struct{}{}:
			default:
			}
			return pubsub.AckTypeAck
		},
	)
	if err != nil {
		return nil, fmt.Errorf("Error subscribing to refresh requests: %w", err)
	}
	return refreshChan, nil
}

func setupRabbitMQ(uri string) (*amqp.Connection, *amqp.Channel, error) {
	con, err := pubsub.ConnectWithBackoff(uri, 5)
	if err != nil {
		return nil, nil, fmt.Errorf("Error connecting to RabbitMQ: %w", err)
	}
	log.Println("Connected to RabbitMQ")

	rabbitChan, err := con.Channel()
	if err != nil {
		con.Close()
		return nil, nil, fmt.Errorf("Error creating channel: %w", err)
	}

	if err := pubsub.DeclareExchange(rabbitChan, "twitch.auth", "fanout"); err != nil {
		log.Fatal("Error declaring RabbitMQ fanout exchange:", err)
	}
	log.Println("Declared RabbitMQ fanout exchange")

	if err := pubsub.DeclareExchange(rabbitChan, "auth.requests", "direct"); err != nil {
		log.Fatal("Error declaring auth.requests direct exchange:", err)
	}
	log.Println("Declared RabbitMQ direct exchange for incoming requests")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch.auth", "auth.refreshed.listener", ""); err != nil {
		log.Fatal("Error declaring auth.refreshed.listener queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.listener queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch.auth", "auth.refreshed.writer", ""); err != nil {
		log.Fatal("Error declaring auth.refreshed.writer queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.writer queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch.auth", "auth.failed", ""); err != nil {
		log.Fatal("Error declaring auth.failed queue:", err)
	}
	log.Println("Declared and bound auth.failed queue")

	return con, rabbitChan, nil
}
