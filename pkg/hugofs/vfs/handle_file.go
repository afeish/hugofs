package vfs

import (
	"context"
	"os"
	"path"
	"sync/atomic"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	metapb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/hugofs/vfs/buffer"
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/util/size"

	"go.uber.org/zap"
)

type FileHandle struct {
	nullHandle
	f *File

	openFlags int
	hasWrite  atomic.Bool

	// debug mode will use local file as the backed source
	debug  bool
	backed *os.File

	size   atomic.Int64
	buffer meta.RWBuffer
}

func newFileHandle(f *File, flags int) Handle {
	vfs := f.VFS()
	opt := vfs.Opt
	fh := &FileHandle{f: f, openFlags: flags, debug: opt.IO.Debug}

	if fh.debug {
		fpath := path.Join(opt.IO.Prefix, f._path())
		dir := path.Dir(fpath)

		err := os.MkdirAll(dir, os.ModePerm&^opt.Umask)
		if err != nil {
			f.lg.Sugar().Fatalf("err mkdir: %s", dir)
		}

		if flags&os.O_EXCL != 0 {
			os.Remove(fpath)
		}

		if flags&os.O_TRUNC != 0 {
			os.Truncate(fpath, 0)
		}

		if _, err = os.Stat(fpath); err != nil {
			fh.backed, err = os.Create(fpath)
			if err != nil {
				f.lg.Sugar().Errorf("err create file [ %s ]: %s", fpath, err)
			}
		} else {
			fh.backed, err = os.OpenFile(fpath, os.O_RDWR, 0666)
			if err != nil {
				f.lg.Sugar().Errorf("err open existed file [ %s ] with flags %d: %s", fpath, flags, err)
			}
		}
		f.lg.Sugar().Debugf("using local file [ %s ] as the backed source to file [ %s ]", fpath, fpath)

	} else {
		fh.buffer = buffer.NewDirectRWBuffer(f.Inode(), vfs.Fs(), vfs.memThrot, vfs.Fs().lg, meta.WithPageCtx(meta.GetWritePageCtx()))
	}

	fh.size.Store(f.Size())
	return fh
}

func (h *FileHandle) Close() error {
	if h.debug {
		return h.backed.Close()
	}

	h.buffer.Destroy()
	return nil
}

func (h *FileHandle) Write(b []byte) (n int, err error) {
	if h.debug {
		return h.backed.Write(b)
	}
	return h.nullHandle.Write(b)
}
func (h *FileHandle) WriteAt(b []byte, off int64) (n int, err error) {
	if h.debug {
		n, err = h.backed.WriteAt(b, off)
	} else {
		n, err = h.buffer.Write(b, off)
	}
	if err != nil {
		return
	}
	if n > 0 {
		h.hasWrite.Store(true)
	}

	size := int64(n + int(off))
	if size > h.size.Load() {
		h.f.SetSize(size)
		h.size.Store(size)
	}
	return
}

func (h *FileHandle) Name() string { return h.f.Name() }

func (h *FileHandle) Read(b []byte) (n int, err error) {
	if h.debug {
		return h.backed.Read(b)
	}
	return h.nullHandle.Read(b)
}
func (h *FileHandle) ReadAt(b []byte, off int64) (n int, err error) {
	if h.debug {
		return h.backed.ReadAt(b, off)
	}

	n, err = h.buffer.Read(b, off)
	h.f.lg.Sugar().Debugf("read %d bytes, summary: %s", n, size.ReadSummary(b, 10))
	return
}

func (h *FileHandle) Flush() (err error) {
	if !h.hasWrite.Load() {
		return
	}

	defer func() {
		h.hasWrite.Store(false)
	}()

	if h.debug {
		if err = h.backed.Sync(); err != nil {
			return
		}
	} else {
		if err = h.buffer.Flush(); err != nil {
			return
		}
	}
	h.f.lg.Debug("flush fileHandle ...", zap.Any("size", h.size.Load()))
	set := SetAttrAtime | SetAttrCtime | SetAttrMtime | SetAttrSize
	return h.f.Flush(NewAttrContext(context.Background(), set))
}
func (h *FileHandle) Release() (err error) {
	h.f.releaseHandle(h)

	if h.debug {
		return h.backed.Close()
	}
	h.buffer.Destroy()
	return nil
}

func (h *FileHandle) Node() Node {
	return h.f
}

func (h *FileHandle) Size() int64 {
	return h.f.Size()
}

func (h *FileHandle) GetLks(ctx context.Context) ([]*metapb.FileLock, error) {
	return h.Node().VFS().fs.GetLks(ctx, h.Node().Inode())
}

func (h *FileHandle) AcquireLock(ctx context.Context, lock *metapb.FileLock) error {
	lock.Ino = h.Node().Inode()
	_, err := h.Node().VFS().fs.SetLk(ctx, lock)
	return err
}
