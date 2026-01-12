package outbox

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"observability-system/shared/messaging"

	"github.com/google/uuid"
)

type OutboxMessage struct {
	ID         int64
	MessageID  string
	EventType  string
	Payload    json.RawMessage
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	RetryCount int
	LockedAt   *time.Time
	LockedBy   *string
	Error      *string
	Exchange   string
	RoutingKey string
}

type OutboxStore struct {
	db *sql.DB
}

func NewOutboxStore(db *sql.DB) *OutboxStore {
	return &OutboxStore{db: db}
}

func (s *OutboxStore) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS outbox (
		id SERIAL PRIMARY KEY,
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
	CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox(status);
	CREATE INDEX IF NOT EXISTS idx_outbox_locked_at ON outbox(locked_at);
	CREATE INDEX IF NOT EXISTS idx_outbox_message_id ON outbox(message_id);

	-- Migration for existing tables (safe to run if columns exist)
	ALTER TABLE outbox ADD COLUMN IF NOT EXISTS message_id VARCHAR(255);
	ALTER TABLE outbox ADD COLUMN IF NOT EXISTS exchange VARCHAR(255) DEFAULT 'inventory';
	ALTER TABLE outbox ADD COLUMN IF NOT EXISTS routing_key VARCHAR(255);
	ALTER TABLE outbox ADD COLUMN IF NOT EXISTS error TEXT;
	ALTER TABLE outbox ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP;
	ALTER TABLE outbox ADD COLUMN IF NOT EXISTS locked_by VARCHAR(255);
	`
	_, err := s.db.Exec(query)

	return err
}

func (s *OutboxStore) Save(eventType string, payload interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	messageID := uuid.New().String()
	query := `
		INSERT INTO outbox (message_id, event_type, payload, status, exchange, routing_key)
		VALUES ($1, $2, $3, 'PENDING', 'inventory', $2)
	`
	_, err = s.db.Exec(query, messageID, eventType, payloadJSON)
	if err != nil {
		return fmt.Errorf("failed to save outbox message: %w", err)
	}

	log.Printf("Saved message to outbox: event_type=%s, message_id=%s", eventType, messageID)
	return nil
}

func (s *OutboxStore) GetPendingMessages(limit int) ([]OutboxMessage, error) {
	query := `
		SELECT id, message_id, event_type, payload, status, created_at, updated_at, retry_count, exchange, routing_key
		FROM outbox
		WHERE status = 'PENDING' OR status = 'pending'
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
		var messageID sql.NullString
		var exchange sql.NullString
		var routingKey sql.NullString

		err := rows.Scan(&msg.ID, &messageID, &msg.EventType, &msg.Payload, &msg.Status, &msg.CreatedAt, &msg.UpdatedAt, &msg.RetryCount, &exchange, &routingKey)
		if err != nil {
			return nil, err
		}
		if messageID.Valid {
			msg.MessageID = messageID.String
		}
		if exchange.Valid {
			msg.Exchange = exchange.String
		}
		if routingKey.Valid {
			msg.RoutingKey = routingKey.String
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func (s *OutboxStore) MarkAsPublished(id int64) error {
	query := `
		UPDATE outbox
		SET status = 'published', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := s.db.Exec(query, id)
	return err
}

func (s *OutboxStore) MarkAsFailed(id int64) error {
	query := `
		UPDATE outbox
		SET status = 'failed', retry_count = retry_count + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := s.db.Exec(query, id)
	return err
}

type OutboxProcessor struct {
	store     *OutboxStore
	publisher messaging.Publisher
}

func NewOutboxProcessor(store *OutboxStore, publisher messaging.Publisher) *OutboxProcessor {
	return &OutboxProcessor{
		store:     store,
		publisher: publisher,
	}
}

func (p *OutboxProcessor) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			p.ProcessMessages()
		}
	}()
	log.Printf("Outbox processor started with interval: %v", interval)
}

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

		msgID := msg.MessageID
		if msgID == "" {
			msgID = fmt.Sprintf("%d", msg.ID)
		}

		message := messaging.Message{
			ID:        msgID,
			Type:      msg.EventType,
			Payload:   payload,
			Timestamp: msg.CreatedAt,
		}

		exchange := msg.Exchange
		if exchange == "" {
			exchange = "inventory"
		}
		routingKey := msg.RoutingKey

		if routingKey == "" {
			routingKey = msg.EventType
		}

		err := p.publisher.Publish(exchange, routingKey, message)
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
