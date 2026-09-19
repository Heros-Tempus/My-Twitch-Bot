package main

import (
	"log"
	"strings"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) getOAuth(msg models.OAuthToken) pubsub.AckType {
	if msg.Token == "" {
		a.revokeToken()
		log.Println("Received error signal from Auth, shutting down")
		return pubsub.AckTypeAck
	}

	updated := a.updateToken(msg)
	if !updated {
		log.Println("Received token older than current token")
	} else {
		log.Printf("OAuth refreshed for bot account %s", msg.BotAccountID)

		isFirstToken := false
		a.firstTokenOnce.Do(func() {
			close(a.firstTokenReceived)
			isFirstToken = true
		})

		if !isFirstToken {
			log.Println("Token rotated. Signaling WebSocket to reconnect...")
			a.cancelConnection()
		}
	}

	return pubsub.AckTypeAck
}

func (a *App) RouteCommand(msg string, user string, m models.OverlayMessage) {
	log.Printf("Received message from %s: %s", user, msg)
	parts := strings.SplitN(msg, " ", 2)
	command := strings.ToLower(parts[0])

	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}
	switch command {
	case "!gc", "!gamechops":
		err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeyCmdSong, models.Command{
			User: user,
			Name: "gc",
			Args: args,
		})
		if err != nil {
			log.Printf("Error publishing GC command: %v", err)
		}
	case "!quote":
		err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeyCmdQuote, models.Command{
			User: user,
			Name: "quote",
			Args: args,
		})
		if err != nil {
			log.Printf("Error publishing quote command: %v", err)
		}
	default:
		err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeyChatOverlay, m)
		if err != nil {
			log.Printf("Error publishing chat message: %v", err)
		}
	}
}

func (a *App) HandleRevocation(revocation SubscriptionRevocation) {
	err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeyTokenRefreshReq, revocation)
	if err != nil {
		log.Printf("Error publishing revocation: %v", err)
	}
}