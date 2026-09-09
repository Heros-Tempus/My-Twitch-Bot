package main

import (
	"database/sql"
	"sync"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Service struct {
	db      *sql.DB
	queries *database.Queries
}

type Action int

const (
	ActionQueue Action = iota
	ActionHelp
	ActionSkip
	ActionClear
	ActionShuffle
	ActionPeek
)

type ParsedCommand struct {
	Action Action
	Params database.QueueRandomTracksParams
}

type Track struct {
	Artist           string
	Track            string
	SourceMedia      string
	Album            string
	OriginalComposer string
}

type RabbitClient struct {
	ch *amqp.Channel
}

type App struct {
	rabbit  *RabbitClient
	service *Service
	mu      sync.Mutex
	isIdle  bool
}

func NewService(dbConn *sql.DB) *Service {
	return &Service{
		db:      dbConn,
		queries: database.New(dbConn),
	}
}
