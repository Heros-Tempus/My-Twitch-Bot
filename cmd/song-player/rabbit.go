package main

import (
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func setupSubscriptions(con *amqp.Connection, ch *amqp.Channel, app *App) {
	err := pubsub.DeclareExchange(ch, "player", "topic")
	if err != nil {
		log.Printf("Failed to declare exchange: %v", err)
	}
	err = pubsub.DeclareExchange(ch, pubsub.ExchangeBot, "topic")
	if err != nil {
		log.Printf("Failed to declare exchange: %v", err)
	}

	err = pubsub.DeclareAndBindQueue(ch, pubsub.ExchangeBot, pubsub.QueueSongPlayerControls, pubsub.KeySongPlay)
	if err != nil {
		log.Printf("Failed to declare and bind queue: %v", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, pubsub.ExchangeBot, pubsub.QueueSongManagerStatus, pubsub.KeySongStatusReq)
	if err != nil {
		log.Printf("Failed to declare and bind queue: %v", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, pubsub.ExchangeBot, pubsub.QueueSongManagerSkip, pubsub.KeySongReady)
	if err != nil {
		log.Printf("Failed to declare and bind queue: %v", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, "player", "player.responses.skip", "player.action.skip")
	if err != nil {
		log.Printf("Failed to declare and bind queue: %v", err)
	}
	err = pubsub.DeclareAndBindQueue(ch, "player", "player.responses.status", "player.action.response")
	if err != nil {
		log.Printf("Failed to declare and bind queue: %v", err)
	}

	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueSongPlayerControls, pubsub.KeySongPlay, pubsub.SimpleQueueTypeDurable, app.handleTrack)
	if err != nil {
		log.Fatalf("Failed to subscribe to track responses: %v", err)
	}

	err = pubsub.SubscribeJSON(con, "player", "player.responses.status", "player.status.response", pubsub.SimpleQueueTypeDurable, app.handleStatus)
	if err != nil {
		log.Fatalf("Failed to subscribe to status responses: %v", err)
	}

	err = pubsub.SubscribeJSON(con, "player", "player.responses.skip", "player.action.skip", pubsub.SimpleQueueTypeDurable, app.handleSkip)
	if err != nil {
		log.Fatalf("Failed to subscribe to skip responses: %v", err)
	}
}

func (a *App) sendStatusRequest() {
	if err := pubsub.PublishJSON(a.rabbit, pubsub.ExchangeBot, pubsub.KeySongStatusReq, models.EmptySignal{}); err != nil {
		log.Printf("Failed to send status_request: %v", err)
	}
}

func (a *App) sendReadySignal() {
	if err := pubsub.PublishJSON(a.rabbit, pubsub.ExchangeBot, pubsub.KeySongReady, models.EmptySignal{}); err != nil {
		log.Printf("Failed to send player_ready: %v", err)
	}
}
