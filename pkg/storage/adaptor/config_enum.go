// Code generated by go-enum DO NOT EDIT.
// Version:
// Revision:
// Build Date:
// Built By:

package adaptor

import (
	"errors"
	"fmt"
)

const (
	// TransportTypeINVALID is a TransportType of type INVALID.
	TransportTypeINVALID TransportType = "INVALID"
	// TransportTypeGRPC is a TransportType of type GRPC.
	TransportTypeGRPC TransportType = "GRPC"
	// TransportTypeTCP is a TransportType of type TCP.
	TransportTypeTCP TransportType = "TCP"
	// TransportTypeMOCK is a TransportType of type MOCK.
	TransportTypeMOCK TransportType = "MOCK"
)

var ErrInvalidTransportType = errors.New("not a valid TransportType")

// String implements the Stringer interface.
func (x TransportType) String() string {
	return string(x)
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x TransportType) IsValid() bool {
	_, err := ParseTransportType(string(x))
	return err == nil
}

var _TransportTypeValue = map[string]TransportType{
	"INVALID": TransportTypeINVALID,
	"GRPC":    TransportTypeGRPC,
	"TCP":     TransportTypeTCP,
	"MOCK":    TransportTypeMOCK,
}

// ParseTransportType attempts to convert a string to a TransportType.
func ParseTransportType(name string) (TransportType, error) {
	if x, ok := _TransportTypeValue[name]; ok {
		return x, nil
	}
	return TransportType(""), fmt.Errorf("%s is %w", name, ErrInvalidTransportType)
}

// MarshalText implements the text marshaller method.
func (x TransportType) MarshalText() ([]byte, error) {
	return []byte(string(x)), nil
}

// UnmarshalText implements the text unmarshaller method.
func (x *TransportType) UnmarshalText(text []byte) error {
	tmp, err := ParseTransportType(string(text))
	if err != nil {
		return err
	}
	*x = tmp
	return nil
}
