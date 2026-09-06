package main

import (
	"github.com/andreykaipov/goobs"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PlayerStatusResponse struct {
	Status        string `json:"status"`
	TimeRemaining int32  `json:"time_remaining,omitempty"`
}

type PlayerTrackPayload struct {
	Url               string `json:"url"`
	Duration          int32  `json:"duration"`
	AttributionString string `json:"attribution_string"`
}

type EmptySignal struct{}

type App struct {
	rabbit        *amqp.Channel
	obs           *goobs.Client
	browserSource string
	textSource    string
	sceneName     string
	textItemId    int

	trackChan  chan PlayerTrackPayload
	statusChan chan PlayerStatusResponse
	skipChan   chan struct{}
}
