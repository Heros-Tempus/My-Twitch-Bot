package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func setupRabbitMQSubscriptions(con *amqp.Connection, ch *amqp.Channel, app *App) error {
	err := pubsub.DeclareExchange(ch, "player", "topic")
	if err != nil {
		return fmt.Errorf("Error declaring player exchange: %w", err)
	}
	err = pubsub.DeclareExchange(ch, pubsub.ExchangeBot, "topic")
	if err != nil {
		return fmt.Errorf("Error declaring Twitch exchange: %w", err)
	}
	
	err = pubsub.DeclareAndBindQueue(ch, "player", "player.requests.status", "player.signals.status_request")
	if err != nil {
		log.Printf("Warning: Failed to pre-declare queue-manager status queue: %v", err)
	}
	
	err = pubsub.DeclareAndBindQueue(ch, "player", "player.requests.ready", "player.signals.ready")
	if err != nil {
		log.Printf("Warning: Failed to pre-declare queue-manager ready queue: %v", err)
	}
	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueSongManagerCommands, pubsub.KeyCmdSong, pubsub.SimpleQueueTypeDurable, app.handleChatCommand)
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
	err := pubsub.PublishJSON(r.ch, pubsub.ExchangeBot, pubsub.KeyChatMessage, models.ChatMessage{Message: message, User: "bot"})
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
	payload := models.PlayerStatusResponse{Status: status, TimeRemaining: timeRemaining}

	if err := pubsub.PublishJSON(r.ch, exchange, key, payload); err != nil {
		log.Printf("Failed to send player status: %v", err)
	}
}

func (r *RabbitClient) SendNextTrack(payload models.PlayerTrackPayload) {
	const exchange = "player"
	const key = "player.track.next"

	if err := pubsub.PublishJSON(r.ch, exchange, key, payload); err != nil {
		log.Printf("Failed to send next track to player: %v", err)
	}
}
