package aggregate

import (
	"encoding/json"

	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/domain"
)

const ProductAggregateType eventsrc.AggregateType = "products"

// ProductAggregate is our aggregate root. It embeds the base es.AggregateRoot for
// event sourcing behavior and holds its own state directly.
type ProductAggregate struct {
	*eventsrc.AggregateRoot
	Product domain.Product
}

// NewProductAggregateEmpty is a factory for creating a new, empty ProductAggregate instance.
// It's used by the repository to create a new aggregate before loading its history.
func NewProductAggregateEmpty() *ProductAggregate {
	p := &ProductAggregate{}
	// The key is to link the base aggregate's apply method and validator
	// to the concrete aggregate's implementations.
	p.AggregateRoot = eventsrc.NewAggregateRoot(ProductAggregateType, p.Apply, p.Validate)
	return p
}

// --- Snapshotting via json.Marshaler / json.Unmarshaler ---

// MarshalJSON implements the json.Marshaler interface for creating snapshots.
func (p *ProductAggregate) MarshalJSON() ([]byte, error) {
	type alias ProductAggregate
	return json.Marshal(&struct {
		*alias
	}{
		alias: (*alias)(p),
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface for restoring from snapshots.
func (p *ProductAggregate) UnmarshalJSON(data []byte) error {
	type alias ProductAggregate
	aux := &struct {
		*alias
	}{
		alias: (*alias)(p),
	}

	// Re-initialize the AggregateRoot with its methods before unmarshaling.
	p.AggregateRoot = eventsrc.NewAggregateRoot(ProductAggregateType, p.Apply, p.Validate)

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}
