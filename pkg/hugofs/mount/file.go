package mount

import (
	"context"
	"io"
	"syscall"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/hugofs/vfs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pkg/errors"
	"github.com/samber/mo"
	"go.uber.org/zap"
)

// FileHandle is a resource identifier for opened files. Usually, a
// FileHandle should implement some of the FileXxxx interfaces.
//
// All of the FileXxxx operations can also be implemented at the
// InodeEmbedder level, for example, one can implement NodeReader
// instead of FileReader.
//
// FileHandles are useful in two cases: First, if the underlying
// storage systems needs a handle for reading/writing. This is the
// case with Unix system calls, which need a file descriptor (See also
// the function `NewLoopbackFile`). Second, it is useful for
// implementing files whose contents are not tied to an inode. For
// example, a file like `/proc/interrupts` has no fixed content, but
// changes on each open call. This means that each file handle must
// have its own view of the content; this view can be tied to a
// FileHandle. Files that have such dynamic content should return the
// FOPEN_DIRECT_IO flag from their `Open` method. See directio_test.go
// for an example.
type FileHandle struct {
	h    vfs.Handle
	fsys *FS

	lg *zap.Logger
}

// Create a new FileHandle
func newFileHandle(h vfs.Handle, fsys *FS) *FileHandle {
	return &FileHandle{
		h:    h,
		fsys: fsys,
		lg:   fsys.opt.Logger,
	}
}

var _ fusefs.FileHandle = (*FileHandle)(nil)

func (fh *FileHandle) Read(ctx context.Context, dest []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	n, err := fh.h.ReadAt(dest, off)
	if err != nil && err != io.EOF {
		fh.lg.Debug("read data error", zap.Error(err))
		return nil, syscall.EIO
	}
	return fuse.ReadResultData(dest[:n]), syscall.F_OK
}

var _ fusefs.FileReader = (*FileHandle)(nil)

func (fh *FileHandle) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	n, err := fh.h.WriteAt(data, off)
	if err != nil {
		fh.lg.Error("write data error", zap.Error(err))
		return 0, syscall.EIO
	}
	return uint32(n), syscall.F_OK
}

var _ fusefs.FileWriter = (*FileHandle)(nil)

func (fh *FileHandle) Release(ctx context.Context) syscall.Errno {
	return translateError(fh.h.Release())
}

var _ fusefs.FileReleaser = (*FileHandle)(nil)

func (fh *FileHandle) Flush(ctx context.Context) syscall.Errno {
	return translateError(fh.h.Flush())
}

var _ fusefs.FileFlusher = (*FileHandle)(nil)

func (fh *FileHandle) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	return translateError(fh.h.Flush())
}

var _ fusefs.FileFsyncer = (*FileHandle)(nil)

func (fh *FileHandle) Getlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) syscall.Errno {
	locks, err := fh.h.GetLks(ctx)
	if err != nil {
		return syscall.EAGAIN
	}
	lk2pb, errno := fuseLK2pb(owner, lk, flags)
	if errno != fuse.OK {
		return syscall.Errno(errno)
	}

	if isOverlap(lk2pb, locks) {
		// 2.1. if overlap, return fuse.EAGAIN
		return syscall.EAGAIN
	}

	// 3. fill the LkOut, return fuse.OK
	out.Typ = lk.Typ
	out.Start = lk.Start
	out.End = lk.End
	out.Pid = lk.Pid
	return 0
}

var _ fusefs.FileGetlker = (*FileHandle)(nil)

func (fh *FileHandle) Setlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	lk2pb, errno := fuseLK2pb(owner, lk, flags)
	if errno != fuse.OK {
		return syscall.Errno(errno)
	}

	locks, err := fh.h.GetLks(ctx)
	if err != nil {
		return syscall.EBUSY
	}

	for _, l := range locks {
		// check if the file is already opened by other users
		if l.Owner != owner {
			// block here, wait for the lock to be released
			return syscall.EBUSY
		}
		// check if the file is already opened by the same user
		if isOverlapLR(lk2pb, l) {
			// block here, wait for the lock to be released
			return syscall.EBUSY
		}
	}

	// 3. Set lock
	if err := fh.h.AcquireLock(ctx, lk2pb); err != nil {
		return syscall.EBUSY
	}
	return 0
}

var _ fusefs.FileSetlker = (*FileHandle)(nil)

func (fh *FileHandle) Setlkw(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	lk2pb, errno := fuseLK2pb(owner, lk, flags)
	if errno != fuse.OK {
		return syscall.Errno(errno)
	}
	locks, err := fh.h.GetLks(ctx)
	if err != nil {
		s, ok := status.FromError(err)
		if ok {
			if s.Code() != codes.NotFound {
				return syscall.EBUSY
			}
			// not found, do nothing
		}
		if errors.Is(err, context.Canceled) {
			return syscall.EINTR
		}
		return syscall.EBUSY
	}
	for _, l := range locks {
		// check if the file is already opened by other users
		if l.Owner != owner {
			// block here, wait for the lock to be released
			return syscall.EBUSY
		}
		// check if the file is already opened by the same user
		if isOverlapLR(lk2pb, l) {
			// block here, wait for the lock to be released
			return syscall.EBUSY
		}
	}

	// 3. Set lock
	if err := fh.h.AcquireLock(ctx, lk2pb); err != nil {
		return syscall.EBUSY
	}

	return 0
}

var _ fusefs.FileSetlkwer = (*FileHandle)(nil)

func fuseLK2pb(owner uint64, lk *fuse.FileLock, flags uint32) (*meta.FileLock, fuse.Status) {
	// 2. check locks if contain overlap
	kind := func() mo.Option[meta.LockKind] {
		switch lk.Typ {
		case syscall.F_RDLCK:
			return mo.Some(meta.LockKind_F_RDLCK)
		case syscall.F_WRLCK:
			return mo.Some(meta.LockKind_F_WRLCK)
		case syscall.F_UNLCK:
			return mo.Some(meta.LockKind_F_UNLCK)
		default:
			return mo.None[meta.LockKind]()
		}
	}()
	if kind.IsAbsent() {
		return nil, fuse.EINVAL
	}
	return &meta.FileLock{
		// Ino:        in.NodeId,
		Owner:      owner,
		Pid:        lk.Pid,
		Start:      lk.Start,
		End:        lk.End,
		Kind:       kind.MustGet(),
		Flag:       IfOr(flags == syscall.LOCK_SH, meta.LockFlag_LOCK_SH, meta.LockFlag_LOCK_EX),
		ExpireTime: uint64(time.Now().Add(5 * time.Minute).Unix()),
	}, fuse.OK
}

func isOverlap(req *meta.FileLock, locks []*meta.FileLock) bool {
	for _, l := range locks {
		if isOverlapLR(req, l) {
			return true
		}
	}
	return false
}
func isOverlapLR(l, r *meta.FileLock) bool {
	if l.Start >= r.Start && l.Start <= r.End {
		return true
	}
	if l.End >= r.Start && l.End <= r.End {
		return true
	}
	return false
}
