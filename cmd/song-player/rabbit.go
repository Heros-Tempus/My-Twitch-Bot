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
	a.rabbitConn = con
	a.rabbitChan = ch

	err = pubsub.DeclareExchange(a.rabbitChan, pubsub.ExchangeBot, "topic")
	if err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueSongPlayerSong, pubsub.KeySongPlay, pubsub.SimpleQueueTypeDurable, a.handleTrack)
	if err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("failed to subscribe to track responses: %w", err)
	}
	
	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueSongManagerStatus, pubsub.KeySongStatusReply, pubsub.SimpleQueueTypeDurable, a.handleStatus)
	if err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("failed to subscribe to status responses: %w", err)
	}
	
	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueSongManagerSkip, pubsub.KeySongSkip, pubsub.SimpleQueueTypeDurable, a.handleSkip)
	if err != nil {
		a.rabbitChan.Close()
		a.rabbitConn.Close()
		return fmt.Errorf("failed to subscribe to skip responses: %w", err)
	}
	
	return nil
}

func (a *App) sendStatusRequest() {
	if err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeySongStatusReq, models.EmptySignal{}); err != nil {
		log.Printf("Failed to send status_request: %v", err)
	}
}

func (a *App) sendReadySignal() {
	if err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeySongReady, models.EmptySignal{}); err != nil {
		log.Printf("Failed to send player_ready: %v", err)
	}
}