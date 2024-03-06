package vfs

import (
	"context"
	"io"
	"os"

	metapb "github.com/afeish/hugo/pb/meta"
	"go.uber.org/zap"
)

type DirHandle struct {
	nullHandle
	d   *Dir
	fis []FileInfo // where Readdir got to

	lg *zap.Logger
}

// newDirHandle opens a directory for read
func newDirHandle(d *Dir) *DirHandle {
	return &DirHandle{
		d:  d,
		lg: d.vfs.Opt.Logger.Named("<dir-handle>"),
	}
}

// String converts it to printable
func (h *DirHandle) String() string {
	if h == nil {
		return "<nil *DirHandle>"
	}
	if h.d == nil {
		return "<nil *DirHandle.d>"
	}
	return h.d.String() + " (r)"
}

// Stat returns info about the current directory
func (h *DirHandle) Stat() (fi os.FileInfo, err error) {
	return h.d, nil
}

// Node returns the Node associated with this - satisfies Noder interface
func (h *DirHandle) Node() Node {
	return h.d
}

// Readdir reads the contents of the directory associated with file and returns
// a slice of up to n FileInfo values, as would be returned by Lstat, in
// directory order. Subsequent calls on the same file will yield further
// FileInfos.
//
// If n > 0, Readdir returns at most n FileInfo structures. In this case, if
// Readdir returns an empty slice, it will return a non-nil error explaining
// why. At the end of a directory, the error is io.EOF.
//
// If n <= 0, Readdir returns all the FileInfo from the directory in a single
// slice. In this case, if Readdir succeeds (reads all the way to the end of
// the directory), it returns the slice and a nil error. If it encounters an
// error before the end of the directory, Readdir returns the FileInfo read
// until that point and a non-nil error.
func (h *DirHandle) Readdir(n int) (fis []FileInfo, err error) {
	if h.fis == nil {
		nodes, err := h.d.ReadDirAll()

		if err != nil {
			return nil, err
		}
		h.fis = []FileInfo{}
		for _, node := range nodes {
			h.fis = append(h.fis, node)
		}
	}
	nn := len(h.fis)
	if n > 0 {
		if nn == 0 {
			return nil, io.EOF
		}
		if nn > n {
			nn = n
		}
	}
	fis, h.fis = h.fis[:nn], h.fis[nn:]
	return fis, nil
}

// Readdirnames reads and returns a slice of names from the directory f.
//
// If n > 0, Readdirnames returns at most n names. In this case, if
// Readdirnames returns an empty slice, it will return a non-nil error
// explaining why. At the end of a directory, the error is io.EOF.
//
// If n <= 0, Readdirnames returns all the names from the directory in a single
// slice. In this case, if Readdirnames succeeds (reads all the way to the end
// of the directory), it returns the slice and a nil error. If it encounters an
// error before the end of the directory, Readdirnames returns the names read
// until that point and a non-nil error.
func (h *DirHandle) Readdirnames(n int) (names []string, err error) {
	nodes, err := h.Readdir(n)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		names = append(names, node.Name())
	}
	return names, nil
}

// Close closes the handle
func (h *DirHandle) Close() (err error) {
	h.fis = nil
	return nil
}

func (h *DirHandle) Size() int64 {
	return h.d.Size()
}

func (h *DirHandle) GetLks(ctx context.Context) ([]*metapb.FileLock, error) {
	return h.Node().VFS().fs.GetLks(ctx, h.Node().Inode())
}

func (h *DirHandle) AcquireLock(ctx context.Context, lock *metapb.FileLock) error {
	lock.Ino = h.Node().Inode()
	_, err := h.Node().VFS().fs.SetLk(ctx, lock)
	return err
}
