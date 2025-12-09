package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"observability-system/shared/logger"
	"observability-system/shared/messaging"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// OutboxMessage represents a message in the outbox table
type OutboxMessage struct {
	ID         int64           `db:"id" json:"id"`
	MessageID  string          `db:"message_id" json:"message_id"`
	EventType  string          `db:"event_type" json:"event_type"`
	Payload    json.RawMessage `db:"payload" json:"payload"`
	Status     string          `db:"status" json:"status"`
	CreatedAt  time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at" json:"updated_at"`
	RetryCount int             `db:"retry_count" json:"retry_count"`
	LockedAt   *time.Time      `db:"locked_at" json:"locked_at,omitempty"`
	LockedBy   *string         `db:"locked_by" json:"locked_by,omitempty"`
	Error      *string         `db:"error" json:"error,omitempty"`
	Exchange   string          `db:"exchange" json:"exchange"`
	RoutingKey string          `db:"routing_key" json:"routing_key"`
}

// OutboxStore handles outbox operations using sqlx
type OutboxStore struct {
	db *sqlx.DB
}

// NewOutboxStore creates a new outbox store
func NewOutboxStore(db *sqlx.DB) *OutboxStore {
	return &OutboxStore{db: db}
}

// Save saves a message to the outbox
func (s *OutboxStore) Save(ctx context.Context, eventType string, payload interface{}) (string, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	messageID := uuid.New().String()
	query := `
		INSERT INTO outbox (message_id, event_type, payload, status)
		VALUES ($1, $2, $3, 'PENDING')
	`
	_, err = s.db.ExecContext(ctx, query, messageID, eventType, payloadJSON)
	if err != nil {
		return "", fmt.Errorf("failed to save outbox message: %w", err)
	}

	return messageID, nil
}

// GetPendingMessagesForProcessing gets messages with pessimistic locking
// Uses FOR UPDATE SKIP LOCKED to allow concurrent workers
func (s *OutboxStore) GetPendingMessagesForProcessing(ctx context.Context, workerID string, batchSize int) ([]OutboxMessage, error) {
	query := `
		UPDATE outbox
		SET 
			status = 'PROCESSING',
			locked_at = NOW(),
			locked_by = $1,
			updated_at = NOW()
		WHERE id IN (
			SELECT id FROM outbox
			WHERE status = 'PENDING'
			  AND (locked_at IS NULL OR locked_at < NOW() - INTERVAL '5 minutes')
			ORDER BY created_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, message_id, event_type, payload, status, created_at, updated_at, retry_count, locked_at, locked_by, error
	`

	var messages []OutboxMessage
	err := s.db.SelectContext(ctx, &messages, query, workerID, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages: %w", err)
	}

	return messages, nil
}

// MarkAsPublished marks a message as published
func (s *OutboxStore) MarkAsPublished(ctx context.Context, messageID int64) error {
	query := `
		UPDATE outbox
		SET status = 'PUBLISHED',
			updated_at = NOW(),
			locked_at = NULL,
			locked_by = NULL
		WHERE id = $1
	`
	_, err := s.db.ExecContext(ctx, query, messageID)
	return err
}

// MarkAsFailed marks a message as failed and increments retry count
func (s *OutboxStore) MarkAsFailed(ctx context.Context, messageID int64, errorMsg string) error {
	query := `
		UPDATE outbox
		SET status = 'FAILED',
			retry_count = retry_count + 1,
			updated_at = NOW(),
			locked_at = NULL,
			locked_by = NULL,
			error = $2
		WHERE id = $1
	`
	_, err := s.db.ExecContext(ctx, query, messageID, errorMsg)
	return err
}

// ResetStuckMessages resets messages that have been locked too long
func (s *OutboxStore) ResetStuckMessages(ctx context.Context, timeoutMinutes int) (int64, error) {
	query := `
		UPDATE outbox
		SET status = 'PENDING',
			locked_at = NULL,
			locked_by = NULL,
			updated_at = NOW()
		WHERE status = 'PROCESSING'
		  AND locked_at < NOW() - INTERVAL '1 minute' * $1
	`

	result, err := s.db.ExecContext(ctx, query, timeoutMinutes)
	if err != nil {
		return 0, fmt.Errorf("failed to reset stuck messages: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

// OutboxWorker processes outbox messages concurrently
type OutboxWorker struct {
	store     *OutboxStore
	logger    logger.Logger
	workerID  string
	batchSize int
	interval  time.Duration
	stopCh    chan struct{}
	publisher messaging.Publisher
}

// NewOutboxWorker creates a new outbox worker
func NewOutboxWorker(
	store *OutboxStore,
	publisher messaging.Publisher,
	log logger.Logger,
	batchSize int,
	interval time.Duration,
) *OutboxWorker {
	return &OutboxWorker{
		store:     store,
		logger:    log,
		workerID:  fmt.Sprintf("outbox-worker-%s", uuid.New().String()[:8]),
		batchSize: batchSize,
		interval:  interval,
		stopCh:    make(chan struct{}),
		publisher: publisher,
	}
}

// Start begins processing messages
func (w *OutboxWorker) Start(ctx context.Context) {
	w.logger.Info("Starting outbox worker",
		logger.String("worker_id", w.workerID),
		logger.Int("batch_size", w.batchSize),
		logger.String("interval", w.interval.String()))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Reset stuck messages on startup
	if count, err := w.store.ResetStuckMessages(ctx, 5); err != nil {
		w.logger.Error("Failed to reset stuck messages", logger.Err(err))
	} else if count > 0 {
		w.logger.Info("Reset stuck messages", logger.Int64("count", count))
	}

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Stopping outbox worker due to context cancellation",
				logger.String("worker_id", w.workerID))
			return
		case <-w.stopCh:
			w.logger.Info("Outbox worker stopped",
				logger.String("worker_id", w.workerID))
			return
		case <-ticker.C:
			w.processMessages(ctx)
		}
	}
}

// Stop gracefully stops the worker
func (w *OutboxWorker) Stop() {
	close(w.stopCh)
}

func (w *OutboxWorker) processMessages(ctx context.Context) {
	messages, err := w.store.GetPendingMessagesForProcessing(ctx, w.workerID, w.batchSize)
	if err != nil {
		w.logger.Error("Failed to fetch pending messages",
			logger.Err(err),
			logger.String("worker_id", w.workerID))
		return
	}

	if len(messages) == 0 {
		return
	}

	w.logger.Info("Processing outbox messages",
		logger.Int("count", len(messages)),
		logger.String("worker_id", w.workerID))

	for _, msg := range messages {
		if err := w.processMessage(ctx, msg); err != nil {
			w.logger.Error("Failed to process message",
				logger.Err(err),
				logger.Int64("id", msg.ID),
				logger.String("message_id", msg.MessageID),
				logger.String("event_type", msg.EventType),
				logger.String("worker_id", w.workerID))

			// Mark as failed
			if err := w.store.MarkAsFailed(ctx, msg.ID, err.Error()); err != nil {
				w.logger.Error("Failed to mark message as failed",
					logger.Err(err),
					logger.Int64("id", msg.ID))
			}
			continue
		}

		// Mark as published
		if err := w.store.MarkAsPublished(ctx, msg.ID); err != nil {
			w.logger.Error("Failed to mark message as published",
				logger.Err(err),
				logger.Int64("id", msg.ID))
		} else {
			w.logger.Info("Message published successfully",
				logger.Int64("id", msg.ID),
				logger.String("message_id", msg.MessageID),
				logger.String("event_type", msg.EventType),
				logger.String("worker_id", w.workerID))
		}
	}
}

func (w *OutboxWorker) processMessage(ctx context.Context, msg OutboxMessage) error {
	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Create message for publisher
	message := messaging.Message{
		ID:        msg.MessageID,
		Type:      msg.EventType,
		Payload:   payload,
		Timestamp: msg.CreatedAt,
	}

	// Use exchange and routing key from the message
	exchange := msg.Exchange
	routingKey := msg.RoutingKey

	// Fallback to defaults if not set
	if exchange == "" {
		exchange = "orders"
	}
	if routingKey == "" {
		routingKey = msg.EventType
	}

	// Publish to message broker
	if err := w.publisher.Publish(exchange, routingKey, message); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}
