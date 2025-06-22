package testutil

import "github.com/0m3kk/eventus/eventsrc"

// ProductCreated is an event emitted when a new product is created.
type ProductCreated struct {
	eventsrc.BaseEvent
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (e ProductCreated) EventType() string { return "ProductCreated" }
