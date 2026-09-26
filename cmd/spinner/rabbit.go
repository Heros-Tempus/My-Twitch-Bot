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
	
	app.rabbitConn = con
	app.rabbitChan = ch

	err = pubsub.DeclareExchange(app.rabbitChan, pubsub.ExchangeBot, "topic")
	if err != nil {
		app.rabbitChan.Close()
		app.rabbitConn.Close()
		return fmt.Errorf("Error declaring bot exchange: %w", err)
	}
	
	return nil
}