package event

import "github.com/0m3kk/eventus/eventsrc"

const ProductUpdatedEventType = "ProductUpdated"

type ProductUpdated struct {
	eventsrc.BaseEvent
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (e ProductUpdated) EventType() string {
	return ProductUpdatedEventType
}

func init() {
	eventsrc.RegisterEvent(ProductUpdatedEventType, func() eventsrc.Event {
		return &ProductUpdated{}
	})
}
