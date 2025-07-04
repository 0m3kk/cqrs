package outbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/infra/postgres"
	"github.com/0m3kk/eventus/outbox"
	"github.com/0m3kk/eventus/testutil"
)

// MockBroker is a simple mock for the messagebus.Broker interface.
type MockBroker struct {
	PublishedEvents chan eventsrc.OutboxEvent
	PublishError    error
}

func (m *MockBroker) Publish(ctx context.Context, topic string, evt eventsrc.OutboxEvent) error {
	if m.PublishError != nil {
		return m.PublishError
	}
	m.PublishedEvents <- evt
	return nil
}

func (m *MockBroker) Subscribe(
	ctx context.Context,
	topic, subscriberID string,
	handler func(context.Context, eventsrc.OutboxEvent) error,
) error {
	return nil
}
func (m *MockBroker) Close() {}

type RelayIntegrationSuite struct {
	testutil.DBIntegrationSuite
	store *postgres.OutboxStore
	db    *postgres.DB
}

func TestRelayIntegrationSuite(t *testing.T) {
	suite.Run(t, new(RelayIntegrationSuite))
}

func (s *RelayIntegrationSuite) SetupTest() {
	s.db = &postgres.DB{Pool: s.Pool}
	s.store = postgres.NewOutboxStore(s.db)
	s.TruncateTables("outbox")
}

func (s *RelayIntegrationSuite) TestRelay_ProcessesAndPublishesEvents() {
	// GIVEN
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broker := &MockBroker{PublishedEvents: make(chan eventsrc.OutboxEvent, 5)}

	// Insert some events into the outbox
	s.insertTestEvents(3)

	// WHEN
	// Start a relay worker
	relay := outbox.NewRelay(s.store, broker, 2, 50*time.Millisecond)
	relay.Start(ctx)

	// THEN
	// Collect published events
	var receivedEvents []eventsrc.OutboxEvent
	for range 3 {
		select {
		case evt := <-broker.PublishedEvents:
			receivedEvents = append(receivedEvents, evt)
		case <-ctx.Done():
			s.Fail("test timed out waiting for events")
		}
	}

	s.Len(receivedEvents, 3)

	// Verify that the events are marked as published in the DB
	var count int
	err := s.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox WHERE published = TRUE").Scan(&count)
	s.Require().NoError(err)
	s.Equal(3, count)
}

func (s *RelayIntegrationSuite) TestRelay_ConcurrentWorkersDoNotProcessSameEvent() {
	// GIVEN
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := &MockBroker{PublishedEvents: make(chan eventsrc.OutboxEvent, 20)}

	numEvents := 15
	s.insertTestEvents(numEvents)

	// WHEN
	// Start multiple relay workers concurrently
	numWorkers := 3
	relays := make([]*outbox.Relay, numWorkers)
	for i := range numWorkers {
		relays[i] = outbox.NewRelay(s.store, broker, 5, 50*time.Millisecond)
		relays[i].Start(ctx)
	}
	defer func() {
		for _, r := range relays {
			r.Stop()
		}
	}()

	// THEN
	// Collect all published events and ensure no duplicates
	publishedIDs := make(map[uuid.UUID]int)
	for range numEvents {
		select {
		case evt := <-broker.PublishedEvents:
			publishedIDs[evt.EventID]++
		case <-time.After(10 * time.Second):
			s.Fail("test timed out waiting for events")
		}
	}

	s.Len(publishedIDs, numEvents, "Should have received all unique events")
	for id, count := range publishedIDs {
		s.Equal(1, count, "Event %s was published more than once", id)
	}

	// Verify that all events are marked as published in the DB using an assertion
	// that retries, making the test robust against timing fluctuations.
	s.Require().Eventually(func() bool {
		var count int
		err := s.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox WHERE published = TRUE").Scan(&count)
		return s.NoError(err) && count == numEvents
	}, 5*time.Second, 100*time.Millisecond, "All events should eventually be marked as published")
}

func (s *RelayIntegrationSuite) insertTestEvents(count int) {
	aggregateID := uuid.New()
	for i := range count {
		evt := testutil.ProductCreated{
			BaseEvent: eventsrc.BaseEvent{ID: uuid.New(), AggID: aggregateID, AggType: "products", Ver: i + 1},
			Name:      "test",
			Price:     1.0,
		}
		err := s.store.SaveEvents(context.Background(), []eventsrc.Event{evt})
		s.Require().Error(err, "SaveEvents should fail outside a transaction")

		// Save correctly within a transaction
		err = s.db.WithTransaction(context.Background(), func(txCtx context.Context) error {
			return s.store.SaveEvents(txCtx, []eventsrc.Event{evt})
		})
		s.Require().NoError(err)
	}
}
