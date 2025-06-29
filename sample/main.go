package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/0m3kk/eventus/cqrs"
	"github.com/0m3kk/eventus/infra/nats"
	"github.com/0m3kk/eventus/infra/postgres"
	"github.com/0m3kk/eventus/outbox"
	"github.com/0m3kk/eventus/sample/command"
	domainRepository "github.com/0m3kk/eventus/sample/domain/repository"
	"github.com/0m3kk/eventus/sample/query/projection"
	"github.com/0m3kk/eventus/sample/query/query"
	viewRepository "github.com/0m3kk/eventus/sample/query/repository"
)

// ProductViewSchema defines the SQL statement for creating the product_views table.
const ProductViewSchema = `
CREATE TABLE IF NOT EXISTS product_views (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price INT NOT NULL,
    version INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);`

// createProductViewTable ensures the product_views table exists in the database.
func createProductViewTable(db *postgres.DB) error {
	_, err := db.Pool.Exec(context.Background(), ProductViewSchema)
	if err != nil {
		return err
	}
	return nil
}

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

	if err = createProductViewTable(db); err != nil {
		slog.Error("Failed to create product views", "error", err)
		os.Exit(1)
	}

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
	// Start multiple relay workers for concurrency
	for range 3 {
		relay := outbox.NewRelay(outboxStore, broker, 10, 2*time.Second)
		relay.Start(ctx)
	}
	slog.Info("Outbox relays started")

	// --- Application Handlers ---

	// Command Side
	createProductHandler := command.NewCreateProductHandler(productRepo, db)
	updateProductHandler := command.NewUpdateProductHandler(productRepo, db)

	// Query Side
	getProductByIDHandler := query.NewGetProductByIDHandler(*productViewRepo)

	// Event Handlers (Subscribers)
	productProjectionHandler := projection.NewProductProjectionHandler(productViewRepo)
	productProjection := cqrs.NewProjection(
		"ProductProjection",
		idempotencyStore,
		productViewRepo, // The product view repo satisfies the VersionedStore interface
		db,
		productProjectionHandler.Handle,
	)

	// Subscribe the projection to the "products" topic
	if err := broker.Subscribe(ctx, "products", "ProductProjection", productProjection.Handle); err != nil {
		slog.Error("Failed to subscribe to topic", "error", err, "topic", "products")
		os.Exit(1)
	}

	// --- Simulate Work (Full CQRS Loop) ---
	go func() {
		// Generate a new ID for the product we're about to create.
		// In a real API, this might come from the request.
		productID := uuid.New()

		// 1. COMMAND: Send a command to create a new product.
		slog.Info("--> 1. Simulating CreateProductCommand...", "productID", productID)
		createCmd := command.CreateProductCommand{
			ID:    productID,
			Name:  "CQRS-Powered Widget",
			Price: 199.99,
		}
		if err := createProductHandler.Handle(context.Background(), createCmd); err != nil {
			slog.Error("Failed to handle CreateProductCommand", "error", err)
			return
		}
		slog.Info("<-- Command handled successfully.")

		// Give the system a moment to process the event through the outbox and projection.
		// This simulates the eventual consistency of the read model.
		slog.Info("... Waiting 5 seconds for eventual consistency ...")
		time.Sleep(5 * time.Second)

		// 2. QUERY: Query the read model to get the product view.
		slog.Info("--> 2. Simulating GetProductByIDQuery...", "productID", productID)
		productView, err := getProductByIDHandler.Query(context.Background(), query.GetProductByID{ID: productID})
		if err != nil {
			slog.Error("Failed to handle GetProductByIDQuery", "error", err)
			return
		}

		slog.Info(
			"<-- Query handled successfully. Product Details:",
			"name",
			productView.Name,
			"price",
			productView.Price,
		)

		// 3. COMMAND: Send a command to update the product.
		slog.Info("--> 3. Simulating UpdateProductCommand...", "productID", productID)
		updateCmd := command.UpdateProduct{
			ID:    productID,
			Name:  "CQRS-Powered Widget (Updated)",
			Price: 200.01,
		}
		if err := updateProductHandler.Handle(context.Background(), updateCmd); err != nil {
			slog.Error("Failed to handle UpdateProductCommand", "error", err)
			return
		}

		// Give the system a moment to process the event through the outbox and projection.
		// This simulates the eventual consistency of the read model.
		slog.Info("... Waiting 5 seconds for eventual consistency ...")
		time.Sleep(5 * time.Second)

		// 4. QUERY: Query the read model to get the product view.
		slog.Info("--> 4. Simulating GetProductByIDQuery...", "productID", productID)
		productView, err = getProductByIDHandler.Query(context.Background(), query.GetProductByID{ID: productID})
		if err != nil {
			slog.Error("Failed to handle GetProductByIDQuery", "error", err)
			return
		}

		slog.Info(
			"<-- Query handled successfully. Product Details:",
			"name",
			productView.Name,
			"price",
			productView.Price,
		)
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("Shutdown signal received. Exiting.")
}
