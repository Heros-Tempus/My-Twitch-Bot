package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func getOAuth(msg Token) pubsub.AckType {
	if msg.Token == "" {
		tokenStore.Revoke()
		log.Println("Received error signal from Auth, shutting down")
		return pubsub.AckTypeAck
	}

	updated := tokenStore.UpdateIfNewer(msg)
	if !updated {
		log.Println("Received token older than current token")
	} else {
		log.Printf("OAuth refreshed for bot account %s", msg.BotAccountID)

		isFirstToken := false
		firstTokenOnce.Do(func() {
			close(firstTokenReceived)
			isFirstToken = true
		})

		if !isFirstToken {
			ConnCancelMu.Lock()
			if CurrentConnCancel != nil {
				log.Println("Token rotated. Signaling WebSocket to reconnect...")
				CurrentConnCancel()
			}
			ConnCancelMu.Unlock()
		}
	}

	return pubsub.AckTypeAck
}

func BuildCommandRouter(ch *amqp.Channel) func(msg string) {

	return func(msg string) {

		parts := strings.SplitN(msg, " ", 2)
		command := strings.ToLower(parts[0])

		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}
		switch command {
		case "!gc":
			fmt.Printf("-> Simulating GC. (Args provided: %q)\n", args)
			err := pubsub.PublishJSON(ch, "twitch", "twitch.chat.commands.gc", Command{
				Name: "gc",
				Args: args,
			})
			if err != nil {
				log.Printf("Error publishing GC command: %v", err)
			}
		case "!recipe":
			fmt.Printf("-> Simulating recipe command. (Args provided: %q)\n", args)
			err := pubsub.PublishJSON(ch, "twitch", "twitch.chat.commands.recipe", Command{
				Name: "recipe",
				Args: args,
			})
			if err != nil {
				log.Printf("Error publishing recipe command: %v", err)
			}
		case "!tts":
			fmt.Printf("-> Simulating TTS command. (Args provided: %q)\n", args)
			err := pubsub.PublishJSON(ch, "twitch", "twitch.chat.commands.tts", Command{
				Name: "tts",
				Args: args,
			})
			if err != nil {
				log.Printf("Error publishing TTS command: %v", err)
			}
		default:
			fmt.Printf("-> Simulating non-command message: %q\n", msg)
			err := pubsub.PublishJSON(ch, "twitch", "twitch.chat", ChatMessage{
				Message: msg,
			})
			if err != nil {
				log.Printf("Error publishing chat message: %v", err)
			}
		}
	}
}
