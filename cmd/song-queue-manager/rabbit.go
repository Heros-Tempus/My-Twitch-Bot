package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func setupRabbitMQSubscriptions(con *amqp.Connection, ch *amqp.Channel, app *App) error {
	err := pubsub.DeclareExchange(ch, "player", "topic")
	if err != nil {
		return fmt.Errorf("Error declaring player exchange: %w", err)
	}
	err = pubsub.DeclareExchange(ch, "twitch", "topic")
	if err != nil {
		return fmt.Errorf("Error declaring Twitch exchange: %w", err)
	}
	err = pubsub.SubscribeJSON(con, "twitch", "song-queue-manager.commands.gc", "twitch.chat.commands.gc", pubsub.SimpleQueueTypeDurable, app.handleChatCommand)
	if err != nil {
		return fmt.Errorf("Error subscribing to chat commands: %w", err)
	}

	err = pubsub.SubscribeJSON(con, "player", "player.requests.status", "player.signals.status_request", pubsub.SimpleQueueTypeDurable, app.handlePlayerStatusRequest)
	if err != nil {
		return fmt.Errorf("Error subscribing to player status requests: %w", err)
	}

	err = pubsub.SubscribeJSON(con, "player", "player.requests.ready", "player.signals.ready", pubsub.SimpleQueueTypeDurable, app.handlePlayerReady)
	if err != nil {
		return fmt.Errorf("Error subscribing to player ready signals: %w", err)
	}

	return nil
}

func (r *RabbitClient) sendToChat(message string) {
	const exchange = "twitch"
	const key = "twitch.chat.send"
	const user = "test user"
	err := pubsub.PublishJSON(r.ch, exchange, key, ChatPayload{Message: message, User: user})
	if err != nil {
		log.Printf("Failed to send message to chat: %v", err)
	}
	log.Println(message)
}

func (r *RabbitClient) Skip() {
	const exchange = "player"
	const key = "player.action.skip"
	err := pubsub.PublishJSON(r.ch, exchange, key, struct{}{})
	if err != nil {
		log.Printf("Failed to publish skip signal: %v", err)
	}
}

func (r *RabbitClient) SendPlayerStatus(status string, timeRemaining int32) {
	const exchange = "player"
	const key = "player.status.response"
	payload := PlayerStatusResponse{Status: status, TimeRemaining: timeRemaining}

	if err := pubsub.PublishJSON(r.ch, exchange, key, payload); err != nil {
		log.Printf("Failed to send player status: %v", err)
	}
}

func (r *RabbitClient) SendNextTrack(payload PlayerTrackPayload) {
	const exchange = "player"
	const key = "player.track.next"

	if err := pubsub.PublishJSON(r.ch, exchange, key, payload); err != nil {
		log.Printf("Failed to send next track to player: %v", err)
	}
}
