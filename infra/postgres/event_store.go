package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/0m3kk/eventus/eventsrc"
)

// Store implements the event.Store interface for PostgreSQL.
type Store struct {
	db                *DB
	outbox            *OutboxStore
	snapshotFrequency int
}

// NewEventStore creates a new PostgreSQL event store.
func NewEventStore(db *DB, outbox *OutboxStore, snapshotFrequency int) *Store {
	return &Store{
		db:                db,
		outbox:            outbox,
		snapshotFrequency: snapshotFrequency,
	}
}

// Save persists an aggregate's uncommitted events and potentially a snapshot.
func (s *Store) Save(ctx context.Context, aggregate eventsrc.Aggregate) error {
	events := aggregate.GetUncommittedEvents()
	if len(events) == 0 {
		return nil
	}

	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return fmt.Errorf("Save must be called within a transaction")
	}

	if err := s.saveEventsInTx(ctx, tx, events); err != nil {
		return err
	}
	if err := s.outbox.SaveEvents(ctx, events); err != nil {
		return err
	}

	if s.shouldSnapshot(aggregate) {
		if err := s.saveSnapshotInTx(ctx, tx, aggregate); err != nil {
			slog.ErrorContext(ctx, "Failed to save snapshot", "aggregateID", aggregate.ID(), "error", err)
		} else {
			slog.InfoContext(ctx, "Snapshot saved successfully", "aggregateID", aggregate.ID(), "version", aggregate.Version())
		}
	}

	return nil
}

// Load reconstructs an aggregate by loading its latest snapshot and subsequent events.
func (s *Store) Load(
	ctx context.Context,
	aggType eventsrc.AggregateType,
	aggID uuid.UUID,
) (json.RawMessage, int, []eventsrc.Event, error) {
	snapshot, version, err := s.loadSnapshot(ctx, aggType, aggID)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to load snapshot: %w", err)
	}

	events, err := s.loadEvents(ctx, aggType, aggID, version)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to load events: %w", err)
	}

	return snapshot, version, events, nil
}

func (s *Store) saveEventsInTx(ctx context.Context, tx pgx.Tx, events []eventsrc.Event) error {
	b := &pgx.Batch{}
	stmt := `
        INSERT INTO event_store (event_id, aggregate_id, aggregate_type, event_type, payload, version)
        VALUES ($1, $2, $3, $4, $5, $6)
    `
	for _, evt := range events {
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("failed to marshal event payload: %w", err)
		}
		b.Queue(stmt, evt.EventID(), evt.AggregateID(), evt.AggregateType(), evt.EventType(), payload, evt.Version())
	}

	br := tx.SendBatch(ctx, b)
	defer br.Close()

	for range len(events) {
		if _, err := br.Exec(); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
				return eventsrc.ErrConcurrency{Msg: fmt.Sprintf("concurrency error: %s", err.Error())}
			}
			return fmt.Errorf("failed to insert event into batch: %w", err)
		}
	}
	return br.Close()
}

func (s *Store) loadEvents(
	ctx context.Context,
	aggType eventsrc.AggregateType,
	aggID uuid.UUID,
	fromVersion int,
) ([]eventsrc.Event, error) {
	query := `
        SELECT event_type, payload
        FROM event_store
        WHERE aggregate_id = $1 AND version > $2
        ORDER BY version ASC
    `
	rows, err := s.db.Pool.Query(ctx, query, aggID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []eventsrc.Event
	for rows.Next() {
		var eventType string
		var payload json.RawMessage
		if err := rows.Scan(&eventType, &payload); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		evtData, err := eventsrc.CreateEvent(eventType)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, evtData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event payload: %w", err)
		}
		events = append(events, evtData)
	}
	return events, rows.Err()
}

func (s *Store) shouldSnapshot(aggregate eventsrc.Aggregate) bool {
	if s.snapshotFrequency <= 0 {
		return false
	}
	return aggregate.Version()%s.snapshotFrequency == 0
}

func (s *Store) saveSnapshotInTx(ctx context.Context, tx pgx.Tx, aggregate eventsrc.Aggregate) error {
	payload, err := json.Marshal(aggregate)
	if err != nil {
		return fmt.Errorf("failed to marshal aggregate for snapshot: %w", err)
	}

	query := `
        INSERT INTO snapshots (aggregate_id, aggregate_type, aggregate_version, payload)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (aggregate_id, aggregate_version) DO NOTHING
    `
	_, err = tx.Exec(ctx, query, aggregate.ID(), aggregate.AggregateType(), aggregate.Version(), payload)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}
	return nil
}

func (s *Store) loadSnapshot(
	ctx context.Context,
	aggType eventsrc.AggregateType,
	aggID uuid.UUID,
) (json.RawMessage, int, error) {
	query := `
        SELECT aggregate_version, payload
        FROM snapshots
        WHERE aggregate_id = $1
        ORDER BY aggregate_version DESC
        LIMIT 1
    `
	var version int
	var payload json.RawMessage
	err := s.db.Pool.QueryRow(ctx, query, aggID).Scan(&version, &payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, nil // No snapshot found, not an error
		}
		return nil, 0, fmt.Errorf("failed to load snapshot: %w", err)
	}
	return payload, version, nil
}
