package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func notifyRabbit(ch *amqp.Channel, token models.OAuthToken, err error) {
	if err != nil {
		if pubErr := pubsub.PublishJSON(ch, pubsub.ExchangeBot, pubsub.KeyTokenFailed, models.OAuthToken{}); pubErr != nil {
			log.Println("Error publishing auth failed message to RabbitMQ:", pubErr)
			return
		}
	}
	if pubErr := pubsub.PublishJSON(ch, pubsub.ExchangeBot, pubsub.KeyTokenRefreshed, token); pubErr != nil {
		log.Println("Error publishing OAuth token to RabbitMQ:", pubErr)
	}
	log.Println("Successfully published OAuth token to RabbitMQ.")
}

func setupRefreshListener(con *amqp.Connection) (<-chan struct{}, error) {
	refreshChan := make(chan struct{}, 1)
	err := pubsub.SubscribeJSON(
		con,
		pubsub.ExchangeBot,
		pubsub.QueueAuthRequests,
		pubsub.KeyTokenRefreshReq,
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

	if err := pubsub.DeclareExchange(rabbitChan, pubsub.ExchangeBot, "topic"); err != nil {
		log.Fatal("Error declaring RabbitMQ topic exchange:", err)
	}
	log.Println("Declared RabbitMQ topic exchange")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, pubsub.ExchangeBot, pubsub.QueueListenerOAuth, pubsub.KeyTokenRefreshed); err != nil {
		log.Fatal("Error declaring auth.refreshed.listener queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.listener queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, pubsub.ExchangeBot, pubsub.QueueWriterOAuth, pubsub.KeyTokenRefreshed); err != nil {
		log.Fatal("Error declaring auth.refreshed.writer queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.writer queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, pubsub.ExchangeBot, pubsub.QueueOverlayOAuth, pubsub.KeyTokenRefreshed); err != nil {
		log.Fatal("Error declaring auth.refreshed.overlay queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.overlay queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, pubsub.ExchangeBot, pubsub.QueueQuoteOAuth, pubsub.KeyTokenRefreshed); err != nil {
		log.Fatal("Error declaring auth.refreshed.quote queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.quote queue")
	if err := pubsub.DeclareAndBindQueue(rabbitChan, pubsub.ExchangeBot, pubsub.QueueOverlayAlerts, pubsub.KeyTokenFailed); err != nil {
		log.Fatal("Error declaring auth.failed.overlay queue:", err)
	}
	log.Println("Declared and bound auth.failed.overlay queue")
	return con, rabbitChan, nil
}
