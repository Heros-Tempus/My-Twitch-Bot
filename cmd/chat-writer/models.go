package main

import (
	"errors"
	"sync"
	"time"
)

type Token struct {
	BotAccountID string
	OwnerID      string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}

type ChatMessage struct {
	Message string `json:"message"`
	User    string `json:"user"`
}

type App struct {
	tokenMu     sync.RWMutex
	token       Token
	tokenChange chan struct{}
}

func newApp() *App {
	return &App{
		tokenChange: make(chan struct{}),
	}
}

var ErrTokenExpired = errors.New("token expired")
