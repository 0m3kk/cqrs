package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// IdempotencyStore implements the handler.IdempotencyStore for PostgreSQL.
type IdempotencyStore struct {
	db *DB
}

func NewIdempotencyStore(db *DB) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

// IsProcessed checks if an event has already been processed by a subscriber.
func (s *IdempotencyStore) IsProcessed(ctx context.Context, eventID uuid.UUID, subscriberID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1 AND subscriber_id = $2)`
	err := s.db.Pool.QueryRow(ctx, query, eventID, subscriberID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check for processed event: %w", err)
	}
	return exists, nil
}

// MarkAsProcessed marks an event as processed. It expects to be called within a transaction.
func (s *IdempotencyStore) MarkAsProcessed(ctx context.Context, eventID uuid.UUID, subscriberID string) error {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return fmt.Errorf("MarkAsProcessed must be called within a transaction")
	}

	query := `INSERT INTO processed_events (event_id, subscriber_id) VALUES ($1, $2)`
	_, err := tx.Exec(ctx, query, eventID, subscriberID)
	if err != nil {
		var pgErr *pgconn.PgError
		// "23505" is the unique_violation error code in PostgreSQL.
		// If we get this, it means another concurrent process just processed the same event.
		// This is a legitimate scenario, not an error. We can safely ignore it.
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil // The event was processed concurrently, which is fine.
		}
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}
	return nil
}
