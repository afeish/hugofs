package buffer

import (
	"io"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"go.uber.org/zap"
)

var (
	_ DirtyPage = (*PageWritter)(nil)
)

type PageWritter struct {
	ino        uint64
	randWriter DirtyPage

	monitor PatternMonitor

	lg   *zap.Logger
	opts meta.Options
}

func NewPageWriter(ino uint64, storage adaptor.StorageBackend, lg *zap.Logger, opts ...Option[*meta.Options]) *PageWritter {
	pw := &PageWritter{
		ino:  ino,
		lg:   lg,
		opts: meta.DefaultOptions,
	}
	ApplyOptions(&pw.opts, opts...)
	pw.monitor = NewWriterPattern(pw.opts.BlockSize)
	pw.randWriter = NewMemBackedDirtyPage(ino, storage, pw.monitor, lg, opts...)
	return pw
}

func (pw *PageWritter) AddPage(p []byte, off int64) (n int, err error) {
	pw.monitor.Monitor(off, len(p))
	blockIdx := off / int64(pw.opts.BlockSize)
	for i := blockIdx; len(p) > 0; i++ {
		writeSize := Min(int64(len(p)), (i+1)*int64(pw.opts.BlockSize)-off)
		pw.randWriter.AddPage(p[:writeSize], off)
		p = p[writeSize:]
		off += writeSize
		n += int(writeSize)
	}
	return
}

func (pw *PageWritter) ReadAt(p []byte, off int64) (n int, err error) {
	blockIdx := off / int64(pw.opts.BlockSize)
	var read int = -1
	for i := blockIdx; len(p) > 0 && read != 0; i++ {
		read, err = pw.randWriter.ReadAt(p, off)
		// pw.lg.Debug("ReadAt", zap.Int64("off", off), zap.Int("read", read), zap.Int("n", n), zap.Int("cap", len(p)), zap.Error(err))
		if err != nil {
			pw.lg.Error("err read dirty page", zap.Error(err))
			return n, err
		}
		if read == 0 {
			break
		}
		off += int64(read)
		n += read
		p = p[read:]
	}
	if len(p) > 0 {
		err = io.EOF
	}
	return
}

func (pw *PageWritter) Flush() error {
	return pw.randWriter.Flush()
}
func (pw *PageWritter) Destroy() {
	pw.randWriter.Destroy()
}
func (pw *PageWritter) LockForRead(start, end int64) {
	pw.randWriter.LockForRead(start, end)
}
func (pw *PageWritter) UnlockForRead(start, end int64) {
	pw.randWriter.UnlockForRead(start, end)
}

func (pw *PageWritter) Idle(d time.Duration) bool {
	return pw.randWriter.Idle(d)
}
