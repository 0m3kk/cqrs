package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/0m3kk/cqrs/event"
	"github.com/0m3kk/cqrs/messagebus"
)

// Store defines the interface for interacting with the outbox storage.
// It abstracts the transactional behavior of processing a batch.
type Store interface {
	// ProcessOutboxBatch fetches a batch of unpublished events, processes them using the provided function,
	// and marks them as published, all within a single transaction.
	// If processFunc returns an error, the entire transaction is rolled back.
	ProcessOutboxBatch(
		ctx context.Context,
		batchSize int,
		processFunc func(ctx context.Context, events []event.OutboxEvent) error,
	) error
}

// TopicMapper is a function type that maps an event type to a message bus topic.
type TopicMapper func(eventType string) string

// Relay is a background worker that polls the outbox and publishes events.
type Relay struct {
	store       Store
	broker      messagebus.Broker
	topicMapper TopicMapper
	batchSize   int
	interval    time.Duration
	wg          sync.WaitGroup
	quit        chan struct{}
}

// NewRelay creates a new Relay instance.
// It can be run with multiple instances for scalability.
func NewRelay(store Store, broker messagebus.Broker, mapper TopicMapper, batchSize int, interval time.Duration) *Relay {
	return &Relay{
		store:       store,
		broker:      broker,
		topicMapper: mapper,
		batchSize:   batchSize,
		interval:    interval,
		quit:        make(chan struct{}),
	}
}

// Start begins the relay's polling process in a separate goroutine.
func (r *Relay) Start(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		slog.InfoContext(ctx, "Outbox relay started")
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := r.processBatch(ctx); err != nil {
					slog.ErrorContext(ctx, "Failed to process outbox batch", "error", err)
				}
			case <-r.quit:
				slog.InfoContext(ctx, "Outbox relay shutting down")
				return
			case <-ctx.Done():
				slog.InfoContext(ctx, "Context cancelled, outbox relay shutting down")
				return
			}
		}
	}()
}

// processBatch defines the logic for publishing events and passes it to the store
// to be executed within a transaction.
func (r *Relay) processBatch(ctx context.Context) error {
	// This function contains the logic to execute once events are fetched and locked.
	processor := func(ctx context.Context, events []event.OutboxEvent) error {
		if len(events) == 0 {
			return nil
		}
		slog.DebugContext(ctx, "Processing fetched events", "count", len(events))

		for _, evt := range events {
			topic := r.topicMapper(evt.EventType)
			if topic == "" {
				slog.WarnContext(
					ctx,
					"No topic mapped for event type, skipping",
					"eventType",
					evt.EventType,
					"eventID",
					evt.EventID,
				)
				continue
			}

			// Publish the event to the message broker.
			if err := r.broker.Publish(ctx, topic, evt); err != nil {
				// Returning an error here will cause the transaction to be rolled back.
				return fmt.Errorf("failed to publish event %s to topic %s: %w", evt.EventID, topic, err)
			}
		}
		slog.InfoContext(ctx, "Successfully published events to broker", "count", len(events))
		return nil
	}

	// The store handles the entire transactional process.
	return r.store.ProcessOutboxBatch(ctx, r.batchSize, processor)
}

// Stop gracefully stops the relay.
func (r *Relay) Stop() {
	close(r.quit)
	r.wg.Wait()
}
