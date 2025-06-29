package eventsrc

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event is the interface that all domain events must implement.
type Event interface {
	EventID() uuid.UUID
	AggregateID() uuid.UUID
	AggregateType() AggregateType
	EventType() string
	Version() int
	Timestamp() time.Time
}

// BaseEvent provides a common implementation for the Event interface.
// Domain events can embed this struct to reduce boilerplate.
type BaseEvent struct {
	ID      uuid.UUID     `json:"id"`
	AggID   uuid.UUID     `json:"aggregate_id"`
	AggType AggregateType `json:"aggregate_type"`
	Ver     int           `json:"version"`
	Ts      time.Time     `json:"ts"`
}

func (b BaseEvent) EventID() uuid.UUID           { return b.ID }
func (b BaseEvent) AggregateID() uuid.UUID       { return b.AggID }
func (b BaseEvent) AggregateType() AggregateType { return b.AggType }
func (b BaseEvent) Version() int                 { return b.Ver }
func (b BaseEvent) Timestamp() time.Time         { return b.Ts }

// OutboxEvent represents the structure of an event stored in the outbox table.
type OutboxEvent struct {
	EventID       uuid.UUID
	AggregateID   uuid.UUID
	AggregateType AggregateType
	EventType     string
	Payload       json.RawMessage
	Version       int
	Ts            time.Time
}
