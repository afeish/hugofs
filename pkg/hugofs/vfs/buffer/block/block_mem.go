package block

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/util/mem"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

var (
	_ meta.Block = (*MemBlock)(nil)

	ErrInvalidWriteOffset = errors.New("invalid write offset")
	ErrInvalidReadOffset  = errors.New("invalid read offset")

	memBlockCounter int64
)

type MemBlock struct {
	sync.RWMutex

	blockSize int64
	highOff   int64 // largest written offset in the block

	written      int64
	lastUsedTime int64

	storage     adaptor.StorageBackend
	key         meta.BlockKey
	data        []byte
	flushed     bool
	writtenMode bool

	err error

	wg sync.WaitGroup

	closable ClosableCh

	interval *adaptor.IntervalList[*adaptor.BlockWriteView]
	lg       *zap.Logger
}

func NewMemBlockForWrite(storage adaptor.StorageBackend, blockKey meta.BlockKey, blockSize int64, lg *zap.Logger) *MemBlock {
	counter := atomic.LoadInt64(&memBlockCounter)
	lg.Sugar().Debugf("%s memBlockCounter %d ++> %d", blockKey, counter, counter+1)

	atomic.AddInt64(&memBlockCounter, 1)
	return &MemBlock{
		storage:   storage,
		blockSize: blockSize,
		key:       blockKey,
		interval:  adaptor.NewIntervalList[*adaptor.BlockWriteView](),
		lg:        lg,
		// cacheStartedCh: make(chan struct{}, 1),
		writtenMode: true,
		closable:    NewClosableCh(),
	}
}

func NewMemBlock(storage adaptor.StorageBackend, blockKey meta.BlockKey, blockSize int64, lg *zap.Logger) *MemBlock {
	return &MemBlock{
		storage:   storage,
		blockSize: blockSize,
		key:       blockKey,
		interval:  adaptor.NewIntervalList[*adaptor.BlockWriteView](),
		lg:        lg.Named("memblock"),
		closable:  NewClosableCh(),
	}
}

func (b *MemBlock) GetData() []byte {
	b.RLock()
	defer b.RUnlock()
	return b.data
}

func (b *MemBlock) IsFlushed() bool {
	b.RLock()
	defer b.RUnlock()
	return b.flushed
}

func (b *MemBlock) ShouldFlush() bool {
	b.RLock()
	defer b.RUnlock()
	return b.written > 0 && !b.flushed
}

func (b *MemBlock) Flush() error {
	b.Lock()
	defer b.Unlock()

	return b.doFlush()
}

func (b *MemBlock) Error() error {
	b.RLock()
	defer b.RUnlock()
	return b.err
}

func (b *MemBlock) doFlush() error {
	if b.flushed || b.written == 0 || b.storage == nil {
		return nil
	}
	b.interval.Merge()
	var (
		err error
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	if b.interval.Total() == b.blockSize {
		b.lg.Debug("flush to remote", zap.String("block", b.key.String()), zap.String("written-size", humanize.IBytes(uint64(b.written))), zap.String("interval", b.interval.String()))

		b.storage.Write(ctx, b.key, b.data)
	} else {
		b.lg.Debug("flush to partial", zap.String("block", b.key.String()), zap.String("written-size", humanize.IBytes(uint64(b.written))), zap.String("interval", b.interval.String()))
		err = b.storage.WriteAt(ctx, b.key, b.interval, b.data)
		if err != nil && !strings.Contains(err.Error(), "block already exists") {
			b.lg.Error("err flush partial block to remote", zap.String("block", b.key.String()), zap.String("written-size", humanize.IBytes(uint64(b.written))), zap.Error(err))
			return err
		}
	}

	b.flushed = true
	b.written = 0
	return nil
}

func (b *MemBlock) IsFull() bool {
	b.Lock()
	defer b.Unlock()
	return b.interval.Total() == b.blockSize
}

func (b *MemBlock) reset() {
	b._doCache()
}

func (b *MemBlock) _doCache() {
	if b.storage == nil {
		return
	}
	data, err := b.storage.Read(context.Background(), b.key)
	if err != nil {
		// b.lg.Error("err read block from remote", zap.String("block", b.key.String()), zap.Error(err))
		if cap(b.data) > 0 {
			mem.Free(b.data)
		}
		b.data = nil
		b.err = err
		return
	}

	b.lg.Debug("read block from remote", zap.String("block", b.key.String()), zap.String("data", size.ReadSummary(data, 10)))

	highOff := 0
	if len(data) != int(b.blockSize) {
		b.data = mem.Allocate(int(b.blockSize))
		highOff = copy(b.data, data)
		if mem.IsAllBytesZero(data) {
			panic("block data null at memblock mismatch")
		}
	} else {
		b.data = data
		highOff = len(data)
	}
	b.written = 0
	b.highOff = int64(highOff)
	atomic.StoreInt64(&b.lastUsedTime, time.Now().UnixNano())
}

func (b *MemBlock) ReadAt(p []byte, off int64) (n int, err error) {
	b.wg.Add(1)
	defer b.wg.Done()
	b.RLock()
	defer b.RUnlock()

	blockSize := b.blockSize
	_, index := b.key.Split()
	blockIdx := BlockIndexInFile(off / blockSize)
	if blockIdx != index {
		return 0, ErrInvalidReadOffset
	}
	startInBlock := off % blockSize

	if b.err != nil {
		if b.err == ErrBlockDestoryed {
			b.reset()
		} else {
			return 0, b.err
		}
	}

	endInBlock := Min(b.highOff, startInBlock+int64(len(p)), int64(blockIdx+1)*blockSize)

	blockStart := index * uint64(blockSize)
	blockLimit := blockStart + uint64(b.highOff)
	if startInBlock >= endInBlock || startInBlock > int64(len(b.data)) {
		b.lg.Error("found not enough data", zap.String("block", b.key.String()), zap.Uint64("blockStart", blockStart), zap.Uint64("blockLimit", blockLimit), zap.Int64("readOff", off), zap.Int64("readEnd", off+int64(endInBlock-startInBlock)),
			zap.Int64("startInBlock", startInBlock),
			zap.Int64("endInBlock", endInBlock),
			zap.Int64("highOff", b.highOff),
			zap.Int("len-data", len(b.data)),
		)
		return 0, io.EOF
	}
	n = copy(p, b.data[startInBlock:endInBlock])
	// b.lg.Debug("found dirty page data", zap.String("block", b.key.String()), zap.Uint64("blockStart", blockStart),
	// 	zap.Uint64("blockLimit", blockLimit), zap.Int64("readOff", off), zap.Int64("readEnd", off+int64(n)),
	// 	zap.String("origin", size.ReadSummary(b.data, 10)), zap.String("data", size.ReadSummary(p[:n], 10)))
	b.lastUsedTime = time.Now().UnixNano()
	return
}

func (b *MemBlock) WriteAt(p []byte, off int64) (n int, err error) {
	b.wg.Add(1)
	defer b.wg.Done()

	blockSize := b.blockSize
	_, index := b.key.Split()

	blockIdx := BlockIndexInFile(off / blockSize)
	if blockIdx != index {
		return 0, ErrInvalidWriteOffset
	}
	blockStart := off % blockSize
	blockEnd := Min(blockStart+int64(len(p)), blockSize)

	n = int(blockEnd - blockStart)

	b.Lock()
	if b.data == nil {
		b.data = mem.Allocate(int(blockSize))
	}
	copy(b.data[blockStart:blockEnd], p[:n])

	it := adaptor.NewInterval[*adaptor.BlockWriteView](blockStart, blockEnd)
	it.Value.SetStartStop(blockStart, blockEnd)
	b.interval.AddInterval(it)

	written := b.written + int64(n)
	b.written = written
	b.highOff = Max(b.highOff, blockEnd)
	b.lastUsedTime = time.Now().UnixNano()
	b.Unlock()

	if written >= b.blockSize {
		err = b.Flush()
	}
	return
}

func (b *MemBlock) Cache() {
	b.Lock()
	b.wg.Add(1)
	defer func() {
		if b.err == ErrBlockDestoryed {
			b.err = nil
		}
		b.wg.Done()
		b.Unlock()
	}()

	b._doCache()
}

func (b *MemBlock) Written() int64 {
	b.RLock()
	defer b.RUnlock()
	return b.interval.Total()
}

func (b *MemBlock) Destroy() {
	b.closable.Close(func() {
		// wait for all reads to finish before destroying the data
		b.wg.Wait()
		b.Lock()
		defer b.Unlock()

		if cap(b.data) > 0 {
			mem.Free(b.data)
		}
		// log.Debug("destroy block cache", zap.String("key", string(b.key)), zap.String("mem-used", humanize.IBytes(uint64(mem.AllocMemory()))))
		b.data = nil

		if b.writtenMode {
			counter := atomic.LoadInt64(&memBlockCounter)
			b.lg.Sugar().Debugf("%s memBlockCounter %d --> %d", b.key, counter, counter-1)
			atomic.AddInt64(&memBlockCounter, -1)
		}

		b.lg.Debug("destory block", zap.String("block", b.key.String()))
		b.err = ErrBlockDestoryed
	})
}

func (b *MemBlock) IsDestroyed() bool {
	return b.closable.IsClosed()
}
