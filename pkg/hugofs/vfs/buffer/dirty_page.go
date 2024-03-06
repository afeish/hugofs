package buffer

import (
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"go.uber.org/zap"
)

var (
	_ DirtyPage = (*MemBackedDirtyPage)(nil)
)

type DirtyPage interface {
	ReadAt(p []byte, off int64) (n int, err error)
	AddPage(p []byte, offset int64) (int, error)
	Flush() error
	Destroy()
	LockForRead(start, end int64)
	UnlockForRead(start, end int64)
	Idle(time.Duration) bool
}

type MemBackedDirtyPage struct {
	lastWriteNs int64
	ino         uint64

	mu        sync.RWMutex
	hasWrites bool

	pipeline *UploadPipeline
	lg       *zap.Logger

	opts meta.Options
}

func NewMemBackedDirtyPage(ino uint64, storage adaptor.StorageBackend, monitor PatternMonitor, lg *zap.Logger, opts ...Option[*meta.Options]) DirtyPage {
	opts = append(opts, meta.WithPageCtx(meta.GetWritePageCtx()))
	dp := &MemBackedDirtyPage{
		ino:  ino,
		lg:   lg,
		opts: meta.DefaultOptions,
	}
	ApplyOptions(&dp.opts, opts...)

	dp.pipeline = NewUploadPipeline(ino, storage, monitor, lg, opts...)
	return dp
}

func (dp *MemBackedDirtyPage) AddPage(p []byte, offset int64) (int, error) {
	dp.mu.Lock()
	dp.hasWrites = true
	dp.lastWriteNs = time.Now().UnixNano()
	dp.mu.Unlock()

	return dp.pipeline.WriteAt(p, offset)
}
func (dp *MemBackedDirtyPage) Flush() error {
	if !dp.HasWrites() {
		return nil
	}
	dp.lg.Debug("flushing dirty page", zap.Uint64("ino", dp.ino))
	dp.pipeline.FlushAll()
	return nil
}
func (dp *MemBackedDirtyPage) ReadAt(p []byte, off int64) (int, error) {
	if !dp.HasWrites() {
		return 0, nil
	}
	return dp.pipeline.ReadAt(p, off)
}

func (dp *MemBackedDirtyPage) HasWrites() bool {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return dp.hasWrites
}
func (dp *MemBackedDirtyPage) Destroy() {
	dp.pipeline.Destory()
}
func (dp *MemBackedDirtyPage) LockForRead(start, end int64) {
	dp.pipeline.LockForRead(start, end)
}
func (dp *MemBackedDirtyPage) UnlockForRead(start, end int64) {
	dp.pipeline.UnlockForRead(start, end)
}

func (dp *MemBackedDirtyPage) Idle(d time.Duration) bool {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return time.Since(time.Unix(0, dp.lastWriteNs)) > time.Minute
}
