package messaging

import "time"

// Message represents a generic message structure
type Message struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
}

// MessageHandler is a function that processes incoming messages
type MessageHandler func(msg Message) error

// Publisher defines the interface for publishing messages
type Publisher interface {
	Publish(exchange, routingKey string, msg Message) error
	Close() error
}

// Consumer defines the interface for consuming messages
type Consumer interface {
	Subscribe(queue string, handler MessageHandler) error
	Close() error
}

// MessageBroker combines publisher and consumer interfaces
type MessageBroker interface {
	Publisher
	Consumer
}
