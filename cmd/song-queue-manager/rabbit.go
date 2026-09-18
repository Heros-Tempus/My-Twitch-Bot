package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func setupRabbitMQSubscriptions(con *amqp.Connection, ch *amqp.Channel, app *App) error {
	err := pubsub.DeclareExchange(ch, pubsub.ExchangeBot, "topic")
	if err != nil {
		return fmt.Errorf("Error declaring Twitch exchange: %w", err)
	}

	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueSongManagerCommands, pubsub.KeyCmdSong, pubsub.SimpleQueueTypeDurable, app.handleChatCommand)
	if err != nil {
		return fmt.Errorf("Error subscribing to chat commands: %w", err)
	}
	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueSongManagerStatus, pubsub.KeySongStatusReq, pubsub.SimpleQueueTypeDurable, app.handlePlayerStatusRequest)
	if err != nil {
		return fmt.Errorf("Error subscribing to player status requests: %w", err)
	}
	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueSongManagerSkip, pubsub.KeySongReady, pubsub.SimpleQueueTypeDurable, app.handlePlayerReady)
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
	err := pubsub.PublishJSON(r.ch, pubsub.ExchangeBot, pubsub.KeySongSkip, models.EmptySignal{})
	if err != nil {
		log.Printf("Failed to publish skip signal: %v", err)
	}
}

func (r *RabbitClient) SendPlayerStatus(status string, timeRemaining int32) {
	payload := models.PlayerStatusResponse{Status: status, TimeRemaining: timeRemaining}
	if err := pubsub.PublishJSON(r.ch, pubsub.ExchangeBot, pubsub.KeySongStatusReply, payload); err != nil {
		log.Printf("Failed to send player status: %v", err)
	}
}

func (r *RabbitClient) SendNextTrack(payload models.PlayerTrackPayload) {
	if err := pubsub.PublishJSON(r.ch, pubsub.ExchangeBot, pubsub.KeySongPlay, payload); err != nil {
		log.Printf("Failed to send next track to player: %v", err)
	}
}
