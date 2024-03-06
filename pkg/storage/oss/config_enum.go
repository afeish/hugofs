// Code generated by go-enum DO NOT EDIT.
// Version:
// Revision:
// Build Date:
// Built By:

package oss

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// PlatformInfoINVALID is a PlatformInfo of type INVALID.
	PlatformInfoINVALID PlatformInfo = iota
	// PlatformInfoAWS is a PlatformInfo of type AWS.
	PlatformInfoAWS
	// PlatformInfoLOCAL is a PlatformInfo of type LOCAL.
	PlatformInfoLOCAL
	// PlatformInfoTENCENT is a PlatformInfo of type TENCENT.
	PlatformInfoTENCENT
	// PlatformInfoBAIDU is a PlatformInfo of type BAIDU.
	PlatformInfoBAIDU
	// PlatformInfoORACLE is a PlatformInfo of type ORACLE.
	PlatformInfoORACLE
	// PlatformInfoMEMORY is a PlatformInfo of type MEMORY.
	PlatformInfoMEMORY
)

var ErrInvalidPlatformInfo = errors.New("not a valid PlatformInfo")

const _PlatformInfoName = "INVALIDAWSLOCALTENCENTBAIDUORACLEMEMORY"

var _PlatformInfoMap = map[PlatformInfo]string{
	PlatformInfoINVALID: _PlatformInfoName[0:7],
	PlatformInfoAWS:     _PlatformInfoName[7:10],
	PlatformInfoLOCAL:   _PlatformInfoName[10:15],
	PlatformInfoTENCENT: _PlatformInfoName[15:22],
	PlatformInfoBAIDU:   _PlatformInfoName[22:27],
	PlatformInfoORACLE:  _PlatformInfoName[27:33],
	PlatformInfoMEMORY:  _PlatformInfoName[33:39],
}

// String implements the Stringer interface.
func (x PlatformInfo) String() string {
	if str, ok := _PlatformInfoMap[x]; ok {
		return str
	}
	return fmt.Sprintf("PlatformInfo(%d)", x)
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x PlatformInfo) IsValid() bool {
	_, ok := _PlatformInfoMap[x]
	return ok
}

var _PlatformInfoValue = map[string]PlatformInfo{
	_PlatformInfoName[0:7]:   PlatformInfoINVALID,
	_PlatformInfoName[7:10]:  PlatformInfoAWS,
	_PlatformInfoName[10:15]: PlatformInfoLOCAL,
	_PlatformInfoName[15:22]: PlatformInfoTENCENT,
	_PlatformInfoName[22:27]: PlatformInfoBAIDU,
	_PlatformInfoName[27:33]: PlatformInfoORACLE,
	_PlatformInfoName[33:39]: PlatformInfoMEMORY,
}

// ParsePlatformInfo attempts to convert a string to a PlatformInfo.
func ParsePlatformInfo(name string) (PlatformInfo, error) {
	if x, ok := _PlatformInfoValue[name]; ok {
		return x, nil
	}
	return PlatformInfo(0), fmt.Errorf("%s is %w", name, ErrInvalidPlatformInfo)
}

// MarshalText implements the text marshaller method.
func (x PlatformInfo) MarshalText() ([]byte, error) {
	return []byte(x.String()), nil
}

// UnmarshalText implements the text unmarshaller method.
func (x *PlatformInfo) UnmarshalText(text []byte) error {
	name := string(text)
	tmp, err := ParsePlatformInfo(name)
	if err != nil {
		return err
	}
	*x = tmp
	return nil
}