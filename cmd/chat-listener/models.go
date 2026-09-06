package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Command struct {
	User string
	Name string
	Args string
}

type ChatMessage struct {
	User    string
	Message string
}

type App struct {
	rabbit *amqp.Channel

	tokenMu       sync.RWMutex
	token         Token
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

func (a *App) getToken() (Token, bool) {
	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()
	return a.token, a.hasValidToken
}

func (a *App) updateToken(msg Token) bool {
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
	Event struct {
		BroadcasterUserID   string `json:"broadcaster_user_id"`
		BroadcasterUserName string `json:"broadcaster_user_name"`
		ChatterUserID       string `json:"chatter_user_id"`
		ChatterUserName     string `json:"chatter_user_name"`
		Message             struct {
			Text string `json:"text"`
		} `json:"message"`
	} `json:"event"`
}

type Token struct {
	BotAccountID string
	OwnerID      string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}
