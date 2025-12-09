package handlers

import (
	"context"
	"sync"

	"observability-system/shared/logger"
	"order-service/internal/inbox"
)

type HandlerFunc func(ctx context.Context, msg inbox.InboxMessage) error

type MessageHandlerRegistry struct {
	log      logger.Logger
	handlers map[string]HandlerFunc
	mu       sync.RWMutex
}

func NewMessageHandlerRegistry(log logger.Logger) *MessageHandlerRegistry {
	return &MessageHandlerRegistry{
		log:      log,
		handlers: make(map[string]HandlerFunc),
	}
}

func (r *MessageHandlerRegistry) Register(eventType string, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[eventType] = handler
	r.log.Info("Registered message handler",
		logger.String("event_type", eventType))
}

func (r *MessageHandlerRegistry) HandleMessage(ctx context.Context, msg inbox.InboxMessage) error {
	r.mu.RLock()
	handler, exists := r.handlers[msg.EventType]
	r.mu.RUnlock()

	if !exists {
		r.log.Warn("No handler registered for event type",
			logger.String("event_type", msg.EventType),
			logger.String("message_id", msg.MessageID))
		return nil
	}

	r.log.Debug("Routing message to handler",
		logger.String("event_type", msg.EventType),
		logger.String("message_id", msg.MessageID))

	return handler(ctx, msg)
}

func (r *MessageHandlerRegistry) GetHandler() inbox.MessageHandler {
	return r.HandleMessage
}

func (r *MessageHandlerRegistry) ListRegisteredHandlers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make([]string, 0, len(r.handlers))
	for eventType := range r.handlers {
		handlers = append(handlers, eventType)
	}
	return handlers
}
