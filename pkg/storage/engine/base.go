package engine

import (
	"context"
	"os"
	"strconv"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/block"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/spf13/afero"
)

//go:generate go run github.com/abice/go-enum -f=$GOFILE --marshal
// EngineType represent supported engine types
/** ENUM(
      FS, // local file	system
      INMEM, // in-memory
	  RPC, // remote
)
*/
type EngineType string

type Config struct {
	Prefix string
	Type   EngineType
	VolID  PhysicalVolumeID
}

var _ Engine = (*Base)(nil)

// Base TODO: implement the Engine interface with file system.
type Base struct {
	afero.Fs
	cfg *Config
}

func New[T interface {
	~struct {
		Engine
	}
	Engine
}](cfg *Config) Engine {
	base := &Base{cfg: cfg}
	creator := base.Creator()
	engine := creator(cfg)
	r := T{Engine: engine}
	return r
}

func (e *Base) Creator() func(cfg *Config) Engine {
	return func(cfg *Config) Engine {
		switch cfg.Type {
		case EngineTypeFS:
			fs := afero.NewOsFs()
			fs.MkdirAll(cfg.Prefix, os.ModePerm)
			e.Fs = afero.NewBasePathFs(fs, cfg.Prefix)
		case EngineTypeINMEM:
			e.Fs = afero.NewBasePathFs(afero.NewMemMapFs(), cfg.Prefix)
		case EngineTypeRPC:
		}
		return e

	}
}

func (b *Base) GetBlockHeader(ctx context.Context, location *Location) (*block.Header, error) {
	f, err := b.Fs.Open(b.ToAbs(location))
	if err != nil {
		return nil, ErrNotFound
	}
	defer f.Close()

	buf := make([]byte, block.HeaderSize)
	if n, err := f.ReadAt(buf, 0); err != nil || n < block.HeaderSize {
		return nil, ErrBlockCorrupted
	}
	return block.DecodeBlockHeader(buf)
}

func (b *Base) GetBlockData(ctx context.Context, location *Location) ([]byte, error) {
	return b.GetBlockDataCb(ctx, location, nil)
}

func (b *Base) GetBlockDataCb(ctx context.Context, location *Location, cb func(*block.Header) error) ([]byte, error) {
	_, span := tracing.Tracer.Start(ctx, "read block from engine")
	defer span.End()
	f, err := b.Fs.Open(b.ToAbs(location))
	if err != nil {
		return nil, ErrNotFound
	}
	defer f.Close()

	buf := make([]byte, block.HeaderSize)
	if n, err := f.ReadAt(buf, 0); err != nil || n < block.HeaderSize {
		return nil, ErrBlockCorrupted
	}
	header, err := block.DecodeBlockHeader(buf)
	if err != nil {
		return nil, err
	}
	buf = make([]byte, header.DataLen)
	if n, err := f.ReadAt(buf, block.HeaderSize); err != nil || n < int(header.DataLen) {
		return nil, ErrBlockCorrupted
	}
	if cb != nil {
		if err := cb(header); err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func (b *Base) WriteBlock(ctx context.Context, location *Location, data *DataContainer) error {
	return b.WriteBlockCb(ctx, location, data, nil)
}

func (b *Base) WriteBlockCb(ctx context.Context, location *Location, data *DataContainer, cb func(*block.Header) error) error {
	_, span := tracing.Tracer.Start(ctx, "write block to engine")
	defer span.End()

	if fi, _ := b.Fs.Stat(b.ToAbs(location)); fi != nil {
		return ErrBlockExisted
	}
	f, err := b.Fs.Create(b.ToAbs(location))
	if err != nil {
		return err
	}
	defer f.Close()
	var blk *block.Block
	if len(data.Hash) > 0 {
		blk, err = block.NewBlockWithHash(data.Index, data.Data, data.Hash)
	} else {
		blk, err = block.NewBlock(data.Index, data.Data)
	}
	if err != nil {
		return err
	}
	header, err := blk.Header()
	if err != nil {
		return err
	}
	_, err = f.Write(blk.Raw())
	if err != nil {
		return err
	}
	if cb != nil {
		return cb(header)
	}
	return nil
}

func (b *Base) CancelBlock(ctx context.Context, location *Location) error {
	f, err := b.Fs.Open(b.ToAbs(location))
	if err != nil {
		return ErrNotFound
	}
	defer f.Close()

	buf := make([]byte, block.HeaderSize)
	if n, err := f.ReadAt(buf, 0); err != nil || n < block.HeaderSize {
		return ErrBlockCorrupted
	}
	header, err := block.DecodeBlockHeader(buf)
	if err != nil {
		return ErrBlockCorrupted
	}
	header.Canceled = 1
	buf = block.EncodeBlockHeader(header)
	_, err = f.Write(buf)
	return err
}

func (b *Base) DeleteBlock(ctx context.Context, location *Location) error {
	name := b.ToAbs(location)
	if _, err := b.Fs.Stat(name); err != nil {
		return ErrBlockNotExisted
	}
	return b.Fs.Remove(name)
}

func (b *Base) ToAbs(location *Location) string {
	return strconv.Itoa(int(location.Slot))
}
