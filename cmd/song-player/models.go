package main

import (
	"sync"
	"time"

	"github.com/andreykaipov/goobs"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	rabbitConn    *amqp.Connection
	rabbitChan    *amqp.Channel
	obs           *goobs.Client
	browserSource string
	textSource    string
	sceneName     string
	textItemId    int
	desktopIP     string 
	mu            sync.Mutex
	timer         *time.Timer
}