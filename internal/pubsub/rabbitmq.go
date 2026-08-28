package pubsub

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
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

func IsConnRefused(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var sysErr *os.SyscallError
		if errors.As(opErr.Err, &sysErr) {
			return errors.Is(sysErr.Err, syscall.ECONNREFUSED)
		}
	}
	return false
}
