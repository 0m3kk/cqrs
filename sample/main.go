package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0m3kk/eventus/cqrs"
	"github.com/0m3kk/eventus/infra/nats"
	"github.com/0m3kk/eventus/infra/postgres"
	"github.com/0m3kk/eventus/outbox"
	"github.com/0m3kk/eventus/sample/command"
	domainRepository "github.com/0m3kk/eventus/sample/domain/repository"
	"github.com/0m3kk/eventus/sample/query/projection"
	viewRepository "github.com/0m3kk/eventus/sample/query/repository"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Create a context that we can cancel on shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --- Dependency Injection ---

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
	const snapshotFrequency = 5
	eventStore := postgres.NewEventStore(db, outboxStore, snapshotFrequency)
	productRepo := domainRepository.NewProductRepository(eventStore)
	idempotencyStore := postgres.NewIdempotencyStore(db)
	productViewRepo := viewRepository.NewProductViewRepository(db.Pool)

	// Framework Components
	for range 3 {
		relay := outbox.NewRelay(outboxStore, broker, 10, 2*time.Second)
		relay.Start(ctx)
	}
	slog.Info("Outbox relays started")

	// Application Handlers
	createProductHandler := command.NewCreateProductHandler(productRepo, db)

	// Event Handlers (Subscribers)
	productProjectionHandler := projection.NewProductProjectionHandler(productViewRepo)
	productProjection := cqrs.NewProjection(
		"ProductProjection",
		idempotencyStore,
		productViewRepo,
		db,
		productProjectionHandler.Handle,
	)

	if err := broker.Subscribe(ctx, "products", "ProductProjection", productProjection.Handle); err != nil {
		slog.Error("Failed to subscribe to topic", "error", err, "topic", "products")
		os.Exit(1)
	}

	// --- Simulate some work ---
	go func() {
		time.Sleep(3 * time.Second)
		slog.Info("Simulating product creation command...")
		cmd := command.CreateProductCommand{
			Name:  "Clean Architecture Widget",
			Price: 299.99,
		}
		if err := createProductHandler.Handle(context.Background(), cmd); err != nil {
			slog.Error("Failed to simulate command", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutdown signal received. Exiting.")
}
