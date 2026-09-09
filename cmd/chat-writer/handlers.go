package main

import (
	"errors"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) handleToken(payload models.OAuthToken) pubsub.AckType {
	a.tokenMu.Lock()
	a.token = payload

	close(a.tokenChange)
	a.tokenChange = make(chan struct{})
	a.tokenMu.Unlock()

	return pubsub.AckTypeAck
}

func (a *App) handleChat(payload models.ChatMessage) pubsub.AckType {
	a.tokenMu.RLock()
	token := a.token
	tokenChange := a.tokenChange
	a.tokenMu.RUnlock()
	log.Printf("Handling chat message from %s: %s", payload.User, payload.Message)
	if token.Token == "" {
		<-tokenChange
		return pubsub.AckTypeNackRequeue
	}

	err := SendChatMessage(token, payload.Message)
	if err == nil {
		return pubsub.AckTypeAck
	}
	log.Printf("Failed to send chat message: %v", err)
	if errors.Is(err, ErrTokenExpired) {
		<-tokenChange
		return pubsub.AckTypeNackRequeue
	}
	return pubsub.AckTypeNackRequeue
}
