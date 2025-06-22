package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain"
)

// ProductViewRepository is a concrete implementation that also satisfies the VersionedStore interface.
type ProductViewRepository struct {
	pool *pgxpool.Pool
}

func NewProductViewRepository(pool *pgxpool.Pool) *ProductViewRepository {
	return &ProductViewRepository{pool: pool}
}

// GetVersion retrieves the current version of the product view.
// It satisfies the handler.VersionedStore interface.
func (r *ProductViewRepository) GetVersion(ctx context.Context, aggregateID uuid.UUID) (int, error) {
	var version int
	query := `SELECT version FROM product_views WHERE id = $1`
	err := r.pool.QueryRow(ctx, query, aggregateID).Scan(&version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // Return 0 if the view doesn't exist yet.
		}
		return 0, fmt.Errorf("failed to get product view version: %w", err)
	}
	return version, nil
}

// SaveProductView saves or updates the product read model.
// This would be called by the event handler's business logic.
func (r *ProductViewRepository) SaveProductView(ctx context.Context, evt domain.ProductCreated) error {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return fmt.Errorf("SaveProductView must be called within a transaction")
	}

	query := `
        INSERT INTO product_views (id, name, price, version, updated_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            price = EXCLUDED.price,
            version = EXCLUDED.version,
            updated_at = EXCLUDED.updated_at
    `
	_, err := tx.Exec(ctx, query, evt.AggregateID(), evt.Name, evt.Price, evt.Version(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to save product view: %w", err)
	}
	return nil
}

// txKey is a private key type to get the transaction from the context.
type txKey struct{}

// ProductViewProjection is a subscriber that creates a denormalized view.
// In a real app, this would write to a different database (e.g., Elasticsearch, Redis).
type ProductViewProjection struct {
	repo *ProductViewRepository
}

func NewProductViewProjection(repo *ProductViewRepository) *ProductViewProjection {
	return &ProductViewProjection{repo: repo}
}

// HandleProductCreated processes the ProductCreated event.
func (p *ProductViewProjection) HandleProductCreated(ctx context.Context, evt eventsrc.OutboxEvent) error {
	var productCreatedEvt domain.ProductCreated
	if err := json.Unmarshal(evt.Payload, &productCreatedEvt); err != nil {
		return fmt.Errorf("failed to unmarshal ProductCreated event: %w", err)
	}

	slog.InfoContext(ctx, "Projecting ProductView",
		"productID", productCreatedEvt.AggregateID(),
		"name", productCreatedEvt.Name,
		"price", productCreatedEvt.Price)

	// The actual business logic is to save the view.
	return p.repo.SaveProductView(ctx, productCreatedEvt)
}
