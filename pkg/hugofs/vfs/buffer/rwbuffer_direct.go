package buffer

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"go.uber.org/zap"
)

var _ meta.RWBuffer = (*DirectRWBuffer)(nil)

type DirectRWBuffer struct {
	mu         sync.RWMutex
	dirtyPages *PageWritter
	readBuffer BlockReadBuffer

	lastAccessNs int64

	lg   *zap.Logger
	opts meta.Options
}

func NewDirectRWBuffer(ino Ino, storage adaptor.StorageBackend, memThrot *Throttler, lg *zap.Logger, opts ...Option[*meta.Options]) meta.RWBuffer {
	rw := &DirectRWBuffer{
		lg:   lg,
		opts: meta.DefaultOptions,
	}
	ApplyOptions(&rw.opts, opts...)

	rw.readBuffer = NewDefaultBlockBuffer(memThrot, ino, rw.opts.BlockSize, storage, lg, opts...)
	rw.dirtyPages = NewPageWriter(ino, storage, lg, opts...)
	return rw
}

func (b *DirectRWBuffer) Read(dst []byte, off int64) (n int, theErr error) {
	size := int64(len(dst))
	b.dirtyPages.LockForRead(off, off+size)
	defer b.dirtyPages.UnlockForRead(off, off+size)
	b.access()

	var (
		err                   error
		read                  int
		noOnRemote, noOnDirty bool
	)
	for n < len(dst) && !(noOnRemote && noOnDirty) {
		read, err = b.readBuffer.Read(dst[n:], int(off))
		n += read
		off += int64(read)
		if err != nil {
			if err == io.EOF {
				theErr = err
			} else {
				return
			}
		}
		if read == 0 {
			noOnRemote = true
		}

		if n >= len(dst) || (noOnRemote && noOnDirty) {
			break
		}

		read, err = b.dirtyPages.ReadAt(dst[n:], off)
		n += read
		off += int64(read)
		if err != nil {
			return
		}
		if read != 0 {
			if theErr == io.EOF {
				theErr = nil
			}
		} else {
			noOnDirty = true
		}
	}
	return
}

func (b *DirectRWBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	return b.Read(p, off)
}

func (b *DirectRWBuffer) Write(p []byte, off int64) (n int, err error) {
	b.access()
	n, err = b.dirtyPages.AddPage(p, off)
	return
}

func (b *DirectRWBuffer) WriteAt(p []byte, off int64) (n int, err error) {
	return b.Write(p, off)
}
func (b *DirectRWBuffer) Size() (int, error) {
	return 0, nil
}
func (b *DirectRWBuffer) Flush() error {
	b.dirtyPages.Flush()
	return nil
}
func (b *DirectRWBuffer) Idle(d time.Duration) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return time.Since(time.Unix(0, b.lastAccessNs)) > d
}
func (b *DirectRWBuffer) Destroy() {
	b.dirtyPages.Destroy()
	b.readBuffer.Destroy()
}

func (b *DirectRWBuffer) access() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastAccessNs = time.Now().UnixNano()
}
func (b *DirectRWBuffer) GetBlockMap() (map[BlockIndexInFile]meta.Block, error) {
	return nil, nil
}

func (b *DirectRWBuffer) ReadFrom(r io.ReaderAt) (n int, err error) {
	var written int
	for {
		reader := io.NewSectionReader(r, int64(written), int64(b.opts.BlockSize))
		writer := io.NewOffsetWriter(b, int64(written))
		n, err := io.Copy(writer, reader)
		if err != nil {
			if err != io.EOF {
				break
			}
			return int(written), err
		}
		if n == 0 {
			break
		}
		written += int(n)
	}
	return written, nil
}

func (b *DirectRWBuffer) WriteTo(w io.Writer) (n int64, err error) {
	// Create a reader
	r := strings.NewReader("Hello Medium")

	// Create a writer
	var bv bytes.Buffer

	// Push data
	r.WriteTo(&bv) // Don't forget &

	bv.WriteTo(w)

	// Optional: verify data
	fmt.Println(bv.String())
	return
}
