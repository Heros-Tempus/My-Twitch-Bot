package main

import (
	"fmt"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (app *App) setupRabbitMQ(rabbitConString string) error {
	con, err := pubsub.ConnectWithBackoff(rabbitConString, 5)
	if err != nil {
		return fmt.Errorf("Error connecting to RabbitMQ: %w", err)
	}
	log.Println("Connected to RabbitMQ")

	ch, err := con.Channel()
	if err != nil {
		con.Close()
		return fmt.Errorf("Error opening RabbitMQ channel: %w", err)
	}
	
	app.RabbitConn = con
	app.RabbitChann = ch

	err = pubsub.DeclareExchange(app.RabbitChann, pubsub.ExchangeBot, "topic")
	if err != nil {
		app.RabbitChann.Close()
		app.RabbitConn.Close()
		return fmt.Errorf("Error declaring bot exchange: %w", err)
	}
	
	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueQuoteOAuth, pubsub.KeyTokenRefreshed, pubsub.SimpleQueueTypeDurable, app.HandleAuthMessage)
	if err != nil {
		app.RabbitChann.Close()
		app.RabbitConn.Close()
		return fmt.Errorf("Error subscribing to auth refreshed quote messages: %w", err)
	}
	
	err = pubsub.SubscribeJSON(con, pubsub.ExchangeBot, pubsub.QueueQuoteCommands, pubsub.KeyCmdQuote, pubsub.SimpleQueueTypeDurable, app.HandleCommandMessage)
	if err != nil {
		app.RabbitChann.Close()
		app.RabbitConn.Close()
		return fmt.Errorf("Error subscribing to quote commands: %w", err)
	}
	return nil
}