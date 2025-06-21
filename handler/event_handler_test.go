// Integration test for the Idempotent Event Handler wrapper.
// --------------------------------------------------------------
package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/0m3kk/cqrs/event"
	"github.com/0m3kk/cqrs/handler"
	"github.com/0m3kk/cqrs/sample/app"
	"github.com/0m3kk/cqrs/sample/infra/postgres"
	"github.com/0m3kk/cqrs/testutil"
)

type HandlerIntegrationSuite struct {
	testutil.DBIntegrationSuite
	store           *postgres.IdempotencyStore
	productViewRepo *app.ProductViewRepository
	db              *postgres.DB
}

func TestHandlerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(HandlerIntegrationSuite))
}

func (s *HandlerIntegrationSuite) SetupTest() {
	s.db = &postgres.DB{Pool: s.Pool}
	s.store = postgres.NewIdempotencyStore(s.db)
	s.productViewRepo = app.NewProductViewRepository(s.Pool)
	s.TruncateTables("processed_events")
}

func (s *HandlerIntegrationSuite) TestIdempotentHandler_HappyPath() {
	// GIVEN
	ctx := context.Background()
	subscriberID := "test-subscriber-1"
	eventID := uuid.New()
	aggregateID := uuid.New()
	handlerCallCount := 0

	// A simple handler that just increments a counter
	mockHandler := func(ctx context.Context, evt event.OutboxEvent) error {
		handlerCallCount++
		return nil
	}

	idempotentHandler := handler.NewIdempotentEventHandler(subscriberID, s.store, s.productViewRepo, s.db, mockHandler)
	testEvent := event.OutboxEvent{EventID: eventID, AggregateID: aggregateID, Version: 1}

	// WHEN
	err := idempotentHandler.Handle(ctx, testEvent)

	// THEN
	s.NoError(err)
	s.Equal(1, handlerCallCount, "Handler should be called exactly once")

	// Verify it was marked as processed
	isProcessed, err := s.store.IsProcessed(ctx, eventID, subscriberID)
	s.NoError(err)
	s.True(isProcessed)
}

func (s *HandlerIntegrationSuite) TestIdempotentHandler_SkipsDuplicateEvent() {
	// GIVEN
	ctx := context.Background()
	subscriberID := "test-subscriber-2"
	aggregateID := uuid.New()
	handlerCallCount := 0

	mockHandler := func(ctx context.Context, evt event.OutboxEvent) error {
		handlerCallCount++
		return nil
	}

	idempotentHandler := handler.NewIdempotentEventHandler(subscriberID, s.store, s.productViewRepo, s.db, mockHandler)
	testEvent := event.OutboxEvent{EventID: uuid.New(), AggregateID: aggregateID, Version: 1}

	// Process it the first time
	err := idempotentHandler.Handle(ctx, testEvent)
	s.Require().NoError(err)
	s.Require().Equal(1, handlerCallCount)

	// WHEN
	// Process the exact same event again
	err = idempotentHandler.Handle(ctx, testEvent)

	// THEN
	s.NoError(err, "Processing a duplicate event should not return an error")
	s.Equal(1, handlerCallCount, "Handler should not be called for a duplicate event")
}

func (s *HandlerIntegrationSuite) TestIdempotentHandler_RollsBackOnHandlerFailure() {
	// GIVEN
	ctx := context.Background()
	subscriberID := "test-subscriber-3"
	eventID := uuid.New()
	aggregateID := uuid.New()
	handlerCallCount := 0

	// A handler that always fails
	failingHandler := func(ctx context.Context, evt event.OutboxEvent) error {
		handlerCallCount++
		return errors.New("business logic failed")
	}

	// We disable retry for this test to check the immediate result
	idempotentHandler := handler.NewIdempotentEventHandler(
		subscriberID,
		s.store,
		s.productViewRepo,
		s.db,
		failingHandler,
		handler.WithMaxElapsedTime(5*time.Second),
	)
	testEvent := event.OutboxEvent{EventID: eventID, AggregateID: aggregateID, Version: 1}

	// WHEN
	err := idempotentHandler.Handle(ctx, testEvent)

	// THEN
	s.Error(err, "Handle should return an error if the inner handler fails after retries")
	s.True(handlerCallCount > 0, "Handler should have been called at least once")

	// Verify it was NOT marked as processed due to the transaction rollback
	isProcessed, dbErr := s.store.IsProcessed(ctx, eventID, subscriberID)
	s.NoError(dbErr)
	s.False(isProcessed, "Event should not be marked as processed if handler fails")
}

func (s *HandlerIntegrationSuite) TestIdempotentHandler_RetriesOnTransientFailure() {
	// GIVEN
	ctx := context.Background()
	subscriberID := "test-subscriber-4"
	aggregateID := uuid.New()
	handlerCallCount := 0

	transientlyFailingHandler := func(ctx context.Context, evt event.OutboxEvent) error {
		handlerCallCount++
		if handlerCallCount < 2 {
			return errors.New("transient database error")
		}
		return nil
	}

	idempotentHandler := handler.NewIdempotentEventHandler(
		subscriberID,
		s.store,
		s.productViewRepo,
		s.db,
		transientlyFailingHandler,
		handler.WithMaxElapsedTime(2*time.Second),
	)
	testEvent := event.OutboxEvent{EventID: uuid.New(), AggregateID: aggregateID, Version: 1}

	// WHEN
	err := idempotentHandler.Handle(ctx, testEvent)

	// THEN
	s.NoError(err, "Handle should eventually succeed after retries")
	s.Equal(2, handlerCallCount, "Handler should be called twice")

	isProcessed, dbErr := s.store.IsProcessed(ctx, testEvent.EventID, subscriberID)
	s.NoError(dbErr)
	s.True(isProcessed)
}

func (s *HandlerIntegrationSuite) TestIdempotentHandler_RejectsOutOfOrderEvent() {
	// GIVEN
	ctx := context.Background()
	subscriberID := "test-subscriber-5-ordering"
	aggregateID := uuid.New()
	handlerCallCount := 0

	// This is the handler for the business logic, which should NOT be called.
	mockHandler := func(ctx context.Context, evt event.OutboxEvent) error {
		handlerCallCount++
		return nil
	}

	// An event with version 2, while the current DB version for the aggregate is 0.
	outOfOrderEvent := event.OutboxEvent{
		EventID:     uuid.New(),
		AggregateID: aggregateID,
		Version:     2,
	}

	idempotentHandler := handler.NewIdempotentEventHandler(
		subscriberID,
		s.store,
		s.productViewRepo,
		s.db,
		mockHandler,
	)

	// WHEN
	err := idempotentHandler.Handle(ctx, outOfOrderEvent)

	// THEN
	// 1. We expect a specific "out of order" error, which is wrapped.
	s.Require().Error(err)
	s.ErrorIs(err, handler.ErrOutOfOrderEvent, "Expected a specific out-of-order error")

	// 2. The business logic handler should never have been called.
	s.Equal(0, handlerCallCount, "Business logic handler should not be called for an out-of-order event")

	// 3. The event should NOT be marked as processed in the idempotency store.
	isProcessed, dbErr := s.store.IsProcessed(ctx, outOfOrderEvent.EventID, subscriberID)
	s.NoError(dbErr)
	s.False(isProcessed, "Out-of-order event should not be marked as processed")

	// 4. The view model version should remain unchanged (at 0).
	currentVersion, dbErr := s.productViewRepo.GetVersion(ctx, aggregateID)
	s.NoError(dbErr)
	s.Equal(0, currentVersion, "View model version should not have changed")
}
