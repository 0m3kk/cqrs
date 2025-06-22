package aggregate

import (
	"context"
	"fmt"
	"reflect"

	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/event"
)

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
	default:
		err = fmt.Errorf("unknown event type: %s", reflect.TypeOf(evt))
	}
	if err != nil {
		return err
	}
	p.SetVersion(evt.Version())
	return nil
}
