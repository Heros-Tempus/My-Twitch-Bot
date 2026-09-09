package main

import (
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func setupSubscriptions(con *amqp.Connection, ch *amqp.Channel, app *App) {
	_ = pubsub.DeclareExchange(ch, "player", "topic")
	_ = pubsub.SubscribeJSON(con, "player", "player.responses.track", "player.track.next", pubsub.SimpleQueueTypeDurable, func(payload models.PlayerTrackPayload) pubsub.AckType {
		app.trackChan <- payload
		return pubsub.AckTypeAck
	})

	_ = pubsub.SubscribeJSON(con, "player", "player.responses.status", "player.status.response", pubsub.SimpleQueueTypeDurable, func(payload models.PlayerStatusResponse) pubsub.AckType {
		app.statusChan <- payload
		return pubsub.AckTypeAck
	})

	_ = pubsub.SubscribeJSON(con, "player", "player.responses.skip", "player.action.skip", pubsub.SimpleQueueTypeDurable, func(msg models.EmptySignal) pubsub.AckType {
		app.skipChan <- struct{}{}
		return pubsub.AckTypeAck
	})
}

func (a *App) sendStatusRequest() {
	const exchange = "player"
	const key = "player.signals.status_request"
	const targetQueue = "player.requests.status"

	err := pubsub.DeclareAndBindQueue(a.rabbit, exchange, targetQueue, key)
	if err != nil {
		log.Printf("Warning: Failed to pre-declare queue-manager status queue: %v", err)
	}
	if err := pubsub.PublishJSON(a.rabbit, exchange, key, models.EmptySignal{}); err != nil {
		log.Printf("Failed to send status_request: %v", err)
	}
}

func (a *App) sendReadySignal() {
	const exchange = "player"
	const key = "player.signals.ready"

	if err := pubsub.PublishJSON(a.rabbit, exchange, key, models.EmptySignal{}); err != nil {
		log.Printf("Failed to send player_ready: %v", err)
	}
}
