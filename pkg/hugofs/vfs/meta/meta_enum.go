// Code generated by go-enum DO NOT EDIT.
// Version:
// Revision:
// Build Date:
// Built By:

package meta

import (
	"errors"
	"fmt"
)

const (
	// EventTypeUPDATE is a EventType of type UPDATE.
	// update of a block info
	EventTypeUPDATE EventType = "UPDATE"
	// EventTypeEVICTREQ is a EventType of type EVICT_REQ.
	// eviction requsst of a block
	EventTypeEVICTREQ EventType = "EVICT_REQ"
	// EventTypeEVICTCONFIRM is a EventType of type EVICT_CONFIRM.
	// confirmation of a block eviction
	EventTypeEVICTCONFIRM EventType = "EVICT_CONFIRM"
)

var ErrInvalidEventType = errors.New("not a valid EventType")

// String implements the Stringer interface.
func (x EventType) String() string {
	return string(x)
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x EventType) IsValid() bool {
	_, err := ParseEventType(string(x))
	return err == nil
}

var _EventTypeValue = map[string]EventType{
	"UPDATE":        EventTypeUPDATE,
	"EVICT_REQ":     EventTypeEVICTREQ,
	"EVICT_CONFIRM": EventTypeEVICTCONFIRM,
}

// ParseEventType attempts to convert a string to a EventType.
func ParseEventType(name string) (EventType, error) {
	if x, ok := _EventTypeValue[name]; ok {
		return x, nil
	}
	return EventType(""), fmt.Errorf("%s is %w", name, ErrInvalidEventType)
}

// MarshalText implements the text marshaller method.
func (x EventType) MarshalText() ([]byte, error) {
	return []byte(string(x)), nil
}

// UnmarshalText implements the text unmarshaller method.
func (x *EventType) UnmarshalText(text []byte) error {
	tmp, err := ParseEventType(string(text))
	if err != nil {
		return err
	}
	*x = tmp
	return nil
}
