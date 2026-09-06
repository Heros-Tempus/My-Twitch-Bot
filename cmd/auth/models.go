package main

import (
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	db          *database.Queries
	rabbitCon   *amqp.Connection
	rabbitChan  *amqp.Channel
	refreshChan <-chan struct{}
	oauth       Oauth
}

func newApp(db *database.Queries, rabbitCon *amqp.Connection, rabbitChan *amqp.Channel, refreshChan <-chan struct{}) *App {
	return &App{
		db:          db,
		rabbitCon:   rabbitCon,
		rabbitChan:  rabbitChan,
		refreshChan: refreshChan,
	}
}

type Oauth struct {
	BotAccountID string
	OwnerID      string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}

type TwitchTokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}
