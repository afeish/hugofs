package adaptor

import (
	"context"
	"errors"
	"sync"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/util/mem"
)

var (
	ErrBlockNotFound = errors.New("block not found")

	_ StorageBackend = (*MemStorageBackend)(nil)
)

type MemStorageBackend struct {
	mu        sync.RWMutex
	blockSize int64
	backed    map[BlockKey][]byte
	ino2key   map[Ino][]BlockKey
}

func NewMemStorageBackend(blockSize int64) StorageBackend {

	return &MemStorageBackend{
		blockSize: blockSize,
		backed:    make(map[BlockKey][]byte),
		ino2key:   make(map[uint64][]BlockKey),
	}
}

func (b *MemStorageBackend) BatchRead(ctx context.Context, ino Ino, blockIdx []uint64) (map[uint64][]byte, error) {
	return nil, nil
}

// right now, mem storage backend may return nil data, so it's not safe for test
func (b *MemStorageBackend) Read(ctx context.Context, key BlockKey) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	r, found := b.backed[key]
	if !found {
		return nil, ErrBlockNotFound
	}
	return r, nil
}
func (b *MemStorageBackend) Write(ctx context.Context, key BlockKey, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if mem.IsAllBytesZero(data) {
		panic("write null data at Write")
	}
	return b.doWrite(key, data)
}

func (b *MemStorageBackend) doWrite(key BlockKey, data []byte) error {
	ino := key.GetIno()
	ka, found := b.ino2key[ino]
	if !found {
		ka = make([]BlockKey, 0)
	}

	off, remaining := int64(0), int64(len(data))
	for remaining > 0 {
		end := Min(b.blockSize, remaining)
		b.ensureBlockExists(key)
		orig := b.backed[key]
		copy(orig, data[off:end])
		ka = append(ka, key)

		remaining -= (end - off)
		off = end
		key = NewBlockKey(ino, key.GetIndex()+1)
	}

	b.ino2key[ino] = ka
	return nil
}

func (b *MemStorageBackend) ensureBlockExists(key BlockKey) {
	if b.backed[key] == nil {
		b.backed[key] = make([]byte, b.blockSize)
	}
}

func (b *MemStorageBackend) BatchWrite(ctx context.Context, dataMap map[BlockKey][]byte) error {
	return nil
}
func (b *MemStorageBackend) WriteAt(ctx context.Context, key BlockKey, interval *IntervalList[*BlockWriteView], data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, found := b.backed[key]
	if !found {
		return b.doWrite(key, data)
	}

	_, err := interval.Copy(r, data)
	return err
}
func (b *MemStorageBackend) Delete(ctx context.Context, ino Ino) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	ka, found := b.ino2key[ino]
	if !found {
		return nil
	}
	for _, k := range ka {
		delete(b.backed, k)
	}
	delete(b.ino2key, ino)
	return nil
}
func (b *MemStorageBackend) WriteAsync(ctx context.Context, key BlockKey, data []byte) Future {
	return nil
}
func (b *MemStorageBackend) FlushAsync(ctx context.Context, ino Ino) {
}
func (b *MemStorageBackend) Destory() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ino2key = make(map[uint64][]BlockKey)
	b.backed = make(map[BlockKey][]byte)
}
