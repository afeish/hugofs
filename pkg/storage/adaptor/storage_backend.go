package adaptor

import (
	"context"
	"sync"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
)

type StorageBackend interface {
	BatchRead(ctx context.Context, ino Ino, blockIdx []uint64) (map[uint64][]byte, error)
	Read(ctx context.Context, key BlockKey) ([]byte, error)
	Write(ctx context.Context, key BlockKey, data []byte) error
	BatchWrite(ctx context.Context, dataMap map[BlockKey][]byte) error
	WriteAt(ctx context.Context, key BlockKey, interval *IntervalList[*BlockWriteView], data []byte) error
	Delete(ctx context.Context, ino Ino) error
	WriteAsync(ctx context.Context, key BlockKey, data []byte) Future
	FlushAsync(ctx context.Context, ino Ino)
	Destory()
}

type Future interface {
	Await() error
	Done()
	IsDone() bool
}

type DefaultFuture struct {
	sync.WaitGroup
	done bool
}

func NewDefaultFuture() Future {
	f := &DefaultFuture{}
	f.Add(1)
	return f
}

func (f *DefaultFuture) Done() {
	f.WaitGroup.Add(-1)
	f.done = true
}

func (f *DefaultFuture) Await() error {
	f.WaitGroup.Wait()
	f.done = true
	return nil
}

func (f *DefaultFuture) IsDone() bool {
	return f.done
}
