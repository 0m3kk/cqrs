package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/domain"
	"github.com/0m3kk/eventus/sample/domain/event"
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

// Validate checks if the aggregate's current state is consistent.
func (p *ProductAggregate) Validate() error {
	if err := p.Product.Validate(); err != nil {
		return err
	}
	return nil
}

// Apply changes the state of the aggregate based on an event.
func (p *ProductAggregate) Apply(ctx context.Context, evt eventsrc.Event) error {
	var err error
	switch e := evt.(type) {
	case *event.ProductCreated:
		err = p.onProductCreated(e)
	case *event.ProductUpdated:
		err = p.onProductUpdated(e)
	default:
		err = fmt.Errorf("unknown event type: %s", reflect.TypeOf(evt))
	}
	if err != nil {
		return err
	}
	p.SetVersion(evt.Version())
	return nil
}

func (p *ProductAggregate) onProductCreated(evt *event.ProductCreated) error {
	p.SetID(evt.AggregateID())
	p.Product.Name = evt.Name
	p.Product.Price = evt.Price
	return nil
}

func (p *ProductAggregate) onProductUpdated(evt *event.ProductUpdated) error {
	p.Product.Name = evt.Name
	p.Product.Price = evt.Price
	return nil
}
