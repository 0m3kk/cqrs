package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0m3kk/eventus/sample/query/view"
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
func (r *ProductViewRepository) SaveProductView(ctx context.Context, view view.ProductView) error {
	query := `
        INSERT INTO product_views (id, name, price, version, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            price = EXCLUDED.price,
            version = EXCLUDED.version,
            updated_at = EXCLUDED.updated_at
    `
	_, err := r.pool.Exec(ctx, query, view.ID, view.Name, view.Price, view.Version, view.CreatedAt, view.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save product view: %w", err)
	}
	return nil
}

func (r *ProductViewRepository) UpdateProductView(ctx context.Context, view view.ProductView) error {
	query := `
		UPDATE product_views SET
			name = $2,
			price = $3,
			version = $4,
			updated_at = $5
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, view.ID, view.Name, view.Price, view.Version, view.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update product view: %w", err)
	}
	return nil
}

// GetProductViewByID retrieves the product view by its ID.
func (r *ProductViewRepository) GetProductViewByID(
	ctx context.Context,
	aggregateID uuid.UUID,
) (*view.ProductView, error) {
	var productView view.ProductView
	query := `
        SELECT id, name, price, version, created_at, updated_at
        FROM product_views
        WHERE id = $1
    `
	err := r.pool.QueryRow(ctx, query, aggregateID).Scan(
		&productView.ID,
		&productView.Name,
		&productView.Price,
		&productView.Version,
		&productView.CreatedAt,
		&productView.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Return nil if the view doesn't exist yet.
		}
		return nil, fmt.Errorf("failed to get product view by ID: %w", err)
	}
	return &productView, nil
}

// txKey is a private key type to get the transaction from the context.
type txKey struct{}
