package testutil

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VersionedRepository is a concrete implementation that also satisfies the VersionedStore interface.
type VersionedRepository struct {
	pool *pgxpool.Pool
}

func NewVersionedRepository(pool *pgxpool.Pool) *VersionedRepository {
	return &VersionedRepository{pool: pool}
}

func (r *VersionedRepository) CreateTable() error {
	createTableSQL := `
CREATE TABLE IF NOT EXISTS versioned_views (
    id UUID PRIMARY KEY,
    version INT NOT NULL
);`

	// Execute the CREATE TABLE statement.
	// Exec is used for statements that do not return rows, like DDL commands (CREATE, ALTER, DROP).
	_, err := r.pool.Exec(context.Background(), createTableSQL)
	if err != nil {
		return err
	}
	return nil
}

// GetVersion retrieves the current version of the versioned view.
// It satisfies the handler.VersionedStore interface.
func (r *VersionedRepository) GetVersion(ctx context.Context, aggregateID uuid.UUID) (int, error) {
	var version int
	query := `SELECT version FROM versioned_views WHERE id = $1`
	err := r.pool.QueryRow(ctx, query, aggregateID).Scan(&version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // Return 0 if the view doesn't exist yet.
		}
		return 0, fmt.Errorf("failed to get versioned view version: %w", err)
	}
	return version, nil
}
