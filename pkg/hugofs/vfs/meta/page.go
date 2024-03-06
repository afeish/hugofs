package meta

import (
	"sync"

	"github.com/afeish/hugo/pkg/util/mem"
)

var (
	writePageCtx     *PageContext
	writePageCtxOnce sync.Once

	readPageCtx     *PageContext
	readPageCtxOnce sync.Once
)

func SetWritePageCtx(ctx *PageContext) {
	writePageCtxOnce.Do(func() {
		writePageCtx = ctx
	})
}
func GetWritePageCtx() *PageContext {
	return writePageCtx
}

func SetReadPageCtx(ctx *PageContext) {
	readPageCtxOnce.Do(func() {
		readPageCtx = ctx
	})
}
func GetReadPageCtx() *PageContext {
	return readPageCtx
}

type PageContext struct {
	mu   *sync.RWMutex
	cond *sync.Cond

	cacheSize int64
	pageSize  int64
	counter   int64
	limit     int64

	zeroBuf []byte
}

func NewPageContext(cacheSize, pageSize int64) *PageContext {
	var mu sync.RWMutex

	return &PageContext{
		mu:        &mu,
		cond:      sync.NewCond(&mu),
		counter:   0,
		cacheSize: cacheSize,
		pageSize:  pageSize,
		limit:     cacheSize / pageSize,
		zeroBuf:   mem.Allocate(int(pageSize)),
	}
}

func (pc *PageContext) Incr() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.counter++
}

func (pc *PageContext) Dec() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.counter--
}

func (pc *PageContext) GetPages() int64 {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.counter
}

func (pc *PageContext) GetZeroPage() []byte {
	return pc.zeroBuf
}

func (pc *PageContext) IsFull() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.counter == pc.limit
}

func (pc *PageContext) Wait(f func()) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	for pc.counter >= pc.limit {
		pc.cond.Wait()
	}

	f()
	pc.counter++
}

func (pc *PageContext) Signal(f func()) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	f()
	pc.counter--
	pc.cond.Signal()
}

func (pc *PageContext) Limit() int64 {
	return pc.limit
}

type Range struct {
	Start int64
	End   int64
}

func (r Range) Size() int64 {
	return r.End - r.Start
}

type Page interface {
	// WriteAt writes data at the given page offset until the whole page is written or the full buffer is written.
	WriteAt(p []byte, off int64) (n int, err error)
	ReadAt(p []byte, off int64) (n int, err error)
	Range() Range
	Free()
	GetSize() int64
}
