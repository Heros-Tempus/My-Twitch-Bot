package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func DeclareExchange(ch *amqp.Channel, exchange, kind string) error {
	return ch.ExchangeDeclare(exchange, kind, true, false, false, false, nil)
}

func DeclareAndBindQueue(ch *amqp.Channel, exchange, queueName, routingKey string) error {
	table := make(amqp.Table)
	table["x-dead-letter-exchange"] = "twitch.dlx"
	_, err := ch.QueueDeclare(
		queueName,
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
		queueName,
		routingKey,
		exchange,
		false,
		nil,
	)
}

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	body, err := json.Marshal(val)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	body, err := EncodeGob(val)
	if err != nil {
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/gob",
			Body:        body,
		},
	)
}

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	rabbitChan, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	table := make(amqp.Table)
	table["x-dead-letter-exchange"] = "twitch.dlx"
	rabbitQueue, err := rabbitChan.QueueDeclare(
		queueName,
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
		key,
		exchange,
		false,
		nil,
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	return rabbitChan, rabbitQueue, nil
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	return subscribe(conn, exchange, queueName, key, queueType, handler, func(data []byte) (T, error) {
		var val T
		err := json.Unmarshal(data, &val)
		return val, err
	})
}

func SubscribeGob[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	return subscribe(conn, exchange, queueName, key, queueType, handler, func(data []byte) (T, error) {
		var val T
		err := DecodeGob(data, &val)
		return val, err
	})
}

func subscribe[T any](conn *amqp.Connection, exchange, queueName, key string, simpleQueueType SimpleQueueType, handler func(T) AckType, unmarshaller func([]byte) (T, error),
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

type SimpleQueueType string

const (
	SimpleQueueTypeDurable   SimpleQueueType = "durable"
	SimpleQueueTypeTransient SimpleQueueType = "transient"
)

type Queue struct {
	Type SimpleQueueType
	Name string
}

type AckType string

const (
	AckTypeAck         AckType = "ack"
	AckTypeNackRequeue AckType = "nack_requeue"
	AckTypeNackDiscard AckType = "nack_discard"
)

type Ack struct {
	Type AckType
	Name string
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
