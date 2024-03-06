package gateway

import (
	"context"
	"sync"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"go.uber.org/zap"
)

var (
	_ adaptor.StorageBackend = (*LockedCoordinator)(nil)
)

// for test concurrent use only
type LockedCoordinator struct {
	mu sync.RWMutex
	VolumeCoordinator
}

func NewLockedVolumeCoordinator(ctx context.Context, cfg *adaptor.Config, lg *zap.Logger) (*LockedCoordinator, error) {
	delegate, err := NewVolumeCoordinator(ctx, cfg, lg)
	if err != nil {
		return nil, err
	}
	return &LockedCoordinator{
		VolumeCoordinator: *delegate,
	}, nil
}

func (c *LockedCoordinator) BatchRead(ctx context.Context, ino Ino, blockIdx []uint64) (map[uint64][]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.VolumeCoordinator.BatchRead(ctx, ino, blockIdx)
}
func (c *LockedCoordinator) Read(ctx context.Context, key adaptor.BlockKey) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.VolumeCoordinator.Read(ctx, key)
}
func (c *LockedCoordinator) Write(ctx context.Context, key adaptor.BlockKey, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.VolumeCoordinator.Write(ctx, key, data)
}
func (c *LockedCoordinator) BatchWrite(ctx context.Context, dataMap map[adaptor.BlockKey][]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.VolumeCoordinator.BatchWrite(ctx, dataMap)
}
func (c *LockedCoordinator) WriteAt(ctx context.Context, key adaptor.BlockKey, interval *adaptor.IntervalList[*adaptor.BlockWriteView], data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.VolumeCoordinator.WriteAt(ctx, key, interval, data)
}
func (c *LockedCoordinator) Delete(ctx context.Context, ino Ino) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.VolumeCoordinator.Delete(ctx, ino)
}
