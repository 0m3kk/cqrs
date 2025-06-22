package aggregate

import "github.com/0m3kk/eventus/sample/domain/event"

func (p *ProductAggregate) onProductCreated(evt *event.ProductCreated) error {
	p.SetID(evt.AggregateID())
	p.Product.Name = evt.Name
	p.Product.Price = evt.Price
	return nil
}
