package eventsrc

import (
	"fmt"
	"sync"
)

// EventFactory is a function that creates a new instance of an event.
type EventFactory func() Event

var (
	eventRegistry = make(map[string]EventFactory)
	mu            sync.RWMutex
)

// RegisterEvent associates an event type name with a factory function.
// It should be called during application initialization (e.g., in an init() function).
// This function will panic if an event type is registered more than once.
func RegisterEvent[T Event](event T) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := eventRegistry[event.EventType()]; ok {
		panic(fmt.Sprintf("event type '%s' is already registered", event.AggregateType()))
	}
	eventRegistry[event.EventType()] = func() Event {
		return *new(T)
	}
}

// CreateEvent instantiates an event given its type name.
// It returns an error if the event type has not been registered.
func CreateEvent(eventType string) (Event, error) {
	mu.RLock()
	defer mu.RUnlock()

	factory, ok := eventRegistry[eventType]
	if !ok {
		return nil, fmt.Errorf("event type '%s' is not registered", eventType)
	}

	return factory(), nil
}
