package engine

import (
	"context"
	"errors"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/block"
)

// Engine is the store engine for saving the blocks.
// It works like a key-value store.
//
// All magic technology should implement in the engine.
type Engine interface {
	GetBlockData(ctx context.Context, location *Location) ([]byte, error)
	GetBlockDataCb(ctx context.Context, location *Location, cb func(*block.Header) error) ([]byte, error)
	WriteBlock(ctx context.Context, location *Location, data *DataContainer) error
	WriteBlockCb(ctx context.Context, location *Location, data *DataContainer, cb func(*block.Header) error) error
	GetBlockHeader(ctx context.Context, location *Location) (*block.Header, error)
	CancelBlock(ctx context.Context, location *Location) error
	DeleteBlock(ctx context.Context, location *Location) error
}

type DataContainer struct {
	Index uint64
	Data  []byte
	Hash  []byte
	Crc   uint32
}

type Location struct {
	VolID PhysicalVolumeID
	Slot  uint64
}

func NewLocation(id PhysicalVolumeID, slot uint64) *Location {
	return &Location{
		VolID: id,
		Slot:  slot,
	}
}

var (
	ErrNotFound        = errors.New("not found")
	ErrBlockCorrupted  = errors.New("block is corrupted")
	ErrBlockExisted    = errors.New("block already exists")
	ErrBlockNotExisted = errors.New("block not exists")
)
