package meta

import (
	"github.com/afeish/hugo/pkg/storage/adaptor"
)

type BlockKey = adaptor.BlockKey

var (
	NewBlockKey    = adaptor.NewBlockKey
	RemoveBlockKey = adaptor.RemoveBlockKey
)

//go:generate go run github.com/abice/go-enum -f=$GOFILE --marshal
// EventType represent supported engine types
/** ENUM(
      UPDATE, // update of a block info
      EVICT_REQ, // eviction requsst of a block
	  EVICT_CONFIRM // confirmation of a block eviction
)
*/
type EventType string

type Event interface {
	GetType() EventType
	GetKey() BlockKey
}

type EvictBlockConfirmEvent struct {
	blockKey BlockKey
}

func NewEvictBlockConfirmEvent(k BlockKey) *EvictBlockConfirmEvent {
	return &EvictBlockConfirmEvent{blockKey: k}
}

func (e *EvictBlockConfirmEvent) GetType() EventType {
	return EventTypeEVICTCONFIRM
}

func (e *EvictBlockConfirmEvent) GetKey() BlockKey {
	return e.blockKey
}

type EvictBlockReqEvent struct {
	blockKey BlockKey
}

func NewEvictBlockReqEvent(k BlockKey) *EvictBlockReqEvent {
	return &EvictBlockReqEvent{blockKey: k}
}

func (e *EvictBlockReqEvent) GetType() EventType {
	return EventTypeEVICTREQ
}

func (e *EvictBlockReqEvent) GetKey() BlockKey {
	return e.blockKey
}

type UpdateBlockEvent struct {
	blockKey     BlockKey
	lastUsedTime int64
}

func (e *UpdateBlockEvent) GetType() EventType {
	return EventTypeUPDATE
}

func (e *UpdateBlockEvent) GetTime() int64 {
	return e.lastUsedTime
}
func (e *UpdateBlockEvent) GetKey() BlockKey {
	return e.blockKey
}
