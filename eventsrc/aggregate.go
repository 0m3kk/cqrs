package eventsrc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// AggregateType defines the type of an aggregate (e.g., "Product", "Order").
type AggregateType string

// Aggregate is the interface that event-sourced aggregates must implement.
// It now embeds the standard JSON marshaling interfaces for snapshotting.
type Aggregate interface {
	json.Marshaler
	json.Unmarshaler

	// ID returns the unique identifier of the aggregate.
	ID() uuid.UUID
	// AggregateType returns the type of the aggregate (e.g., "Product").
	AggregateType() AggregateType
	// Version returns the current version of the aggregate after applying events.
	Version() int
	// GetUncommittedEvents returns the list of new, uncommitted events.
	GetUncommittedEvents() []Event
	// ClearUncommittedEvents clears the list of uncommitted events.
	ClearUncommittedEvents()
	// LoadFromHistory rehydrates the aggregate's state from a stream of past events.
	LoadFromHistory(ctx context.Context, events []Event)
	// Apply applies an event to the aggregate, changing its state.
	Apply(ctx context.Context, evt Event)
	// Validate checks if the aggregate's current state is valid.
	Validate() error
}

// AggregateRoot is a base implementation for event-sourced aggregates.
// It tracks the aggregate's ID, version, and uncommitted events.
type AggregateRoot struct {
	id            uuid.UUID
	aggType       AggregateType
	version       int
	events        []Event
	applyMethod   func(context.Context, Event)
	validateState func() error
}

// NewAggregateRoot is a constructor for AggregateRoot.
// It requires references to the concrete aggregate's apply and validate methods.
func NewAggregateRoot(
	aggType AggregateType,
	applyMethod func(context.Context, Event),
	validateState func() error,
) *AggregateRoot {
	return &AggregateRoot{
		aggType:       aggType,
		applyMethod:   applyMethod,
		validateState: validateState,
	}
}

func (a *AggregateRoot) ID() uuid.UUID                 { return a.id }
func (a *AggregateRoot) AggregateType() AggregateType  { return a.aggType }
func (a *AggregateRoot) Version() int                  { return a.version }
func (a *AggregateRoot) GetUncommittedEvents() []Event { return a.events }
func (a *AggregateRoot) ClearUncommittedEvents()       { a.events = nil }

// TrackChange records a new event by applying it, validating the new state,
// and adding it to the list of uncommitted events.
func (a *AggregateRoot) TrackChange(ctx context.Context, evt Event) error {
	a.applyMethod(ctx, evt)
	if err := a.validateState(); err != nil {
		return fmt.Errorf("state validation failed after applying event %s: %w", evt.EventType(), err)
	}
	a.events = append(a.events, evt)
	return nil
}

// LoadFromHistory rehydrates the aggregate's state by applying a series of past events.
// It does NOT validate the state, as historical events are assumed to be valid.
func (a *AggregateRoot) LoadFromHistory(ctx context.Context, history []Event) {
	for _, evt := range history {
		a.applyMethod(ctx, evt)
	}
}

// Apply is a placeholder and should be implemented by embedding structs.
func (a *AggregateRoot) Apply(ctx context.Context, evt Event) {
	a.applyMethod(ctx, evt)
}

// Validate is a placeholder and should be implemented by embedding structs.
func (a *AggregateRoot) Validate() error {
	return a.validateState()
}

// Setters used internally by apply methods of the concrete aggregate.
func (a *AggregateRoot) SetID(id uuid.UUID)     { a.id = id }
func (a *AggregateRoot) SetVersion(version int) { a.version = version }
