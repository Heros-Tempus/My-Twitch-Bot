package pubsub

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
)

func DeclareAndBindQueue(ch *amqp.Channel, exchange ExchangeName, queueName QueueName, routingKey RoutingKey) error {
	table := make(amqp.Table)
	table["x-dead-letter-exchange"] = "twitch.dlx"
	_, err := ch.QueueDeclare(
		string(queueName),
		true,
		false,
		false,
		false,
		table,
	)
	if err != nil {
		return err
	}
	return ch.QueueBind(
		string(queueName),
		string(routingKey),
		string(exchange),
		false,
		nil,
	)
}

func DeclareAndBind(conn *amqp.Connection, exchange ExchangeName, queueName QueueName, key RoutingKey, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	rabbitChan, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	table := make(amqp.Table)
	table["x-dead-letter-exchange"] = "twitch.dlx"
	rabbitQueue, err := rabbitChan.QueueDeclare(
		string(queueName),
		queueType == SimpleQueueTypeDurable,
		queueType == SimpleQueueTypeTransient,
		queueType == SimpleQueueTypeTransient,
		false,
		table,
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	err = rabbitChan.QueueBind(
		rabbitQueue.Name,
		string(key),
		string(exchange),
		false,
		nil,
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	return rabbitChan, rabbitQueue, nil
}

func subscribe[T any](conn *amqp.Connection, exchange ExchangeName, queueName QueueName, key RoutingKey, simpleQueueType SimpleQueueType, handler func(T) AckType, unmarshaller func([]byte) (T, error),
) error {
	c, q, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return fmt.Errorf("Could not declare and bind queue: %v", err)
	}
	err = c.Qos(10, 0, false)
	if err != nil {
		return fmt.Errorf("Could not set QoS: %v", err)
	}
	msgs, err := c.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("Could not start consuming messages: %v", err)
	}
	go func() {
		for d := range msgs {
			var val T
			val, err := unmarshaller(d.Body)
			if err != nil {
				fmt.Printf("Failed to unmarshal message: %v\n", err)
				continue
			}
			ack := handler(val)
			switch ack {
			case AckTypeAck:
				d.Ack(false)
			case AckTypeNackRequeue:
				d.Nack(false, true)
			case AckTypeNackDiscard:
				d.Nack(false, false)
			}
		}
	}()
	return nil
}

func EncodeGob[T any](val T) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodeGob[T any](data []byte, val *T) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(val)
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
