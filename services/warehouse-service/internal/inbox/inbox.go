package inbox

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"observability-system/shared/messaging"
)

// InboxMessage represents a message in the inbox table
type InboxMessage struct {
	ID         int64
	MessageID  string
	EventType  string
	Payload    json.RawMessage
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	RetryCount int
	SenderID   string
	Exchange   string
	RoutingKey string
	Error      *string
	LockedAt   *time.Time
	LockedBy   *string
}

// InboxStore handles inbox operations
type InboxStore struct {
	db *sql.DB
}

// NewInboxStore creates a new inbox store
func NewInboxStore(db *sql.DB) *InboxStore {
	return &InboxStore{db: db}
}

// InitSchema creates the inbox table
func (s *InboxStore) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS inbox (
		id SERIAL PRIMARY KEY,
		sender_id VARCHAR(255) NOT NULL,
		message_id VARCHAR(255) UNIQUE NOT NULL,
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		status VARCHAR(50) DEFAULT 'PENDING',
		retry_count INT DEFAULT 0,
		exchange VARCHAR(255) DEFAULT 'inventory',
		routing_key VARCHAR(255),
		error TEXT,
		locked_at TIMESTAMP,
		locked_by VARCHAR(255),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_inbox_status ON inbox(status);
	CREATE INDEX IF NOT EXISTS idx_inbox_message_id ON inbox(message_id);
	CREATE INDEX IF NOT EXISTS idx_inbox_locked_at ON inbox(locked_at);

	-- Migration for existing tables
	ALTER TABLE inbox ADD COLUMN IF NOT EXISTS sender_id VARCHAR(255) DEFAULT 'unknown';
	ALTER TABLE inbox ADD COLUMN IF NOT EXISTS exchange VARCHAR(255) DEFAULT 'inventory';
	ALTER TABLE inbox ADD COLUMN IF NOT EXISTS routing_key VARCHAR(255);
	ALTER TABLE inbox ADD COLUMN IF NOT EXISTS error TEXT;
	ALTER TABLE inbox ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP;
	ALTER TABLE inbox ADD COLUMN IF NOT EXISTS locked_by VARCHAR(255);
	`
	_, err := s.db.Exec(query)
	return err
}

// Save saves a message to the inbox
func (s *InboxStore) Save(messageID, eventType string, payload interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Assuming sender is unknown if not provided in interface (changing signature would break callers?)
	// I'll keep signature same for now but defaulting sender_id
	senderID := "unknown"

	query := `
		INSERT INTO inbox (message_id, event_type, payload, status, sender_id, exchange)
		VALUES ($1, $2, $3, 'PENDING', $4, 'inventory')
		ON CONFLICT (message_id) DO NOTHING
	`
	result, err := s.db.Exec(query, messageID, eventType, payloadJSON, senderID)
	if err != nil {
		return fmt.Errorf("failed to save inbox message: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Saved message to inbox: message_id=%s, event_type=%s", messageID, eventType)
	} else {
		log.Printf("Message already exists in inbox: message_id=%s", messageID)
	}

	return nil
}

// MarkAsProcessed marks a message as processed
func (s *InboxStore) MarkAsProcessed(messageID string) error {
	query := `
		UPDATE inbox
		SET status = 'processed', updated_at = CURRENT_TIMESTAMP
		WHERE message_id = $1
	`
	_, err := s.db.Exec(query, messageID)
	return err
}

// MarkAsFailed marks a message as failed
func (s *InboxStore) MarkAsFailed(messageID string) error {
	query := `
		UPDATE inbox
		SET status = 'failed', retry_count = retry_count + 1, updated_at = CURRENT_TIMESTAMP
		WHERE message_id = $1
	`
	_, err := s.db.Exec(query, messageID)
	return err
}

// MessageExists checks if a message already exists
func (s *InboxStore) MessageExists(messageID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM inbox WHERE message_id = $1)`
	err := s.db.QueryRow(query, messageID).Scan(&exists)
	return exists, err
}

// InboxHandler creates a message handler with inbox pattern
func InboxHandler(store *InboxStore, handler func(msg messaging.Message) error) messaging.MessageHandler {
	return func(msg messaging.Message) error {
		// Check if message already exists
		exists, err := store.MessageExists(msg.ID)
		if err != nil {
			return fmt.Errorf("failed to check message existence: %w", err)
		}

		if exists {
			log.Printf("Message already processed: %s", msg.ID)
			return nil
		}

		// Save to inbox
		if err := store.Save(msg.ID, msg.Type, msg.Payload); err != nil {
			return err
		}

		// Process the message
		if err := handler(msg); err != nil {
			store.MarkAsFailed(msg.ID)
			return err
		}

		// Mark as processed
		return store.MarkAsProcessed(msg.ID)
	}
}
