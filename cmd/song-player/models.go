package main

import (
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/andreykaipov/goobs"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	rabbit        *amqp.Channel
	obs           *goobs.Client
	browserSource string
	textSource    string
	sceneName     string
	textItemId    int

	trackChan  chan models.PlayerTrackPayload
	statusChan chan models.PlayerStatusResponse
	skipChan   chan struct{}
}
