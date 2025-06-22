package repository

import (
	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/aggregate"
)

// ProductRepository is a Postgres-backed implementation of the product repository.
type ProductRepository struct {
	*eventsrc.Repository[*aggregate.ProductAggregate]
}

// newEmptyProduct is a factory for creating a new, empty Product instance.
// It's used by the repository to create a new aggregate before loading its history.
func newEmptyProduct() *aggregate.ProductAggregate {
	p := &aggregate.ProductAggregate{}
	// The key is to link the base aggregate's apply method and validator
	// to the concrete aggregate's implementations.
	p.AggregateRoot = eventsrc.NewAggregateRoot(aggregate.ProductAggregateType, p.Apply, p.Validate)
	return p
}

// NewProductRepository creates a new product repository.
// It internally creates a generic es.Repository configured for the Product aggregate.
func NewProductRepository(store eventsrc.Store) *ProductRepository {
	// The generic repository needs a factory function to create an empty aggregate.
	repo := eventsrc.NewRepository(store, newEmptyProduct)

	return &ProductRepository{
		Repository: repo,
	}
}
