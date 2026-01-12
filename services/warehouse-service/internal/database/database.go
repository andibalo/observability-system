package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

func NewConnection(url string) (*sql.DB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func InitSchema(db *sql.DB) error {
	schema := `
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

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}
