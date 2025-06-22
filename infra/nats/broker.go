package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/0m3kk/eventus/eventsrc"
)

// NATSBroker is an implementation of the Broker interface using NATS.
type NATSBroker struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// NewNATSBroker creates a new NATSBroker instance.
func NewNATSBroker(url string) (*NATSBroker, error) {
	nc, err := nats.Connect(
		url,
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &NATSBroker{conn: nc, js: js}, nil
}

// Publish sends an event to a NATS topic.
func (b *NATSBroker) Publish(ctx context.Context, topic string, evt eventsrc.OutboxEvent) error {
	// Ensure the stream exists for the topic
	streamName := topic
	_, err := b.js.StreamInfo(streamName)
	if err != nil {
		if err == nats.ErrStreamNotFound {
			slog.InfoContext(ctx, "Stream not found, creating it", "stream", streamName)
			_, err = b.js.AddStream(&nats.StreamConfig{
				Name:     streamName,
				Subjects: []string{fmt.Sprintf("%s.*", streamName)},
			})
			if err != nil {
				return fmt.Errorf("failed to create stream %s: %w", streamName, err)
			}
		} else {
			return fmt.Errorf("failed to get stream info for %s: %w", streamName, err)
		}
	}

	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	// Use the AggregateID as the NATS message subject suffix for partitioning.
	// Example subject: products.c7c0b6f2-7a7e-4b2a-8f3b-5e4e2a1e0b5e
	subject := fmt.Sprintf("%s.%s", topic, evt.AggregateID.String())

	_, err = b.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish event to NATS: %w", err)
	}

	slog.DebugContext(ctx, "Event published successfully", "topic", topic, "subject", subject, "eventID", evt.EventID)
	return nil
}

// Subscribe creates a durable, pull-based subscription.
func (b *NATSBroker) Subscribe(
	ctx context.Context,
	topic, subscriberID string,
	handler func(context.Context, eventsrc.OutboxEvent) error,
) error {
	streamName := topic
	consumerName := fmt.Sprintf("%s-%s", topic, subscriberID)

	// Create a durable consumer. This ensures that if the service restarts,
	// it will resume from where it left off.
	sub, err := b.js.PullSubscribe(
		fmt.Sprintf("%s.*", streamName), // Subscribe to all subjects in the stream
		consumerName,                    // Durable name
		nats.PullMaxWaiting(128),
	)
	if err != nil {
		if err == nats.ErrNoMatchingStream {
			slog.InfoContext(ctx, "Stream not found, creating it", "stream", streamName)
			_, err = b.js.AddStream(&nats.StreamConfig{
				Name:     streamName,
				Subjects: []string{fmt.Sprintf("%s.*", streamName)},
			})
			if err != nil {
				return fmt.Errorf("failed to create stream %s: %w", streamName, err)
			}
		} else {
			return fmt.Errorf("failed to create pull subscription: %w", err)
		}
	}

	go func() {
		slog.InfoContext(ctx, "Subscriber started", "topic", topic, "subscriberID", subscriberID)
		for {
			select {
			case <-ctx.Done():
				slog.InfoContext(ctx, "Subscriber stopping", "topic", topic, "subscriberID", subscriberID)
				return
			default:
				// Fetch a batch of messages
				msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
				if err != nil {
					if err != nats.ErrTimeout {
						slog.ErrorContext(ctx, "Failed to fetch messages", "error", err, "topic", topic)
					}
					continue
				}

				for _, msg := range msgs {
					var evt eventsrc.OutboxEvent
					if err := json.Unmarshal(msg.Data, &evt); err != nil {
						slog.ErrorContext(ctx, "Failed to unmarshal event, skipping", "error", err, "topic", topic)
						msg.Nak() // Negative acknowledgment, message might be redelivered
						continue
					}

					// Process the message
					if err := handler(ctx, evt); err != nil {
						slog.ErrorContext(ctx, "Handler failed to process event", "error", err, "eventID", evt.EventID)
						msg.Nak() // Nak to signal processing failure
					} else {
						msg.Ack() // Ack to confirm successful processing
					}
				}
			}
		}
	}()

	return nil
}

// Close gracefully closes the NATS connection.
func (b *NATSBroker) Close() {
	if b.conn != nil {
		b.conn.Close()
	}
}
