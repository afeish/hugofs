package page

import (
	"sync"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"go.uber.org/zap"
)

var (
	_ meta.Page = (*MemPage)(nil)
)

type MemPage struct {
	mu           sync.RWMutex
	offsetInFile int64
	cap          int64
	size         int64

	block meta.Block
	ctx   *meta.PageContext

	lg *zap.Logger
}

func NewMemPage(ctx *meta.PageContext, block meta.Block, offsetInFile int64, cap int64, lg *zap.Logger) *MemPage {
	return &MemPage{
		ctx:          ctx,
		block:        block,
		offsetInFile: offsetInFile,
		cap:          cap,
		lg:           lg,
	}
}

func (mp *MemPage) WriteAt(p []byte, off int64) (n int, err error) {
	expectN := Min(mp.cap-off, int64(len(p)))
	n, err = mp.block.WriteAt(p[0:expectN], mp.offsetInFile+off)
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.size += (int64(n) + off)
	return
}

func (mp *MemPage) ReadAt(p []byte, off int64) (n int, err error) {
	expectN := Min(mp.GetSize()-off, int64(len(p)))
	return mp.block.ReadAt(p[0:expectN], mp.offsetInFile+off)
}

func (mp *MemPage) Range() meta.Range {
	return meta.Range{
		Start: mp.offsetInFile,
		End:   mp.offsetInFile + mp.cap,
	}
}

func (mp *MemPage) GetSize() int64 {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.size
}

func (mp *MemPage) SetSize(sz int64) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.size = sz
}

func (mp *MemPage) IsComplete() bool {
	return mp.GetSize() >= mp.cap
}

func (mp *MemPage) Free() {
	mp.block.WriteAt(mp.ctx.GetZeroPage(), mp.offsetInFile)
	mp.SetSize(0)
}
