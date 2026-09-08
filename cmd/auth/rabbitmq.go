package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func notifyRabbit(chann *amqp.Channel, token Oauth, err error) {
	if err != nil {
		if pubErr := pubsub.PublishJSON(chann, "twitch", "auth.refreshed.failed", Oauth{}); pubErr != nil {
			log.Println("Error publishing auth failed message to RabbitMQ:", pubErr)
			return
		}
	}
	if pubErr := pubsub.PublishJSON(chann, "twitch", "auth.refreshed.listener", token); pubErr != nil {
		log.Println("Error publishing OAuth token to RabbitMQ:", pubErr)
	}
	if pubErr := pubsub.PublishJSON(chann, "twitch", "auth.refreshed.writer", token); pubErr != nil {
		log.Println("Error publishing OAuth token to RabbitMQ:", pubErr)
	}
	if pubErr := pubsub.PublishJSON(chann, "twitch", "auth.refreshed.overlay", token); pubErr != nil {
		log.Println("Error publishing OAuth token to RabbitMQ:", pubErr)
	}
	if pubErr := pubsub.PublishJSON(chann, "twitch", "auth.refreshed.quote", token); pubErr != nil {
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

	if err := pubsub.DeclareExchange(rabbitChan, "twitch", "topic"); err != nil {
		log.Fatal("Error declaring RabbitMQ topic exchange:", err)
	}
	log.Println("Declared RabbitMQ topic exchange")

	if err := pubsub.DeclareExchange(rabbitChan, "auth.requests", "direct"); err != nil {
		log.Fatal("Error declaring auth.requests direct exchange:", err)
	}
	log.Println("Declared RabbitMQ direct exchange for incoming requests")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch", "auth.refreshed.listener", "auth.refreshed.listener"); err != nil {
		log.Fatal("Error declaring auth.refreshed.listener queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.listener queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch", "auth.refreshed.writer", "auth.refreshed.writer"); err != nil {
		log.Fatal("Error declaring auth.refreshed.writer queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.writer queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch", "auth.refreshed.overlay", "auth.refreshed.overlay"); err != nil {
		log.Fatal("Error declaring auth.refreshed.overlay queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.overlay queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch", "auth.refreshed.quote", "auth.refreshed.quote"); err != nil {
		log.Fatal("Error declaring auth.refreshed.quote queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.quote queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch", "auth.refreshed.failed", "auth.refreshed.failed"); err != nil {
		log.Fatal("Error declaring auth.refreshed.failed queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.failed queue")
	return con, rabbitChan, nil
}
