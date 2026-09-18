package main

import (
	"errors"
	"sync"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	rabbitConn  *amqp.Connection
	rabbitChan      *amqp.Channel
	tokenMu     sync.RWMutex
	token       models.OAuthToken
	tokenChange chan struct{}
}

func newApp() *App {
	return &App{
		tokenChange: make(chan struct{}),
	}
}

var ErrTokenExpired = errors.New("token expired")