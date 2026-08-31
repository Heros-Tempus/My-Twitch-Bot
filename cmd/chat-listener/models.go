package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"
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

var (
	CurrentConnCancel context.CancelFunc
	ConnCancelMu      sync.Mutex
)
	
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

type TokenContainer struct {
	mu            sync.RWMutex
	hasValidToken bool
	isRevoked     bool
	token         Token
}

func (t *TokenContainer) Get() (Token, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.token, t.hasValidToken
}

func (t *TokenContainer) Revoke() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hasValidToken = false
	t.isRevoked = true
}

func (t *TokenContainer) InvalidateForRefresh() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.isRevoked {
		return false
	}
	if t.hasValidToken {
		t.hasValidToken = false
		return true
	}
	return false
}

func (t *TokenContainer) UpdateIfNewer(msg Token) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isRevoked {
		return false
	}

	if t.hasValidToken && t.token.ExpiresAt.After(msg.ExpiresAt) {
		return false
	}

	t.token = msg
	t.hasValidToken = true
	return true
}
