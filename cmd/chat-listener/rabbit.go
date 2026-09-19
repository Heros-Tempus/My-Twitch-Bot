package main

import (
	"fmt"
	"log"

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
		return fmt.Errorf("error opening RabbitMQ channel: %w", err)
	}

	a.rabbitConn = con
	a.rabbitChan = ch

	if err := pubsub.DeclareExchange(a.rabbitChan, pubsub.ExchangeBot, "topic"); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error declaring bot exchange: %w", err)
	}

	if err := pubsub.SubscribeJSON(a.rabbitConn, pubsub.ExchangeBot, pubsub.QueueListenerOAuth, pubsub.KeyTokenRefreshed, pubsub.SimpleQueueTypeDurable, a.getOAuth); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error subscribing to RabbitMQ: %w", err)
	}
	
	return nil
}