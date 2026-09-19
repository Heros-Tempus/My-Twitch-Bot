package main
import (
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	db          *database.Queries
	rabbitConn  *amqp.Connection
	rabbitChan      *amqp.Channel
	refreshChan chan struct{}
	oauth       models.OAuthToken
}

func newApp(db *database.Queries) *App {
	return &App{
		db:          db,
		refreshChan: make(chan struct{}, 1),
	}
}

type TwitchTokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}
