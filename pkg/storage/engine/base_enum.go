// Code generated by go-enum DO NOT EDIT.
// Version:
// Revision:
// Build Date:
// Built By:

package engine

import (
	"errors"
	"fmt"
)

const (
	// EngineTypeFS is a EngineType of type FS.
	// local file	system
	EngineTypeFS EngineType = "FS"
	// EngineTypeINMEM is a EngineType of type INMEM.
	// in-memory
	EngineTypeINMEM EngineType = "INMEM"
	// EngineTypeRPC is a EngineType of type RPC.
	// remote
	EngineTypeRPC EngineType = "RPC"
)

var ErrInvalidEngineType = errors.New("not a valid EngineType")

// String implements the Stringer interface.
func (x EngineType) String() string {
	return string(x)
}

// String implements the Stringer interface.
func (x EngineType) IsValid() bool {
	_, err := ParseEngineType(string(x))
	return err == nil
}

var _EngineTypeValue = map[string]EngineType{
	"FS":    EngineTypeFS,
	"INMEM": EngineTypeINMEM,
	"RPC":   EngineTypeRPC,
}

// ParseEngineType attempts to convert a string to a EngineType.
func ParseEngineType(name string) (EngineType, error) {
	if x, ok := _EngineTypeValue[name]; ok {
		return x, nil
	}
	return EngineType(""), fmt.Errorf("%s is %w", name, ErrInvalidEngineType)
}

// MarshalText implements the text marshaller method.
func (x EngineType) MarshalText() ([]byte, error) {
	return []byte(string(x)), nil
}

// UnmarshalText implements the text unmarshaller method.
func (x *EngineType) UnmarshalText(text []byte) error {
	tmp, err := ParseEngineType(string(text))
	if err != nil {
		return err
	}
	*x = tmp
	return nil
}
