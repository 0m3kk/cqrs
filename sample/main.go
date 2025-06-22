package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0m3kk/eventus/cqrs"
	"github.com/0m3kk/eventus/outbox"
	"github.com/0m3kk/eventus/sample/app"
	"github.com/0m3kk/eventus/sample/infra/nats"
	"github.com/0m3kk/eventus/sample/infra/postgres"
)

// defineTopicMapper is an application-specific function that defines routing rules.
func defineTopicMapper() outbox.TopicMapper {
	return func(eventType string) string {
		switch eventType {
		case "ProductCreated":
			return "products"
		// Add other event-to-topic mappings here
		// case "OrderCreated":
		// 	return "orders"
		default:
			return "" // No topic for this event
		}
	}
}

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Create a context that we can cancel on shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --- Dependency Injection ---

	// Get config from environment
	dsn := os.Getenv("APP_DSN")
	if dsn == "" {
		slog.Error("APP_DSN environment variable not set")
		os.Exit(1)
	}
	natsURL := os.Getenv("APP_NATS_URL")
	if natsURL == "" {
		slog.Error("APP_NATS_URL environment variable not set")
		os.Exit(1)
	}

	// Infrastructure
	db, err := postgres.NewDB(dsn)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("Database connection established")

	broker, err := nats.NewNATSBroker(natsURL)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer broker.Close()
	slog.Info("NATS connection established")

	outboxStore := postgres.NewOutboxStore(db)
	idempotencyStore := postgres.NewIdempotencyStore(db)
	productViewRepo := app.NewProductViewRepository(db.Pool)

	// Framework Components
	topicMapper := defineTopicMapper()

	// Run multiple relay workers for scalability
	for range 3 {
		relay := outbox.NewRelay(outboxStore, broker, topicMapper, 10, 2*time.Second)
		relay.Start(ctx) // It runs in the background
		// In a real app, you'd manage these relay instances for graceful shutdown.
	}
	slog.Info("Outbox relays started")

	// Application Handlers
	// Command Handlers (would be exposed via an API in a real app)
	createProductHandler := app.NewCreateProductHandler(outboxStore, db)

	// Event Handlers (Subscribers)
	productProjection := app.NewProductViewProjection(productViewRepo)
	idempotentProductViewHandler := cqrs.NewProjection(
		"ProductViewProjection", // Unique subscriber ID
		idempotencyStore,
		productViewRepo,
		db,
		productProjection.HandleProductCreated,
	)

	// Start subscribing to topics
	if err := broker.Subscribe(ctx, "products", "ProductViewProjection", idempotentProductViewHandler.Handle); err != nil {
		slog.Error("Failed to subscribe to topic", "error", err, "topic", "products")
		os.Exit(1)
	}

	// --- Simulate some work ---
	go func() {
		// Wait a bit for subscriptions to be ready
		time.Sleep(3 * time.Second)
		slog.Info("Simulating product creation command...")
		cmd := app.CreateProductCommand{
			Name:  "Production-Ready Widget",
			Price: 99.99,
		}
		if err := createProductHandler.Handle(context.Background(), cmd); err != nil {
			slog.Error("Failed to simulate command", "error", err)
		}
	}()

	// --- Wait for shutdown signal ---
	<-ctx.Done()
	slog.Info("Shutdown signal received. Exiting.")
	// Graceful shutdown logic would go here (e.g., waiting for relays/subscribers to finish)
}
