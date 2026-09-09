package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	rabbit *amqp.Channel

	tokenMu       sync.RWMutex
	token         models.OAuthToken
	hasValidToken bool
	isRevoked     bool

	firstTokenOnce     sync.Once
	firstTokenReceived chan struct{}

	connMu     sync.Mutex
	connCancel context.CancelFunc
}

func newApp() *App {
	return &App{
		firstTokenReceived: make(chan struct{}),
	}
}

func (a *App) getToken() (models.OAuthToken, bool) {
	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()
	return a.token, a.hasValidToken
}

func (a *App) updateToken(msg models.OAuthToken) bool {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	if a.isRevoked || (a.hasValidToken && a.token.ExpiresAt.After(msg.ExpiresAt)) {
		return false
	}
	a.token = msg
	a.hasValidToken = true
	return true
}

func (a *App) revokeToken() {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	a.hasValidToken = false
	a.isRevoked = true
}

func (a *App) invalidateForRefresh() bool {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	if a.isRevoked || !a.hasValidToken {
		return false
	}
	a.hasValidToken = false
	return true
}

func (a *App) setConnectionCancel(cancel context.CancelFunc) {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	a.connCancel = cancel
}

func (a *App) cancelConnection() {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.connCancel != nil {
		a.connCancel()
	}
}

type SubscriptionRevocation struct {
	Subscription struct {
		ID        string    `json:"id"`
		Type      string    `json:"type"`
		Version   string    `json:"version"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"subscription"`
}

type WSEnvelope struct {
	Metadata struct {
		MessageType string `json:"message_type"`
	} `json:"metadata"`
	Payload json.RawMessage `json:"payload"`
}

type SessionWelcome struct {
	Session struct {
		ID                      string `json:"id"`
		KeepaliveTimeoutSeconds int    `json:"keepalive_timeout_seconds"`
	} `json:"session"`
}

type SessionReconnect struct {
	Session struct {
		ID           string `json:"id"`
		ReconnectURL string `json:"reconnect_url"`
	} `json:"session"`
}
type ChatNotification struct {
	Event TwitchChatMessageEvent `json:"event"`
}

type TwitchChatMessageEvent struct {
	BroadcasterUserID   string `json:"broadcaster_user_id"`
	BroadcasterUserName string `json:"broadcaster_user_name"`
	ChatterUserID       string `json:"chatter_user_id"`
	ChatterUserName     string `json:"chatter_user_name"`
	Color               string `json:"color"`
	MessageType         string `json:"message_type"`
	Badges              []struct {
		SetID string `json:"set_id"`
		ID    string `json:"id"`
	} `json:"badges"`
	Message struct {
		Text      string `json:"text"`
		Fragments []struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			Emote *struct {
				ID string `json:"id"`
			} `json:"emote"`
		} `json:"fragments"`
	} `json:"message"`
}