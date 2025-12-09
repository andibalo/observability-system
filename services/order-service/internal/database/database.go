package database

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// NewConnection creates a new database connection using sqlx
func NewConnection(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// InitSchema initializes all required database tables
func InitSchema(db *sqlx.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		customer_id VARCHAR(255) NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'pending',
		items JSONB NOT NULL,
		total_amount DECIMAL(10, 2) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS outbox (
		id SERIAL PRIMARY KEY,
		message_id VARCHAR(255) UNIQUE NOT NULL,
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		status VARCHAR(50) DEFAULT 'PENDING',
		retry_count INT DEFAULT 0,
		exchange VARCHAR(255) DEFAULT 'orders',
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

	CREATE TABLE IF NOT EXISTS inbox (
		id SERIAL PRIMARY KEY,
		message_id VARCHAR(255) UNIQUE NOT NULL,
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
		retry_count INT NOT NULL DEFAULT 0,
		error TEXT,
		locked_at TIMESTAMP,
		locked_by VARCHAR(255),
		http_status_code INT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_inbox_status ON inbox(status);
	CREATE INDEX IF NOT EXISTS idx_inbox_message_id ON inbox(message_id);
	CREATE INDEX IF NOT EXISTS idx_inbox_locked_at ON inbox(locked_at);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}
