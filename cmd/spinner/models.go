package main

import (
	"sync"

	"github.com/andreykaipov/goobs"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	rabbitConn    *amqp.Connection
	rabbitChan    *amqp.Channel
	obs           *goobs.Client
	browserSource string
	sceneName     string
	spinnerItemId int
	hostIP        string
	dataPath      string
	mu            sync.Mutex
	isSpinning    bool
}
