package eventsrc

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// ErrConcurrency is returned when an event store operation fails due to
// a version mismatch, indicating a concurrent modification.
type ErrConcurrency struct {
	Msg string
}

func (e ErrConcurrency) Error() string {
	return e.Msg
}

// Store defines the interface for persisting and retrieving aggregates.
// The implementation is responsible for handling events and snapshots transparently.
type Store interface {
	// Save persists the uncommitted events of an aggregate. The implementation
	// is responsible for handling concurrency checks and deciding whether
	// to create a snapshot as part of the same atomic operation.
	Save(ctx context.Context, aggregate Aggregate) error

	// Load reconstructs an aggregate. The implementation should first try
	// to load the latest snapshot and then load all subsequent events.
	// It returns the snapshot payload (if any), the history of events since the snapshot,
	// and any error that occurred.
	Load(
		ctx context.Context,
		aggregateID uuid.UUID,
	) (snapshot json.RawMessage, version int, history []Event, err error)
}
