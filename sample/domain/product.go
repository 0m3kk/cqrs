package domain

import (
	"github.com/google/uuid"

	"github.com/0m3kk/eventus/eventsrc"
)

// Product is our aggregate root.
type Product struct {
	ID      uuid.UUID
	Name    string
	Price   float64
	Version int
	events  []eventsrc.Event
}

// ProductCreated is an event emitted when a new product is created.
type ProductCreated struct {
	eventsrc.BaseEvent
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (e ProductCreated) EventType() string { return "ProductCreated" }

// NewProduct is a factory function for creating a new product.
func NewProduct(name string, price float64) *Product {
	p := &Product{}
	aggID := uuid.New()

	evt := ProductCreated{
		BaseEvent: eventsrc.BaseEvent{
			ID:    uuid.New(),
			AggID: aggID,
			Ver:   1,
		},
		Name:  name,
		Price: price,
	}

	p.apply(evt)
	p.events = append(p.events, evt)
	return p
}

// GetUncommittedEvents returns and clears the list of uncommitted events.
func (p *Product) GetUncommittedEvents() []eventsrc.Event {
	evts := p.events
	p.events = nil
	return evts
}

// apply changes the state of the aggregate based on an event.
func (p *Product) apply(evt eventsrc.Event) {
	switch e := evt.(type) {
	case ProductCreated:
		p.ID = e.AggregateID()
		p.Name = e.Name
		p.Price = e.Price
	}
	p.Version = evt.Version()
}
