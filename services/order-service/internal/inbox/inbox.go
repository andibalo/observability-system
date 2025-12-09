package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"observability-system/shared/logger"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type InboxMessage struct {
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
}

type InboxStore struct {
	db *sqlx.DB
}

func NewInboxStore(db *sqlx.DB) *InboxStore {
	return &InboxStore{db: db}
}

func (s *InboxStore) Save(ctx context.Context, messageID, eventType string, payload interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO inbox (message_id, event_type, payload, status)
		VALUES ($1, $2, $3, 'PENDING')
		ON CONFLICT (message_id) DO NOTHING
	`
	result, err := s.db.ExecContext(ctx, query, messageID, eventType, payloadJSON)
	if err != nil {
		return fmt.Errorf("failed to save inbox message: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("message already exists: %s", messageID)
	}

	return nil
}

func (s *InboxStore) GetByMessageID(ctx context.Context, messageID string) (*InboxMessage, error) {
	var msg InboxMessage
	query := `SELECT * FROM inbox WHERE message_id = $1`
	err := s.db.GetContext(ctx, &msg, query, messageID)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *InboxStore) GetAll(ctx context.Context) ([]InboxMessage, error) {
	var messages []InboxMessage
	query := `SELECT * FROM inbox ORDER BY created_at DESC LIMIT 100`
	err := s.db.SelectContext(ctx, &messages, query)
	if err != nil {
		return []InboxMessage{}, nil // Return empty slice on error
	}
	return messages, nil
}

func (s *InboxStore) GetPendingMessagesForProcessing(ctx context.Context, workerID string, batchSize int, maxRetries int) ([]InboxMessage, error) {
	query := `
		UPDATE inbox
		SET 
			status = 'PROCESSING',
			locked_at = NOW(),
			locked_by = $1,
			updated_at = NOW()
		WHERE id IN (
			SELECT id FROM inbox
			WHERE (status = 'PENDING' OR (status = 'FAILED' AND retry_count < $3))
			  AND (locked_at IS NULL OR locked_at < NOW() - INTERVAL '5 minutes')
			ORDER BY created_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, message_id, event_type, payload, status, created_at, updated_at, retry_count, locked_at, locked_by, error
	`

	var messages []InboxMessage
	err := s.db.SelectContext(ctx, &messages, query, workerID, batchSize, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages: %w", err)
	}

	return messages, nil
}

func (s *InboxStore) MarkAsProcessed(ctx context.Context, messageID int64) error {
	query := `
		UPDATE inbox
		SET status = 'PROCESSED',
			updated_at = NOW(),
			locked_at = NULL,
			locked_by = NULL
		WHERE id = $1
	`
	_, err := s.db.ExecContext(ctx, query, messageID)
	return err
}

func (s *InboxStore) IncrementRetryAndMarkPending(ctx context.Context, messageID int64, errorMsg string) error {
	query := `
		UPDATE inbox
		SET status = 'PENDING',
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

func (s *InboxStore) MarkAsFailed(ctx context.Context, messageID int64, errorMsg string) error {
	query := `
		UPDATE inbox
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

func (s *InboxStore) MessageExists(ctx context.Context, messageID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM inbox WHERE message_id = $1)`
	err := s.db.GetContext(ctx, &exists, query, messageID)
	return exists, err
}

func (s *InboxStore) ResetStuckMessages(ctx context.Context, timeoutMinutes int) (int64, error) {
	query := `
		UPDATE inbox
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

type MessageHandler func(ctx context.Context, msg InboxMessage) error

type InboxWorker struct {
	store      *InboxStore
	logger     logger.Logger
	workerID   string
	batchSize  int
	interval   time.Duration
	maxRetries int
	stopCh     chan struct{}
	handler    MessageHandler
}

func NewInboxWorker(
	store *InboxStore,
	handler MessageHandler,
	log logger.Logger,
	batchSize int,
	interval time.Duration,
	maxRetries int,
) *InboxWorker {
	return &InboxWorker{
		store:      store,
		logger:     log,
		workerID:   fmt.Sprintf("inbox-worker-%s", uuid.New().String()[:8]),
		batchSize:  batchSize,
		interval:   interval,
		maxRetries: maxRetries,
		stopCh:     make(chan struct{}),
		handler:    handler,
	}
}

func (w *InboxWorker) Start(ctx context.Context) {
	w.logger.Info("Starting inbox worker",
		logger.String("worker_id", w.workerID),
		logger.Int("batch_size", w.batchSize),
		logger.Int("max_retries", w.maxRetries),
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
			w.logger.Info("Stopping inbox worker due to context cancellation",
				logger.String("worker_id", w.workerID))
			return
		case <-w.stopCh:
			w.logger.Info("Inbox worker stopped",
				logger.String("worker_id", w.workerID))
			return
		case <-ticker.C:
			w.processMessages(ctx)
		}
	}
}

func (w *InboxWorker) Stop() {
	close(w.stopCh)
}

func (w *InboxWorker) processMessages(ctx context.Context) {
	messages, err := w.store.GetPendingMessagesForProcessing(ctx, w.workerID, w.batchSize, w.maxRetries)
	if err != nil {
		w.logger.Error("Failed to fetch pending messages",
			logger.Err(err),
			logger.String("worker_id", w.workerID))
		return
	}

	if len(messages) == 0 {
		return
	}

	w.logger.Info("Processing inbox messages",
		logger.Int("count", len(messages)),
		logger.String("worker_id", w.workerID))

	for _, msg := range messages {
		if err := w.handler(ctx, msg); err != nil {
			w.logger.Error("Failed to process message",
				logger.Err(err),
				logger.Int64("id", msg.ID),
				logger.String("message_id", msg.MessageID),
				logger.String("event_type", msg.EventType),
				logger.Int("retry_count", msg.RetryCount),
				logger.String("worker_id", w.workerID))

			if msg.RetryCount+1 >= w.maxRetries {
				w.logger.Warn("Max retries exceeded, marking as FAILED",
					logger.Int64("id", msg.ID),
					logger.String("message_id", msg.MessageID),
					logger.Int("retry_count", msg.RetryCount+1),
					logger.Int("max_retries", w.maxRetries))

				if err := w.store.MarkAsFailed(ctx, msg.ID, err.Error()); err != nil {
					w.logger.Error("Failed to mark message as failed",
						logger.Err(err),
						logger.Int64("id", msg.ID))
				}
			} else {
				w.logger.Info("Marking message for retry",
					logger.Int64("id", msg.ID),
					logger.String("message_id", msg.MessageID),
					logger.Int("retry_count", msg.RetryCount+1),
					logger.Int("max_retries", w.maxRetries))

				if err := w.store.IncrementRetryAndMarkPending(ctx, msg.ID, err.Error()); err != nil {
					w.logger.Error("Failed to mark message for retry",
						logger.Err(err),
						logger.Int64("id", msg.ID))
				}
			}
			continue
		}

		if err := w.store.MarkAsProcessed(ctx, msg.ID); err != nil {
			w.logger.Error("Failed to mark message as processed",
				logger.Err(err),
				logger.Int64("id", msg.ID))
		} else {
			w.logger.Info("Message processed successfully",
				logger.Int64("id", msg.ID),
				logger.String("message_id", msg.MessageID),
				logger.String("event_type", msg.EventType),
				logger.String("worker_id", w.workerID))
		}
	}
}
