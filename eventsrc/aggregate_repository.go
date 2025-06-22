package eventsrc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// Repository handles the loading and saving of event-sourced aggregates.
type Repository[T Aggregate] struct {
	eventStore        Store
	newEmptyAggregate func() T
}

// NewRepository creates a new generic repository for a specific aggregate type.
func NewRepository[T Aggregate](store Store, newEmptyAggregate func() T) *Repository[T] {
	return &Repository[T]{
		eventStore:        store,
		newEmptyAggregate: newEmptyAggregate,
	}
}

// Load retrieves an aggregate from the event store by its ID.
func (r *Repository[T]) Load(ctx context.Context, id uuid.UUID) (T, error) {
	aggregate := r.newEmptyAggregate()

	// The event store returns the snapshot payload, its version, and subsequent events.
	snapshotPayload, _, history, err := r.eventStore.Load(ctx, id)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to load aggregate %s from store: %w", id, err)
	}

	// 1. If a snapshot exists, unmarshal it into the aggregate first.
	if snapshotPayload != nil {
		if err := json.Unmarshal(snapshotPayload, &aggregate); err != nil {
			var zero T
			slog.ErrorContext(
				ctx,
				"Failed to unmarshal snapshot, cannot load aggregate",
				"aggregateID",
				id,
				"error",
				err,
			)
			return zero, fmt.Errorf("failed to unmarshal snapshot for aggregate %s: %w", id, err)
		}
	}

	// 2. Apply the remaining events from history. This will bring the aggregate
	// to its most recent state.
	aggregate.LoadFromHistory(ctx, history)

	return aggregate, nil
}

// Save persists the uncommitted events of an aggregate.
func (r *Repository[T]) Save(ctx context.Context, aggregate T) error {
	if len(aggregate.GetUncommittedEvents()) == 0 {
		return nil // Nothing to save
	}

	if err := r.eventStore.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save aggregate %s: %w", aggregate.ID(), err)
	}

	aggregate.ClearUncommittedEvents()

	return nil
}
