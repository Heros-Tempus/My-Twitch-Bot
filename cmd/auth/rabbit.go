package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) setupRabbitMQ(uri string) error {
	con, err := pubsub.ConnectWithBackoff(uri, 5)
	if err != nil {
		return fmt.Errorf("error connecting to RabbitMQ: %w", err)
	}
	log.Println("Connected to RabbitMQ")

	ch, err := con.Channel()
	if err != nil {
		con.Close()
		return fmt.Errorf("error creating channel: %w", err)
	}

	a.rabbitConn = con
	a.rabbitChan = ch

	if err := pubsub.DeclareExchange(a.rabbitChan, pubsub.ExchangeBot, "topic"); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error declaring RabbitMQ topic exchange: %w", err)
	}

	// Pre-declare queues so the first token isn't dropped if consumers boot slowly
	queuesToBind := []struct {
		queue pubsub.QueueName
		key   pubsub.RoutingKey
	}{
		{pubsub.QueueListenerOAuth, pubsub.KeyTokenRefreshed},
		{pubsub.QueueWriterOAuth, pubsub.KeyTokenRefreshed},
		{pubsub.QueueOverlayOAuth, pubsub.KeyTokenRefreshed},
		{pubsub.QueueQuoteOAuth, pubsub.KeyTokenRefreshed},
		{pubsub.QueueOverlayAlerts, pubsub.KeyTokenFailed},
	}

	for _, qb := range queuesToBind {
		if err := pubsub.DeclareAndBindQueue(a.rabbitChan, pubsub.ExchangeBot, qb.queue, qb.key); err != nil {
			a.rabbitChan.Close()
			a.rabbitConn.Close()
			return fmt.Errorf("error declaring queue %s: %w", qb.queue, err)
		}
	}
	log.Println("Successfully declared and bound all consumer auth queues.")

	err = pubsub.SubscribeJSON(
		a.rabbitConn,
		pubsub.ExchangeBot,
		pubsub.QueueAuthRequests,
		pubsub.KeyTokenRefreshReq,
		pubsub.SimpleQueueTypeTransient,
		func(msg struct{}) pubsub.AckType {
			select {
			case a.refreshChan <- struct{}{}:
			default:
			}
			return pubsub.AckTypeAck
		},
	)
	if err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error subscribing to refresh requests: %w", err)
	}

	return nil
}

func (a *App) publishOAuth(err error) {
	if err != nil {
		log.Printf("Publishing auth failed message to RabbitMQ: %v", err)
		if pubErr := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeyTokenFailed, models.EmptySignal{}); pubErr != nil {
			log.Println("Error publishing auth failed message to RabbitMQ:", pubErr)
			return
		}
		return
	}
	if pubErr := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeyTokenRefreshed, a.oauth); pubErr != nil {
		log.Println("Error publishing OAuth token to RabbitMQ:", pubErr)
	} else {
		log.Println("Successfully published OAuth token to RabbitMQ.")
	}
}