package block

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/buffer/page"
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"

	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/util/executor"
	"github.com/afeish/hugo/pkg/util/freelist"
	"github.com/afeish/hugo/pkg/util/mem"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/zhangyunhao116/skipmap"
	"go.uber.org/zap"
)

var (
	_ meta.Block = (*FileBlock)(nil)

	ErrBlockDestoryed = errors.New("block is destroyed")
)

type fileBlocks []*FileBlock

func (ws fileBlocks) Len() int { return len(ws) }

func (ws fileBlocks) Less(i, j int) bool {
	a, b := ws[i], ws[j]

	return a.totalWritten() > b.totalWritten()
}

func (ws fileBlocks) Swap(i, j int) {
	ws[i], ws[j] = ws[j], ws[i]
	ws[i].index = i
	ws[j].index = j
}

func (ws fileBlocks) Find(key meta.BlockKey) int {
	for index, item := range ws {
		if item.meta.key == key {
			return index
		}
	}
	return -1
}

func (ws *fileBlocks) Push(x interface{}) {
	n := len(*ws)
	item := x.(*FileBlock)
	item.index = n
	*ws = append(*ws, item)
}

func (ws *fileBlocks) Pop() interface{} {
	old := *ws
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*ws = old[0 : n-1]
	return item
}

func (ws *fileBlocks) String() string {
	var sb strings.Builder
	for i, item := range *ws {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(item.meta.key.String() + ":" + humanize.IBytes(uint64(item.totalWritten())))
	}
	return sb.String()
}

type FileBlock struct {
	mu       sync.RWMutex
	swapFile *SwapFile
	meta     *SwapBlockMeta
	highOff  int64
	written  int64

	index int
	err   error

	wg sync.WaitGroup

	pages    *skipmap.OrderedMap[int64, meta.Page]
	interval *adaptor.IntervalList[*adaptor.BlockWriteView]

	flushed  bool
	closable ClosableCh
	lg       *zap.Logger

	storage adaptor.StorageBackend

	cacheStartedCh chan struct{}

	lastUsedTime int64
}

func NewFileBlock(swapFile *SwapFile, blockMeta *SwapBlockMeta, lg *zap.Logger) *FileBlock {
	return &FileBlock{
		swapFile: swapFile,
		meta:     blockMeta,
		lg:       lg.Named("fileblock"),
		interval: adaptor.NewIntervalList[*adaptor.BlockWriteView](),
		pages:    skipmap.New[int64, meta.Page](),
		closable: NewClosableCh(),

		storage:        swapFile.storage,
		cacheStartedCh: make(chan struct{}),
	}
}

func (b *FileBlock) Cache() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b._doCache()
}

func (b *FileBlock) _doCache() {
	b.wg.Add(1)
	defer func() {
		if b.err == ErrBlockDestoryed {
			b.err = nil
		}
		b.wg.Done()
	}()

	if b.storage == nil {
		return
	}
	b.flushed = true // no need to flush

	b.lg.Debug("read block from remote", zap.String("block", b.meta.key.String()))
	data, err := b.storage.Read(context.Background(), b.meta.key)
	if err != nil {
		b.err = err
		b.lg.Error("read block from remote error", zap.Error(err))
		return
	}
	_, err = b.writeToPage(data, int64(b.meta.key.GetIndex())*b.meta.blockSize)
	if err != nil {
		b.err = err
		return
	}
	atomic.StoreInt64(&b.lastUsedTime, time.Now().UnixNano())
}

func (b *FileBlock) Error() error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.err
}

func (b *FileBlock) WriteAt(p []byte, off int64) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.wg.Add(1)
	defer b.wg.Done()

	blockSize := b.meta.blockSize
	_, index := b.meta.key.Split()

	blockIdx := BlockIndexInFile(off / blockSize)
	if blockIdx != index {
		return 0, ErrInvalidWriteOffset
	}
	blockStart := off % blockSize
	blockEnd := Min(blockStart+int64(len(p)), blockSize)

	n = int(blockEnd - blockStart)

	// b.lg.Info("write data to fileBlock", zap.Int("len", n), zap.Int64("written", b.written+int64(n)), zap.Int64("blockSize", b.meta.blockSize), zap.Int64("offset", (b.meta.index*blockSize)+blockStart))
	n, err = b.writeToPage(p, off)

	if err != nil {
		return
	}

	it := adaptor.NewInterval[*adaptor.BlockWriteView](blockStart, blockEnd)
	it.Value.SetStartStop(blockStart, blockEnd)
	b.interval.AddInterval(it)

	written := b.written + int64(n)
	b.written = written
	b.highOff = Max(b.highOff, blockEnd)

	if written >= b.meta.blockSize {
		err = b.doFlush()
		return
	}
	b.flushed = false
	return
}

func (b *FileBlock) writeToPage(p []byte, off int64) (n int, err error) {
	pageSize := b.meta.pageSize
	pageIdx := off / pageSize
	var (
		actualN int
	)
	for ; len(p) > 0; pageIdx++ {
		pageOff := off % pageSize
		expectN := Min(int64(len(p)), (pageIdx+1)*pageSize-off)

		_page, found := b.pages.Load(pageIdx)

		if !found {
			// b.lg.Sugar().Debugf("acquiring offset for page idx %d, current page is %d, pageSize is %s, page is %s\n", pageIdx, b.swapFile.opts.pageCtx.GetPages(), humanize.IBytes(uint64(b.swapFile.opts.pageSize)), string(p))
			b.swapFile.OccupyPage()
			offsetInFile := b.swapFile.AcquireOffset()
			_page = page.NewFilePage(b.swapFile.opts.PageCtx, b.swapFile.file, offsetInFile, pageSize, b.lg)
			b.pages.Store(pageIdx, _page)
		}

		// rg := page.Range()
		// b.lg.Sugar().Debugf("write to file range %s --- %s", humanize.IBytes(uint64(rg.Start)), humanize.IBytes(uint64(rg.End)))
		actualN, err = _page.WriteAt(p[:expectN], pageOff)
		if err != nil {
			return
		}

		p = p[actualN:]
		off += int64(actualN)
		n += int(actualN)
	}
	return
}
func (b *FileBlock) ReadAt(p []byte, off int64) (n int, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	b.wg.Add(1)
	defer b.wg.Done()
	blockSize := b.meta.blockSize
	_, index := b.meta.key.Split()
	blockIdx := BlockIndexInFile(off / blockSize)
	offLimit := int64(blockIdx+1) * blockSize
	if blockIdx != index {
		return 0, ErrInvalidReadOffset
	}

	if b.err != nil {
		if b.err == ErrBlockDestoryed {
			b.reset()
		} else {
			return 0, b.err
		}
	}

	pageSize := b.meta.pageSize
	startOff := off
	for len(p) > 0 && off < offLimit {
		pageIdx := off / int64(pageSize)
		page, ok := b.pages.Load(pageIdx)
		pageOff := off % pageSize

		var actualN int
		var expectN int64 = Min(offLimit-off, int64(len(p)), int64(pageSize-pageOff))
		if !ok { // zero it out
			// b.lg.Debug("zeroing out page", zap.String("block", b.meta.key.String()), zap.Int64("pageIdx", pageIdx), zap.Int64("off", off))
			actualN = copy(p[:expectN], make([]byte, expectN))
		} else {
			actualN, err = page.ReadAt(p[:expectN], pageOff)
			if err != nil {
				b.lg.Error("page read error", zap.Int64("off", startOff), zap.Int("n", n), zap.Error(err))
				n += int(actualN)
				break
			}
		}

		off += int64(actualN)
		n += int(actualN)
		p = p[actualN:]
	}
	if len(p) > 0 {
		b.lg.Debug("file block read less data than the provided buffer, EOF ...", zap.Int64("off", startOff), zap.Int("n", n), zap.Int("remain-cap", len(p)))
		err = io.EOF
		return
	}
	return
}

func (b *FileBlock) IsFull() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.interval.Total() == b.meta.blockSize
}

func (b *FileBlock) Written() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.written
}

func (b *FileBlock) IsDestroyed() bool {
	return b.closable.IsClosed()
}

func (b *FileBlock) Destroy() {
	b.closable.Close(func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		b.wg.Wait()
		if err := b.doFlush(); err != nil {
			//TODO: rpc error: code = Unknown desc = write block
			b.lg.Error("flush failed", zap.String("block", string(b.meta.key)), zap.Error(err))
		}
		b.pages.Range(func(idx int64, p meta.Page) bool {
			b.pages.Delete(idx)
			p.Free()
			pageStart := p.Range().Start
			b.swapFile.freePage(pageStart)
			return true
		})

		b.lg.Debug("destory block", zap.String("block", b.meta.key.String()))
		b.err = ErrBlockDestoryed
	})
}

func (b *FileBlock) reset() {
	b._doCache()
}

func (b *FileBlock) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.flushed {
		return nil
	}

	return b.doFlush()
}

func (b *FileBlock) totalWritten() int64 {
	var total int64
	b.pages.Range(func(idx int64, p meta.Page) bool {
		total += p.GetSize()
		return true
	})

	return total
}

func (b *FileBlock) doFlush() error {
	if b.flushed || b.written == 0 {
		return nil
	}

	buffer := mem.Allocate(int(b.meta.blockSize))
	defer mem.Free(buffer)

	var (
		n    int64
		read int
		err  error
	)
	b.pages.Range(func(idx int64, p meta.Page) bool {
		read, err = p.ReadAt(buffer[n:], 0)
		if err != nil {
			return false
		}
		if read == 0 {
			return false
		}
		n += int64(read)
		return true
	})

	if err != nil && err != io.EOF {
		return errors.Wrapf(err, "short read")
	}
	if n != b.meta.blockSize {
		for i := n; i < b.meta.blockSize; i++ {
			buffer[i] = 0
		}
	}

	b.interval.Merge()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	if b.interval.Total() == b.meta.blockSize {
		b.lg.Debug("flush to remote", zap.String("block", b.meta.key.String()), zap.String("written-size", humanize.IBytes(uint64(b.written))), zap.String("interval", b.interval.String()))
		b.swapFile.storage.Write(ctx, b.meta.key, buffer)
	} else {
		b.lg.Debug("partial flush to remote", zap.String("block", b.meta.key.String()), zap.String("written-size", humanize.IBytes(uint64(b.written))), zap.String("interval", b.interval.String()))
		err = b.swapFile.storage.WriteAt(ctx, b.meta.key, b.interval, buffer)
		if err != nil && !strings.Contains(err.Error(), "block already exists") {
			b.lg.Error("err flush partial block to remote", zap.String("block", b.meta.key.String()), zap.String("written-size", humanize.IBytes(uint64(b.written))), zap.Error(err))
			return err
		}
	}

	b.flushed = true
	b.written = 0

	return nil
}

type SwapFile struct {
	mu *sync.RWMutex

	ino     uint64
	storage adaptor.StorageBackend
	file    *os.File

	lastUsedNs int64 // last time the file was accessed
	closec     ClosableCh

	blocks fileBlocks
	lookup map[meta.BlockKey]meta.Block

	freePageList *freelist.FreeListG[int64] // free page start

	usage *FileUsage

	lg *zap.Logger

	executor *executor.LimitedConcurrentExecutor
	opts     meta.Options
}

func NewSwapFile(ino uint64, storage adaptor.StorageBackend, lg *zap.Logger, opts ...Option[*meta.Options]) *SwapFile {
	mu := &sync.RWMutex{}
	sf := &SwapFile{
		lg:           lg,
		mu:           mu,
		closec:       NewClosableCh(),
		storage:      storage,
		ino:          ino,
		blocks:       fileBlocks{},
		lookup:       make(map[adaptor.BlockKey]meta.Block),
		freePageList: freelist.NewFreeListUnboundG(func() int64 { return 0 }),
		opts:         meta.DefaultOptions,
		usage:        newFileUsage(),
	}

	ApplyOptions[*meta.Options](&sf.opts, opts...)
	if sf.opts.BlockSize < sf.opts.PageSize {
		sf.opts.PageSize = sf.opts.BlockSize
	}
	if sf.opts.PageCtx == nil {
		sf.opts.PageCtx = meta.NewPageContext(sf.opts.CacheSize, sf.opts.PageSize)
	}
	sf.executor = executor.NewLimitedConcurrentExecutor(sf.opts.Limit)
	heap.Init(&sf.blocks)
	return sf
}

func (sf *SwapFile) NewFileBlock(key meta.BlockKey) (meta.Block, error) {
	if sf.file == nil {
		var err error
		sf.file, err = os.CreateTemp(sf.opts.CacheDir, fmt.Sprintf("swap%d-", sf.ino))
		if err != nil {
			sf.lg.Error("create swap file err", zap.Error(err))
			return nil, nil
		}

		sf.lg.Info("NewSwapFile", zap.String("dir", sf.file.Name()),
			zap.String("blockSize", humanize.IBytes(uint64(sf.opts.BlockSize))),
			zap.String("pageSize", humanize.IBytes(uint64(sf.opts.PageSize))),
			zap.Int64("pageLimit", sf.opts.PageCtx.Limit()),
		)
	}
	sf.mu.Lock()
	defer sf.mu.Unlock()

	block := NewFileBlock(sf, newSwapBlockMeta(key, sf.opts.BlockSize, sf.opts.PageSize), sf.lg)
	heap.Push(&sf.blocks, block)
	sf.lookup[key] = block
	sf.usage.TrackBlock(key)
	return block, nil
}

func (sf *SwapFile) GetBlock(key meta.BlockKey) meta.Block {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	sf.lastUsedNs = time.Now().UnixNano()
	return sf.lookup[key]
}

func (sf *SwapFile) AcquireOffset() int64 {
	var offsetInFile int64
	sf.opts.PageCtx.Wait(func() {
		pageSize := sf.opts.PageSize

		offsetInFile = sf.freePageList.NewWith(func() int64 {
			off := sf.usage.Size
			sf.usage.Size += pageSize
			return off
		})

	})
	return offsetInFile
}

func (sf *SwapFile) OccupyPage() {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	sf.lastUsedNs = time.Now().UnixNano()
	sf.usage.IncrUsedPages()
	if sf.opts.PageCtx.IsFull() {
		sort.SliceStable(sf.blocks, func(i, j int) bool {
			return sf.blocks[i].totalWritten() > sf.blocks[j].totalWritten()
		})
		for i := 0; i < sf.blocks.Len()/10+1; i++ {
			block := sf.blocks[i]
			delete(sf.lookup, block.meta.key)
			sf.usage.UntrackBlock(block.meta.key)
			sf.executor.Execute(func() {
				block.Destroy()
				sf.usage.IncrEvict()
			})
		}
	}
}

func (sf *SwapFile) OccupiedUsage() *FileUsage {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.usage
}

func (sf *SwapFile) FreeResource() {
	sf.closec.Close(func() {
		if sf.file != nil {
			sf.lg.Info("free swapfile", zap.Uint64("ino", sf.ino), zap.String("file", sf.file.Name()))
			sf.mu.Lock()

			n := len(sf.blocks)
			for n > 0 {
				block := sf.blocks.Pop().(*FileBlock)
				delete(sf.lookup, block.meta.key)
				sf.usage.UntrackBlock(block.meta.key)
				sf.usage.EvictCount++
				block.Destroy()
				n--
			}
			sf.mu.Unlock()

			sf.file.Close()
			os.Remove(sf.file.Name())
		}
	})

}

func (sf *SwapFile) freePage(pageStart int64) {
	sf.opts.PageCtx.Signal(func() {
		sf.freePageList.Free(pageStart)
		sf.usage.DecUsedPages()
		// sf.lg.Sugar().Debugf("free ino[%d] page idx %d", sf.ino, pageStart/sf.opts.pageSize)
	})
}

type SwapBlockMeta struct {
	key       meta.BlockKey
	blockSize int64
	pageSize  int64
}

func newSwapBlockMeta(key meta.BlockKey, blockSize, pageSize int64) *SwapBlockMeta {
	return &SwapBlockMeta{
		key:       key,
		blockSize: blockSize,
		pageSize:  pageSize,
	}
}

type FileUsage struct {
	mu           sync.RWMutex
	Size         int64 // file offset in the swap file (aligned by pageSize)
	LastUsedNs   int64
	InUsedPages  int
	ActiveBlocks map[meta.BlockKey]struct{}
	EvictCount   int
}

func newFileUsage() *FileUsage {
	return &FileUsage{
		ActiveBlocks: make(map[adaptor.BlockKey]struct{}),
	}
}

func (fu *FileUsage) TrackBlock(key meta.BlockKey) {
	fu.mu.Lock()
	defer fu.mu.Unlock()
	fu.ActiveBlocks[key] = struct{}{}
}

func (fu *FileUsage) UntrackBlock(key meta.BlockKey) {
	fu.mu.Lock()
	defer fu.mu.Unlock()
	delete(fu.ActiveBlocks, key)
}

func (fu *FileUsage) IncrEvict() {
	fu.mu.Lock()
	defer fu.mu.Unlock()
	fu.EvictCount++
}

func (fu *FileUsage) IncrUsedPages() {
	fu.mu.Lock()
	defer fu.mu.Unlock()
	fu.InUsedPages++
}

func (fu *FileUsage) DecUsedPages() {
	fu.mu.Lock()
	defer fu.mu.Unlock()
	fu.InUsedPages++
}

func (fu *FileUsage) String() string {
	fu.mu.RLock()
	defer fu.mu.RUnlock()
	return fmt.Sprintf("File-Size: %d, In-Used-Pages: %d, Active-Blocks: %s, Evict-Blocks-Count: %d", fu.Size, fu.InUsedPages, fu.ActiveBlocks, fu.EvictCount)
}
