package rabbitmq

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"observability-system/shared/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	url     string
}

// NewClient creates a new RabbitMQ client
func NewClient(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	client := &Client{
		conn:    conn,
		channel: channel,
		url:     url,
	}

	log.Println("Successfully connected to RabbitMQ")
	return client, nil
}

// Publish publishes a message to an exchange
func (c *Client) Publish(exchange, routingKey string, msg messaging.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = c.channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.Printf("Published message to exchange=%s, routingKey=%s", exchange, routingKey)
	return nil
}

// Subscribe subscribes to a queue and processes messages
func (c *Client) Subscribe(queue string, handler messaging.MessageHandler) error {
	msgs, err := c.channel.Consume(
		queue, // queue
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	log.Printf("Subscribed to queue: %s", queue)

	go func() {
		for d := range msgs {
			var msg messaging.Message
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				d.Nack(false, false) // Reject message
				continue
			}

			if err := handler(msg); err != nil {
				log.Printf("Failed to handle message: %v", err)
				d.Nack(false, true) // Requeue message
				continue
			}

			d.Ack(false) // Acknowledge message
		}
	}()

	return nil
}

// DeclareExchange declares an exchange
func (c *Client) DeclareExchange(name, kind string) error {
	return c.channel.ExchangeDeclare(
		name,  // name
		kind,  // type
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
}

// DeclareQueue declares a queue
func (c *Client) DeclareQueue(name string) error {
	_, err := c.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	return err
}

// BindQueue binds a queue to an exchange
func (c *Client) BindQueue(queue, exchange, routingKey string) error {
	return c.channel.QueueBind(
		queue,      // queue name
		routingKey, // routing key
		exchange,   // exchange
		false,
		nil,
	)
}

// Close closes the RabbitMQ connection
func (c *Client) Close() error {
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			return err
		}
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
