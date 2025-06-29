package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/0m3kk/eventus/eventsrc"
)

// OutboxStore implements the outbox.Store interface for PostgreSQL.
type OutboxStore struct {
	db *DB
}

func NewOutboxStore(db *DB) *OutboxStore {
	return &OutboxStore{db: db}
}

// ProcessOutboxBatch handles the entire lifecycle of fetching, processing,
// and marking outbox events as published within a single, robust transaction.
func (s *OutboxStore) ProcessOutboxBatch(
	ctx context.Context,
	batchSize int,
	processFunc func(ctx context.Context, events []eventsrc.OutboxEvent) error,
) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for processing outbox batch: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Fetch and lock a batch of events within the transaction.
	events, err := fetchAndLockUnpublishedInTx(ctx, tx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to fetch and lock events: %w", err)
	}

	if len(events) == 0 {
		return nil // Nothing to do, commit the empty transaction.
	}

	// 2. Execute the provided processing logic (e.g., publishing to a broker).
	// If this function returns an error, the transaction will be rolled back.
	if err := processFunc(ctx, events); err != nil {
		return fmt.Errorf("event processing function failed: %w", err)
	}

	// 3. If processing was successful, mark the events as published.
	if err := markAsPublishedInTx(ctx, tx, events); err != nil {
		return fmt.Errorf("failed to mark events as published: %w", err)
	}

	// 4. Commit the transaction to finalize all changes.
	return tx.Commit(ctx)
}

// fetchAndLockUnpublishedInTx is an internal helper that performs the SELECT ... FOR UPDATE.
func fetchAndLockUnpublishedInTx(ctx context.Context, tx pgx.Tx, batchSize int) ([]eventsrc.OutboxEvent, error) {
	query := `
        SELECT event_id, aggregate_id, aggregate_type, event_type, payload, version, ts
        FROM outbox
        WHERE published = FALSE
        ORDER BY ts
        LIMIT $1
        FOR UPDATE SKIP LOCKED
    `
	rows, err := tx.Query(ctx, query, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query outbox: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByPos[eventsrc.OutboxEvent])
}

// markAsPublishedInTx is an internal helper that updates the `published` flag.
func markAsPublishedInTx(ctx context.Context, tx pgx.Tx, events []eventsrc.OutboxEvent) error {
	eventIDs := make([]uuid.UUID, len(events))
	for i, e := range events {
		eventIDs[i] = e.EventID
	}

	query := `UPDATE outbox SET published = TRUE WHERE event_id = ANY($1)`
	cmdTag, err := tx.Exec(ctx, query, eventIDs)
	if err != nil {
		return fmt.Errorf("failed to execute update for marking events as published: %w", err)
	}

	if cmdTag.RowsAffected() != int64(len(eventIDs)) {
		return fmt.Errorf(
			"consistency error: expected to mark %d events, but marked %d",
			len(eventIDs),
			cmdTag.RowsAffected(),
		)
	}

	return nil
}

// SaveEvents saves events to the outbox table. It expects to be run within a transaction.
func (s *OutboxStore) SaveEvents(ctx context.Context, events []eventsrc.Event) error {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return fmt.Errorf("SaveEvents must be called within a transaction")
	}

	b := &pgx.Batch{}
	stmt := `
        INSERT INTO outbox (event_id, aggregate_id, aggregate_type, event_type, payload, version, ts)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `
	for _, evt := range events {
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("failed to marshal event payload for event %s: %w", evt.EventID(), err)
		}
		b.Queue(
			stmt,
			evt.EventID(),
			evt.AggregateID(),
			evt.AggregateType(),
			evt.EventType(),
			payload,
			evt.Version(),
			evt.Timestamp(),
		)
	}

	br := tx.SendBatch(ctx, b)
	defer br.Close()

	// Check for errors from each queued command
	for i := range len(events) {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to insert event #%d into outbox batch: %w", i+1, err)
		}
	}

	return br.Close()
}
