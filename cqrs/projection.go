package cqrs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/google/uuid"

	"github.com/0m3kk/eventus/eventsrc"
)

// ErrOutOfOrderEvent is returned when an event is received with a version that is not the expected next version.
var ErrOutOfOrderEvent = errors.New("out of order event")

// IdempotencyStore defines the interface for checking and storing processed event IDs.
type IdempotencyStore interface {
	IsProcessed(ctx context.Context, eventID uuid.UUID, subscriberID string) (bool, error)
	MarkAsProcessed(ctx context.Context, eventID uuid.UUID, subscriberID string) error
}

// VersionedStore defines an interface for read models that support versioning.
type VersionedStore interface {
	// GetVersion retrieves the current version of an aggregate's view model.
	// It should return 0 if the view model does not exist yet.
	GetVersion(ctx context.Context, aggregateID uuid.UUID) (int, error)
}

// TransactionalHandler defines a function that executes business logic within a transaction.
type TransactionalHandler func(ctx context.Context) error

// Transactor defines an interface for an object that can execute a function within a transaction.
type Transactor interface {
	WithTransaction(ctx context.Context, fn TransactionalHandler) error
}

// ProjectionHandler main logic for projection view.
type ProjectionHandler func(ctx context.Context, evt eventsrc.OutboxEvent) error

// Projection is a decorator that wraps a business logic handler
// with idempotency checks and retry logic.
type Projection struct {
	subscriberID   string
	idempStore     IdempotencyStore
	versionStore   VersionedStore // Store for checking view model version
	transactor     Transactor
	handler        ProjectionHandler
	maxElapsedTime time.Duration
}

// ProjectionOption is a function that configures an IdempotentEventHandler.
type ProjectionOption func(*Projection)

// WithMaxElapsedTime is an option to provide a custom backoff max elapsed time.
func WithMaxElapsedTime(maxElapsedTime time.Duration) ProjectionOption {
	return func(h *Projection) {
		h.maxElapsedTime = maxElapsedTime
	}
}

// NewProjection creates a new idempotent event handler.
func NewProjection(
	subscriberID string,
	idempStore IdempotencyStore,
	versionStore VersionedStore,
	transactor Transactor,
	handler func(ctx context.Context, evt eventsrc.OutboxEvent) error,
	opts ...ProjectionOption,
) *Projection {
	h := &Projection{
		subscriberID:   subscriberID,
		idempStore:     idempStore,
		versionStore:   versionStore,
		transactor:     transactor,
		handler:        handler,
		maxElapsedTime: 1 * time.Minute, // Set default
	}

	// Apply custom options
	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Handle processes an event with idempotency and retry logic.
func (h *Projection) Handle(ctx context.Context, evt eventsrc.OutboxEvent) error {
	// 1. Idempotency Check
	isProcessed, err := h.idempStore.IsProcessed(ctx, evt.EventID, h.subscriberID)
	if err != nil {
		return fmt.Errorf("failed to check for event idempotency: %w", err)
	}
	if isProcessed {
		slog.WarnContext(ctx, "Event already processed, skipping", "eventID", evt.EventID, "subscriber", h.subscriberID)
		return nil
	}

	// Retry Logic with Exponential Backoff
	operation := func() (any, error) {
		// 2. Ordering Check (by Version) - this now happens INSIDE the retry loop.
		currentVersion, err := h.versionStore.GetVersion(ctx, evt.AggregateID)
		if err != nil {
			// This is a transient error with the database, so we should retry.
			return nil, fmt.Errorf("failed to get current view version: %w", err)
		}
		if evt.Version <= currentVersion {
			slog.WarnContext(ctx, "Received old or duplicate event version, skipping",
				"eventID", evt.EventID, "eventVersion", evt.Version, "currentVersion", currentVersion)
			// This is a success case, no need to retry. We mark it as permanent to stop backoff.
			// It's important to still mark the event as processed in the idempotency store
			// to prevent it from being re-evaluated if it arrives again.
			return nil, backoff.Permanent(h.transactor.WithTransaction(ctx, func(txCtx context.Context) error {
				return h.idempStore.MarkAsProcessed(txCtx, evt.EventID, h.subscriberID)
			}))
		}

		if evt.Version != currentVersion+1 {
			slog.WarnContext(ctx, "Received out-of-order event, will be retried by the broker",
				"eventID", evt.EventID, "eventVersion", evt.Version, "expectedVersion", currentVersion+1)
			// Return the specific error. The broker infrastructure is responsible for how to handle this
			// (e.g., NAK with delay). This is considered a final state for this attempt.
			return nil, backoff.Permanent(ErrOutOfOrderEvent)
		}

		// 3. Transactional Execution
		// This ensures that business logic and marking the event as processed
		// happen atomically.
		txErr := h.transactor.WithTransaction(ctx, func(txCtx context.Context) error {
			// Execute the actual business logic
			if err := h.handler(txCtx, evt); err != nil {
				return fmt.Errorf("handler business logic failed: %w", err)
			}
			// Mark as processed within the same transaction
			if err := h.idempStore.MarkAsProcessed(txCtx, evt.EventID, h.subscriberID); err != nil {
				return fmt.Errorf("failed to mark event as processed: %w", err)
			}
			return nil
		})

		// Don't retry on certain application-level errors
		if txErr != nil && errors.Is(txErr, context.Canceled) {
			return nil, backoff.Permanent(txErr)
		}
		return nil, txErr
	}

	bo := backoff.NewExponentialBackOff()

	_, err = backoff.Retry(ctx, operation, backoff.WithBackOff(bo), backoff.WithMaxElapsedTime(h.maxElapsedTime))
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to process event after multiple retries",
			"error",
			err,
			"eventID",
			evt.EventID,
			"subscriber",
			h.subscriberID,
		)
		// Return error to have the message NAK'd and possibly redelivered or sent to a dead-letter queue.
		return err
	}

	slog.InfoContext(
		ctx,
		"Event processed successfully by idempotent handler",
		"eventID",
		evt.EventID,
		"subscriber",
		h.subscriberID,
	)
	return nil
}
