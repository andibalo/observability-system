package outbox

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"observability-system/shared/messaging"
)

// OutboxMessage represents a message in the outbox table
type OutboxMessage struct {
	ID         int64
	EventType  string
	Payload    json.RawMessage
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	RetryCount int
}

// OutboxStore handles outbox operations
type OutboxStore struct {
	db *sql.DB
}

// NewOutboxStore creates a new outbox store
func NewOutboxStore(db *sql.DB) *OutboxStore {
	return &OutboxStore{db: db}
}

// InitSchema creates the outbox table
func (s *OutboxStore) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS outbox (
		id SERIAL PRIMARY KEY,
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		status VARCHAR(50) DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		retry_count INT DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox(status);
	CREATE INDEX IF NOT EXISTS idx_outbox_created_at ON outbox(created_at);
	`
	_, err := s.db.Exec(query)
	return err
}

// Save saves a message to the outbox
func (s *OutboxStore) Save(eventType string, payload interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO outbox (event_type, payload, status)
		VALUES ($1, $2, 'pending')
	`
	_, err = s.db.Exec(query, eventType, payloadJSON)
	if err != nil {
		return fmt.Errorf("failed to save outbox message: %w", err)
	}

	log.Printf("Saved message to outbox: event_type=%s", eventType)
	return nil
}

// GetPendingMessages retrieves pending messages from the outbox
func (s *OutboxStore) GetPendingMessages(limit int) ([]OutboxMessage, error) {
	query := `
		SELECT id, event_type, payload, status, created_at, updated_at, retry_count
		FROM outbox
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []OutboxMessage
	for rows.Next() {
		var msg OutboxMessage
		err := rows.Scan(&msg.ID, &msg.EventType, &msg.Payload, &msg.Status, &msg.CreatedAt, &msg.UpdatedAt, &msg.RetryCount)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// MarkAsPublished marks a message as published
func (s *OutboxStore) MarkAsPublished(id int64) error {
	query := `
		UPDATE outbox
		SET status = 'published', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := s.db.Exec(query, id)
	return err
}

// MarkAsFailed marks a message as failed
func (s *OutboxStore) MarkAsFailed(id int64) error {
	query := `
		UPDATE outbox
		SET status = 'failed', retry_count = retry_count + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := s.db.Exec(query, id)
	return err
}

// OutboxProcessor processes outbox messages
type OutboxProcessor struct {
	store     *OutboxStore
	publisher messaging.Publisher
}

// NewOutboxProcessor creates a new outbox processor
func NewOutboxProcessor(store *OutboxStore, publisher messaging.Publisher) *OutboxProcessor {
	return &OutboxProcessor{
		store:     store,
		publisher: publisher,
	}
}

// Start starts the outbox processor
func (p *OutboxProcessor) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			p.ProcessMessages()
		}
	}()
	log.Printf("Outbox processor started with interval: %v", interval)
}

// ProcessMessages processes pending outbox messages
func (p *OutboxProcessor) ProcessMessages() {
	messages, err := p.store.GetPendingMessages(10)
	if err != nil {
		log.Printf("Failed to get pending messages: %v", err)
		return
	}

	for _, msg := range messages {
		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("Failed to unmarshal payload for message %d: %v", msg.ID, err)
			continue
		}

		message := messaging.Message{
			ID:        fmt.Sprintf("%d", msg.ID),
			Type:      msg.EventType,
			Payload:   payload,
			Timestamp: msg.CreatedAt,
		}

		exchange := "inventory" // Default exchange for warehouse service
		err := p.publisher.Publish(exchange, msg.EventType, message)
		if err != nil {
			log.Printf("Failed to publish message %d: %v", msg.ID, err)
			p.store.MarkAsFailed(msg.ID)
			continue
		}

		if err := p.store.MarkAsPublished(msg.ID); err != nil {
			log.Printf("Failed to mark message %d as published: %v", msg.ID, err)
		}
	}
}
