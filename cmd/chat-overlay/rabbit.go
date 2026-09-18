package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) setupRabbitMQ(rabbitConString string) error {
	con, err := pubsub.ConnectWithBackoff(rabbitConString, 5)
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

	if err := pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueOverlayOAuth, pubsub.KeyTokenRefreshed, pubsub.SimpleQueueTypeDurable, a.handleAuthUpdate); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error subscribing to auth messages: %w", err)
	}

	if err := pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueOverlayAlerts, pubsub.KeyTokenFailed, pubsub.SimpleQueueTypeDurable, a.handleAuthFailure); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error subscribing to auth failures: %w", err)
	}

	return nil
}

func (a *App) subscribeToChat() error {
	if err := pubsub.SubscribeJSON(a.rabbitConn, pubsub.ExchangeBot, pubsub.QueueOverlayMessages, pubsub.KeyChatOverlay, pubsub.SimpleQueueTypeDurable, a.handleIncomingMessage); err != nil {
		return fmt.Errorf("error subscribing to chat overlay: %w", err)
	}
	return nil
}