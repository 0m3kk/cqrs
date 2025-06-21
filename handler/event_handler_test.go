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
	"github.com/0m3kk/cqrs/sample/infra/postgres"
	"github.com/0m3kk/cqrs/testutil"
)

type HandlerIntegrationSuite struct {
	testutil.DBIntegrationSuite
	store *postgres.IdempotencyStore
	db    *postgres.DB
}

func TestHandlerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(HandlerIntegrationSuite))
}

func (s *HandlerIntegrationSuite) SetupTest() {
	s.db = &postgres.DB{Pool: s.Pool}
	s.store = postgres.NewIdempotencyStore(s.db)
	s.TruncateTables("processed_events")
}

func (s *HandlerIntegrationSuite) TestIdempotentHandler_HappyPath() {
	// GIVEN
	ctx := context.Background()
	subscriberID := "test-subscriber-1"
	eventID := uuid.New()
	handlerCallCount := 0

	// A simple handler that just increments a counter
	mockHandler := func(ctx context.Context, evt event.OutboxEvent) error {
		handlerCallCount++
		return nil
	}

	idempotentHandler := handler.NewIdempotentEventHandler(subscriberID, s.store, s.db, mockHandler)
	testEvent := event.OutboxEvent{EventID: eventID}

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
	eventID := uuid.New()
	handlerCallCount := 0

	mockHandler := func(ctx context.Context, evt event.OutboxEvent) error {
		handlerCallCount++
		return nil
	}

	idempotentHandler := handler.NewIdempotentEventHandler(subscriberID, s.store, s.db, mockHandler)
	testEvent := event.OutboxEvent{EventID: eventID}

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
		s.db,
		failingHandler,
		handler.WithMaxElapsedTime(5*time.Second),
	)
	testEvent := event.OutboxEvent{EventID: eventID}

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
