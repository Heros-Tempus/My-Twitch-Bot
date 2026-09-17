package main

import (
	"sync"
	"time"

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
	mu            sync.Mutex
	timer         *time.Timer
	skipPending   bool
}
