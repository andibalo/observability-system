package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"observability-system/shared/logger"
	"order-service/internal/inbox"
)

type OrderEventHandler struct {
	log logger.Logger
}

func NewOrderEventHandler(log logger.Logger) *OrderEventHandler {
	return &OrderEventHandler{
		log: log,
	}
}

func (h *OrderEventHandler) HandleOrderCreated(ctx context.Context, msg inbox.InboxMessage) error {
	var payload struct {
		OrderID    string  `json:"order_id"`
		CustomerID string  `json:"customer_id"`
		Amount     float64 `json:"amount"`
		Items      []struct {
			SKU      string  `json:"sku"`
			Quantity int     `json:"quantity"`
			Price    float64 `json:"price"`
		} `json:"items"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal order.created payload: %w", err)
	}

	h.log.Info("Processing order created event",
		logger.String("message_id", msg.MessageID),
		logger.String("order_id", payload.OrderID),
		logger.String("customer_id", payload.CustomerID),
		logger.String("amount", fmt.Sprintf("%.2f", payload.Amount)))

	// TODO: Implement your business logic

	h.log.Info("Successfully processed order created event",
		logger.String("order_id", payload.OrderID))

	return nil
}

func (h *OrderEventHandler) HandleOrderUpdated(ctx context.Context, msg inbox.InboxMessage) error {
	var payload struct {
		OrderID   string            `json:"order_id"`
		Status    string            `json:"status"`
		UpdatedBy string            `json:"updated_by"`
		Changes   map[string]string `json:"changes"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal order.updated payload: %w", err)
	}

	h.log.Info("Processing order updated event",
		logger.String("message_id", msg.MessageID),
		logger.String("order_id", payload.OrderID),
		logger.String("status", payload.Status))

	// TODO: Implement your business logic

	h.log.Info("Successfully processed order updated event",
		logger.String("order_id", payload.OrderID))

	return nil
}

func (h *OrderEventHandler) HandleOrderCancelled(ctx context.Context, msg inbox.InboxMessage) error {
	var payload struct {
		OrderID      string  `json:"order_id"`
		Reason       string  `json:"reason"`
		CancelledBy  string  `json:"cancelled_by"`
		RefundAmount float64 `json:"refund_amount"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal order.cancelled payload: %w", err)
	}

	h.log.Info("Processing order cancelled event",
		logger.String("message_id", msg.MessageID),
		logger.String("order_id", payload.OrderID),
		logger.String("reason", payload.Reason),
		logger.String("refund_amount", fmt.Sprintf("%.2f", payload.RefundAmount)))

	// TODO: Implement your business logic

	h.log.Info("Successfully processed order cancelled event",
		logger.String("order_id", payload.OrderID))

	return nil
}
