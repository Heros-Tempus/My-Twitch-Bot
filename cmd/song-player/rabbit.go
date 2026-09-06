package main

import (
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func setupSubscriptions(con *amqp.Connection, ch *amqp.Channel, app *App) {
	_ = pubsub.DeclareExchange(ch, "player", "topic")
	_ = pubsub.SubscribeJSON(con, "player", "player.responses.track", "player.track.next", pubsub.SimpleQueueTypeDurable, func(payload PlayerTrackPayload) pubsub.AckType {
		app.trackChan <- payload
		return pubsub.AckTypeAck
	})

	_ = pubsub.SubscribeJSON(con, "player", "player.responses.status", "player.status.response", pubsub.SimpleQueueTypeDurable, func(payload PlayerStatusResponse) pubsub.AckType {
		app.statusChan <- payload
		return pubsub.AckTypeAck
	})

	_ = pubsub.SubscribeJSON(con, "player", "player.responses.skip", "player.action.skip", pubsub.SimpleQueueTypeDurable, func(msg EmptySignal) pubsub.AckType {
		app.skipChan <- struct{}{}
		return pubsub.AckTypeAck
	})
}
