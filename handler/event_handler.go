package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/google/uuid"

	"github.com/0m3kk/cqrs/event"
)

// IdempotencyStore defines the interface for checking and storing processed event IDs.
type IdempotencyStore interface {
	IsProcessed(ctx context.Context, eventID uuid.UUID, subscriberID string) (bool, error)
	MarkAsProcessed(ctx context.Context, eventID uuid.UUID, subscriberID string) error
}

// TransactionalHandler defines a function that executes business logic within a transaction.
type TransactionalHandler func(ctx context.Context) error

// Transactor defines an interface for an object that can execute a function within a transaction.
type Transactor interface {
	WithTransaction(ctx context.Context, fn TransactionalHandler) error
}

// IdempotentEventHandler is a decorator that wraps a business logic handler
// with idempotency checks and retry logic.
type IdempotentEventHandler struct {
	subscriberID   string
	store          IdempotencyStore
	transactor     Transactor
	handler        func(ctx context.Context, evt event.OutboxEvent) error
	maxElapsedTime time.Duration
}

// HandlerOption is a function that configures an IdempotentEventHandler.
type HandlerOption func(*IdempotentEventHandler)

// WithMaxElapsedTime is an option to provide a custom backoff max elapsed time.
func WithMaxElapsedTime(maxElapsedTime time.Duration) HandlerOption {
	return func(h *IdempotentEventHandler) {
		h.maxElapsedTime = maxElapsedTime
	}
}

// NewIdempotentEventHandler creates a new idempotent event handler.
func NewIdempotentEventHandler(
	subscriberID string,
	store IdempotencyStore,
	transactor Transactor,
	handler func(ctx context.Context, evt event.OutboxEvent) error,
	opts ...HandlerOption,
) *IdempotentEventHandler {
	h := &IdempotentEventHandler{
		subscriberID:   subscriberID,
		store:          store,
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
func (h *IdempotentEventHandler) Handle(ctx context.Context, evt event.OutboxEvent) error {
	// 1. Idempotency Check
	isProcessed, err := h.store.IsProcessed(ctx, evt.EventID, h.subscriberID)
	if err != nil {
		return fmt.Errorf("failed to check for event idempotency: %w", err)
	}
	if isProcessed {
		slog.WarnContext(ctx, "Event already processed, skipping", "eventID", evt.EventID, "subscriber", h.subscriberID)
		return nil
	}

	// 2. Retry Logic with Exponential Backoff
	operation := func() (any, error) {
		// 3. Transactional Execution
		// This ensures that business logic and marking the event as processed
		// happen atomically.
		txErr := h.transactor.WithTransaction(ctx, func(txCtx context.Context) error {
			// Execute the actual business logic
			if err := h.handler(txCtx, evt); err != nil {
				return fmt.Errorf("handler business logic failed: %w", err)
			}
			// Mark as processed within the same transaction
			if err := h.store.MarkAsProcessed(txCtx, evt.EventID, h.subscriberID); err != nil {
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
