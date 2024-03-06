package page

import (
	"os"
	"sync"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"go.uber.org/zap"
)

var (
	_ meta.Page = (*FilePage)(nil)
)

type FilePage struct {
	mu           sync.RWMutex
	offsetInFile int64
	cap          int64
	size         int64

	file *os.File
	ctx  *meta.PageContext

	lg *zap.Logger
}

func NewFilePage(ctx *meta.PageContext, file *os.File, offsetInFile int64, cap int64, lg *zap.Logger) *FilePage {
	return &FilePage{
		ctx:          ctx,
		offsetInFile: offsetInFile,
		cap:          cap,
		file:         file,
		lg:           lg,
	}
}

func (fp *FilePage) WriteAt(p []byte, off int64) (n int, err error) {
	expectN := Min(fp.cap-off, int64(len(p)))
	n, err = fp.file.WriteAt(p[0:expectN], fp.offsetInFile+off)
	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.size += (int64(n) + off)
	return
}

func (fp *FilePage) ReadAt(p []byte, off int64) (n int, err error) {
	expectN := Min(fp.GetSize()-off, int64(len(p)))
	return fp.file.ReadAt(p[0:expectN], fp.offsetInFile+off)
}

func (fp *FilePage) Range() meta.Range {
	return meta.Range{
		Start: fp.offsetInFile,
		End:   fp.offsetInFile + fp.cap,
	}
}

func (fp *FilePage) GetSize() int64 {
	fp.mu.RLock()
	defer fp.mu.RUnlock()
	return fp.size
}

func (fp *FilePage) SetSize(sz int64) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.size = sz
}

func (fp *FilePage) IsComplete() bool {
	return fp.GetSize() >= fp.cap
}

func (fp *FilePage) Free() {
	fp.file.WriteAt(fp.ctx.GetZeroPage(), fp.offsetInFile)
	fp.SetSize(0)
}
