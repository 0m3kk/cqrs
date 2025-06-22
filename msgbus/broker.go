package msgbus

import (
	"context"

	"github.com/0m3kk/eventus/eventsrc"
)

// Broker defines the interface for a message broker used to publish events.
type Broker interface {
	// Publish sends an event to a specific topic.
	Publish(ctx context.Context, topic string, evt eventsrc.OutboxEvent) error
	// Subscribe creates a subscription to a topic and handles incoming messages
	// using the provided handler function.
	Subscribe(
		ctx context.Context,
		topic, subscriberID string,
		handler func(ctx context.Context, evt eventsrc.OutboxEvent) error,
	) error
	// Close gracefully shuts down the broker connection.
	Close()
}
