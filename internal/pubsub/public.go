package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func ConnectWithBackoff(uri string, maxAttempts int) (*amqp.Connection, error) {
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
		if errors.As(err, &netErr) || IsConnRefused(err) {
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

func ConnectWithJitter(uri string, maxAttempts int) (*amqp.Connection, error) {
	baseBackoff := time.Second
	maxBackoff := 30 * time.Second

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
		if errors.As(err, &netErr) || IsConnRefused(err) {
			temp := baseBackoff
			for i := 1; i < attempt; i++ {
				temp *= 2
				if temp > maxBackoff {
					temp = maxBackoff
					break
				}
			}

			sleepDuration := time.Duration(rand.Float64() * float64(temp))

			fmt.Printf("connect attempt %d failed (%v), retrying in %s (max ceiling was %s)\n",
				attempt, err, sleepDuration, temp)

			time.Sleep(sleepDuration)
			continue
		}
		return nil, fmt.Errorf("unrecoverable error connecting to RabbitMQ: %w", err)
	}

	return nil, fmt.Errorf("failed to connect after %d attempts", maxAttempts)
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange ExchangeName, queueName QueueName, key RoutingKey, queueType SimpleQueueType, handler func(T) AckType) error {
	return subscribe(conn, exchange, queueName, key, queueType, handler, func(data []byte) (T, error) {
		var val T
		err := json.Unmarshal(data, &val)
		return val, err
	})
}

func SubscribeGob[T any](conn *amqp.Connection, exchange ExchangeName, queueName QueueName, key RoutingKey, queueType SimpleQueueType, handler func(T) AckType) error {
	return subscribe(conn, exchange, queueName, key, queueType, handler, func(data []byte) (T, error) {
		var val T
		err := DecodeGob(data, &val)
		return val, err
	})
}

func PublishJSON[T any](ch *amqp.Channel, exchange ExchangeName, key RoutingKey, val T) error {
	body, err := json.Marshal(val)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		string(exchange),
		string(key),
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

func PublishGob[T any](ch *amqp.Channel, exchange ExchangeName, key RoutingKey, val T) error {
	body, err := EncodeGob(val)
	if err != nil {
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		string(exchange),
		string(key),
		false,
		false,
		amqp.Publishing{
			ContentType: "application/gob",
			Body:        body,
		},
	)
}
