// Package event is used to implement the event system,
// which is aim to decouple feature logic from the primary logic.
package event

// The Event represents an event, it is the basic unit of the event system.
// This interface is used to decouple line-logic and event-logic.
type Event interface {
	// Name identifies the event.
	Name() string
	// Payload returns the event payload.
	Payload() []byte
	// Metadata returns the event metadata.
	Metadata() map[string]string
	// Before do something before an event really executes.
	Before(fn func() error) error
	// Execute executes the event.
	Execute(result Result) error
	// After do something after an event finished executing.
	After(fn func() error) error
}

// Result represents the result of an event.
// T is the excepted type of result.
type Result interface {
	IsOk() bool
	IsError() bool
	Ok() []byte
	Err() error
}
