package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
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

	a.rabbitChan = ch
	a.rabbitConn = con

	if err := pubsub.DeclareExchange(a.rabbitChan, pubsub.ExchangeBot, "topic"); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error declaring Twitch exchange: %w", err)
	}

	if err := pubsub.SubscribeJSON(a.rabbitConn, pubsub.ExchangeBot, pubsub.QueueSongManagerCommands, pubsub.KeyCmdSong, pubsub.SimpleQueueTypeDurable, a.handleChatCommand); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error subscribing to chat commands: %w", err)
	}
	
	if err := pubsub.SubscribeJSON(a.rabbitConn, pubsub.ExchangeBot, pubsub.QueueSongManagerStatus, pubsub.KeySongStatusReq, pubsub.SimpleQueueTypeDurable, a.handlePlayerStatusRequest); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error subscribing to player status requests: %w", err)
	}
	
	if err := pubsub.SubscribeJSON(a.rabbitConn, pubsub.ExchangeBot, pubsub.QueueSongManagerSkip, pubsub.KeySongReady, pubsub.SimpleQueueTypeDurable, a.handlePlayerReady); err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("error subscribing to player ready signals: %w", err)
	}

	return nil
}

func (a *App) sendToChat(message string) {
	err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeyChatMessage, models.ChatMessage{Message: message, User: "bot"})
	if err != nil {
		log.Printf("Failed to send message to chat: %v", err)
	}
	log.Println(message)
}

func (a *App) Skip() {
	err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeySongSkip, models.EmptySignal{})
	if err != nil {
		log.Printf("Failed to publish skip signal: %v", err)
	}
}

func (a *App) SendPlayerStatus(status string, timeRemaining int32) {
	payload := models.PlayerStatusResponse{Status: status, TimeRemaining: timeRemaining}
	if err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeySongStatusReply, payload); err != nil {
		log.Printf("Failed to send player status: %v", err)
	}
}

func (a *App) SendNextTrack(payload models.PlayerTrackPayload) {
	if err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeySongPlay, payload); err != nil {
		log.Printf("Failed to send next track to player: %v", err)
	}
}