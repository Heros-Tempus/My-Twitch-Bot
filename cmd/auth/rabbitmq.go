package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	amqp "github.com/rabbitmq/amqp091-go"
)

func connectWithBackoff(uri string, maxAttempts int) (*amqp.Connection, error) {
	backoff := time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		con, err := amqp.Dial(uri)
		if err == nil {
			return con, nil
		}

		var amqpErr *amqp.Error
		if errors.As(err, &amqpErr) {
			switch amqpErr.Code {
			case 403, 530:
				return nil, fmt.Errorf("non-retryable AMQP error: %w", err)
			}
		}

		var netErr net.Error
		if errors.As(err, &netErr) || isConnRefused(err) {
			fmt.Printf("connect attempt %d failed (%v), retrying in %s\n", attempt, err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}

		return nil, fmt.Errorf("unrecoverable error connecting to RabbitMQ: %w", err)
	}

	return nil, fmt.Errorf("failed to connect after %d attempts", maxAttempts)
}

func isConnRefused(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var sysErr *os.SyscallError
		if errors.As(opErr.Err, &sysErr) {
			return errors.Is(sysErr.Err, syscall.ECONNREFUSED)
		}
	}
	return false
}

func notifyRabbit(chann *amqp.Channel, token Oauth, err error) {
	if err != nil {
		if pubErr := pubsub.PublishJSON(chann, "twitch.topic", "auth.failed", Oauth{}); pubErr != nil {
			log.Fatal("Error publishing auth failed message to RabbitMQ:", pubErr)
			return
		}
	}

	if pubErr := pubsub.PublishJSON(chann, "twitch.auth", "auth.refreshed.#", token); pubErr != nil {
		log.Fatal("Error publishing OAuth token to RabbitMQ:", pubErr)
	}
	log.Println("Successfully published OAuth token to RabbitMQ.")
}

func setupRabbitMQ(uri string) (*amqp.Connection, *amqp.Channel, error) {
	con, err := connectWithBackoff(uri, 5)
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting to RabbitMQ: %w", err)
	}
	log.Println("Connected to RabbitMQ")

	rabbitChan, err := con.Channel()
	if err != nil {
		con.Close()
		return nil, nil, fmt.Errorf("error creating channel: %w", err)
	}

	if err := pubsub.DeclareExchange(rabbitChan, "twitch.auth", "fanout"); err != nil {
		log.Fatal("Error declaring RabbitMQ fanout exchange:", err)
	}
	log.Println("Declared RabbitMQ fanout exchange")

	if err := pubsub.DeclareExchange(rabbitChan, "twitch.topic", "topic"); err != nil {
		log.Fatal("Error declaring RabbitMQ topic exchange:", err)
	}
	log.Println("Declared RabbitMQ topic exchange")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch.auth", "auth.refreshed.listener", ""); err != nil {
		log.Fatal("Error declaring auth.refreshed.listener queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.listener queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch.auth", "auth.refreshed.writer", ""); err != nil {
		log.Fatal("Error declaring auth.refreshed.writer queue:", err)
	}
	log.Println("Declared and bound auth.refreshed.writer queue")

	if err := pubsub.DeclareAndBindQueue(rabbitChan, "twitch.topic", "auth.failed", ""); err != nil {
		log.Fatal("Error declaring auth.failed queue:", err)
	}
	log.Println("Declared and bound auth.failed queue")

	return con, rabbitChan, nil
}