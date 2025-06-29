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
// The factory function is responsible for creating a new, empty instance of a specific event.
// This function will panic if an event type is registered more than once.
func RegisterEvent(eventType string, factory EventFactory) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := eventRegistry[eventType]; ok {
		panic(fmt.Sprintf("event type '%s' is already registered", eventType))
	}

	eventRegistry[eventType] = factory
}

// CreateEvent instantiates an event given its type name by calling its registered factory.
// It returns an error if the event type has not been registered.
func CreateEvent(eventType string) (Event, error) {
	mu.RLock()
	defer mu.RUnlock()

	factory, ok := eventRegistry[eventType]
	if !ok {
		return nil, fmt.Errorf("event type '%s' is not registered", eventType)
	}

	// Call the factory to get a new instance (e.g., &ProductCreated{})
	return factory(), nil
}
