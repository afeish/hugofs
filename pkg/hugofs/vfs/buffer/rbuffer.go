package buffer

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/buffer/block"
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/util/size"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/zhangyunhao116/skipmap"
	"golang.org/x/sync/errgroup"
	"marwan.io/singleflight"

	"github.com/pingcap/log"
	"go.uber.org/zap"
)

const DefaultBlockSize = 4 * size.MebiByte

// BlockReadBuffer represents a read-write buffer for a file.
// It consists of a map of blocks and each block has a fixed size.
type BlockReadBuffer interface {
	Read(p []byte, off int) (count int, err error)
	Destroy() error
}

var _ BlockReadBuffer = (*DefaultBlockReadBuffer)(nil)

// DefaultBlockReadBuffer will update modifications to the underlying storage backend immediately.
type DefaultBlockReadBuffer struct {
	sync.RWMutex
	ino       uint64
	lastOpsNs int64 //last nano of read or write

	blockSize int64 // blockSize may be changed
	readPos   int   //last offset of read

	blocks *skipmap.OrderedMap[meta.BlockKey, meta.Block]

	storage adaptor.StorageBackend

	closeCh ClosableCh

	throttler   *Throttler
	subscribeCh <-chan meta.Event
	lg          *zap.Logger

	g            singleflight.Group[meta.Block]
	lockedKeySet mapset.Set[meta.BlockKey]

	monitor  PatternMonitor
	swapfile *block.SwapFile

	opts meta.Options
}

func NewDefaultBlockBuffer(memThrot *Throttler, ino Ino, blockSize int64, storage adaptor.StorageBackend, lg *zap.Logger, opts ...Option[*meta.Options]) BlockReadBuffer {
	envCfg := GetEnvCfg()
	dir := envCfg.Cache.Dir
	readCacheDir := path.Join(dir, "read-cache")
	os.MkdirAll(readCacheDir, os.ModePerm&^022)

	buffer := &DefaultBlockReadBuffer{
		ino:       ino,
		blockSize: blockSize,
		blocks:    skipmap.New[meta.BlockKey, meta.Block](),
		storage:   storage,

		closeCh: NewClosableCh(),

		throttler:    memThrot,
		subscribeCh:  memThrot.Register(ino),
		lg:           lg,
		lockedKeySet: mapset.NewSet[meta.BlockKey](),
		monitor:      NewReaderPattern(),
		opts:         meta.DefaultOptions,
	}
	opts = append([]Option[*meta.Options]{
		meta.WithBlockSize(int64(blockSize)),
		meta.WithCacheDir(readCacheDir),
		meta.WithCacheSize(int64(envCfg.Cache.FileReadSize)),
		meta.WithPageCtx(meta.GetReadPageCtx()),
	}, opts...)

	ApplyOptions[*meta.Options](&buffer.opts, opts...)
	buffer.swapfile = block.NewSwapFile(ino, storage, lg, opts...)

	buffer.init()

	return buffer
}
func (b *DefaultBlockReadBuffer) init() {
	go func() {
		for {
			select {
			case evt, ok := <-b.subscribeCh: // should be across the file
				if !ok {
					return
				}
				if evt.GetType() != meta.EventTypeEVICTREQ {
					continue
				}

				evict, ok := evt.(*meta.EvictBlockReqEvent)
				if !ok {
					continue
				}

				key := evict.GetKey()
				ino, _ := key.Split()
				if ino != b.ino {
					continue
				}
				b.evictBlock(key)
				b.throttler.Send(meta.NewEvictBlockConfirmEvent(key))
			case <-b.closeCh.ClosedCh():
				return
			}
		}
	}()

}

func (b *DefaultBlockReadBuffer) indexRange(p []byte, off int) meta.Range {
	// Determine the range of blocks to read.
	startBlockIdx := int64(off) / b.blockSize
	endBlockIdx := int64(off+len(p)) / b.blockSize
	if int64(off+len(p))%b.blockSize == 0 {
		endBlockIdx--
	}

	return meta.Range{
		Start: startBlockIdx,
		End:   endBlockIdx,
	}
}
func (b *DefaultBlockReadBuffer) Read(p []byte, off int) (n int, err error) {
	if cap(p) == 0 {
		log.Warn("dest is empty", zap.Int("offset", off))
		return 0, io.EOF
	}
	b.monitor.Monitor(int64(off), len(p))

	if !b.monitor.IsSequentialMode() && b.throttler.IsFull() { // random read
		// read from local cache first
		n, err = b.readFromLocal(p, off)
		if n > 0 {
			return
		}
		//read from remote directly
		return b.readFromRemote(p, off)
	}

	// Determine the range of blocks to read.
	indexRange := b.indexRange(p, off)
	if indexRange.End < 0 {
		return 0, io.EOF
	}

	var (
		read int // how many bytes can be read from the current block
	)
	// Iterate through each block in the range and read its contents.
	for blockIdx := indexRange.Start; blockIdx <= indexRange.End; {
		key := meta.NewBlockKey(b.ino, uint64(blockIdx))
		b.lockedKeySet.Add(key)
		_block, found := b.getBlock(key)
		if !found {
			// key.Lock()
			if _block, found = b.getBlock(key); found && _block != nil {
				// key.Unlock()
				goto readFlag
			}
			if !b.throttler.TryAcquire(key) {
				// key.Unlock()
				return
			}
			_block, err, _ = b.g.Do(key.String(), func() (meta.Block, error) {
				block := block.NewMemBlock(b.storage, key, int64(b.blockSize), b.lg)
				block.Cache()
				return block, block.Error()
			})
			if err != nil {
				if errors.Is(err, adaptor.ErrBlockNotFound) {
					err = io.EOF
				}
				// key.Unlock()
				return
			}

			b.setBlock(key, _block)
			// key.Unlock()
		}

	readFlag:
		read, err = _block.ReadAt(p[n:], int64(off))
		b.lg.Debug("read block", zap.Int("off", off), zap.Int("read", read), zap.Int("n", n+read), zap.Int("remain-cap", len(p[n+read:])), zap.String("data", size.ReadSummary(p[n:n+read], 10)))
		if err != nil {
			if errors.Is(err, adaptor.ErrBlockNotFound) {
				err = io.EOF
				return
			}
		}

		n += read
		off += read
		b.updateReadCursor(off)

		blockIdx++
	}

	if n < len(p) {
		b.lg.Debug("read less data than the provided buffer, EOF ...", zap.Int("off", off), zap.Int("n", n), zap.Int("read", read), zap.Int("cap", len(p[n:])))
		err = io.EOF
		return
	}

	return
}

func (b *DefaultBlockReadBuffer) readFromLocal(p []byte, off int) (n int, err error) {
	indexRange := b.indexRange(p, off)
	if indexRange.End < 0 {
		return 0, io.EOF
	}

	b.Lock()
	defer b.Unlock()

	var (
		read int // how many bytes can be read from the current block
	)
	// Iterate through each block in the range and read its contents.
	for blockIdx := indexRange.Start; blockIdx <= indexRange.End; {
		key := adaptor.NewBlockKey(b.ino, uint64(blockIdx))
		b.lockedKeySet.Add(key)
		block, found := b.getBlock(key)
		if !found || block == nil {
			block = b.swapfile.GetBlock(key)
			if block == nil {
				return 0, nil
			}
		}

		read, err = block.ReadAt(p[n:], int64(off))
		b.lg.Debug("read block from local", zap.Int("off", off), zap.Int("n", n), zap.Int("read", read), zap.Int("cap", len(p[n:])))
		if err != nil {
			if errors.Is(err, adaptor.ErrBlockNotFound) {
				err = io.EOF
				return
			}
		}
		n += read
		off += read
		b._updateReadCursor(off)

		if n >= len(p) { // read full
			return
		}
		blockIdx++
	}

	if n < len(p) { // not read enough
		b.lg.Debug("read less data than the provided buffer, EOF ...", zap.Int("off", off), zap.Int("n", n), zap.Int("read", read), zap.Int("remain-cap", len(p[n:])))
		err = io.EOF
		return
	}

	return
}

func (b *DefaultBlockReadBuffer) readFromRemote(p []byte, off int) (n int, err error) {
	indexRange := b.indexRange(p, off)
	if indexRange.End < 0 {
		return 0, io.EOF
	}

	var (
		read  int // how many bytes can be read from the current block
		block meta.Block
	)
	// Iterate through each block in the range and read its contents.
	for blockIdx := indexRange.Start; blockIdx <= indexRange.End; {
		key := meta.NewBlockKey(b.ino, uint64(blockIdx))

		block = b.swapfile.GetBlock(key)
		if block == nil {
			block, err = b.swapfile.NewFileBlock(key)
			if err != nil {
				return 0, err
			}

			block.Cache()
		}

		read, err = block.ReadAt(p[n:], int64(off))
		n += read

		if err != nil {
			if errors.Is(err, adaptor.ErrBlockNotFound) {
				err = io.EOF
				return
			}
		}
		off += read
		blockIdx++
	}

	if n < len(p) { // not read enough
		b.lg.Debug("read less data than the provided buffer, EOF ...", zap.Int("off", off), zap.Int("n", n), zap.Int("read", read), zap.Int("cap", len(p[n:])))
		err = io.EOF
		return
	}

	return
}
func (b *DefaultBlockReadBuffer) updateReadCursor(off int) {
	b.Lock()
	defer b.Unlock()
	b._updateReadCursor(off)
}

func (b *DefaultBlockReadBuffer) _updateReadCursor(off int) {
	b.readPos = off
	b.lastOpsNs = time.Now().UnixNano()
}

func (b *DefaultBlockReadBuffer) evictBlock(key meta.BlockKey) {
	block, found := b.getBlock(key)
	if !found {
		return
	}
	b.Lock()
	b.blocks.Delete(key)
	b.g.Forget(key.String())
	b.Unlock()

	block.Destroy()
}

func (b *DefaultBlockReadBuffer) getBlock(key meta.BlockKey) (meta.Block, bool) {
	return b.blocks.Load(key)
}

func (b *DefaultBlockReadBuffer) setBlock(key meta.BlockKey, block meta.Block) {
	b.blocks.Store(key, block)
}

func (b *DefaultBlockReadBuffer) Destroy() (err error) {
	b.closeCh.Close(func() {
		g, _ := errgroup.WithContext(context.Background())
		b.blocks.Range(func(key meta.BlockKey, block meta.Block) bool {
			g.Go(func() error {
				block.Destroy()
				return nil
			})
			return true
		})
		if err = g.Wait(); err != nil {
			return
		}

		if b.throttler != nil {
			b.throttler.UnRegister(b.ino)
		}

		b.lg.Debug("destroy read buffer, current block size", zap.Int("block-count", b.blocks.Len()))
		b.swapfile.FreeResource()

		meta.RemoveBlockKey(b.lockedKeySet)
	})
	return
}
