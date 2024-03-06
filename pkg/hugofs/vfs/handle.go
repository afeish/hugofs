package vfs

import (
	"context"
	"os"

	"github.com/afeish/hugo/pb/meta"
)

// Handle is the interface satisfied by open files or directories.
// It is the methods on *os.File, plus a few more useful for FUSE
// filingsystems.  Not all of them are supported.
type Handle interface {
	Readdir(n int) ([]FileInfo, error)
	Close() error

	Read(b []byte) (n int, err error)
	ReadAt(b []byte, off int64) (n int, err error)
	Write(b []byte) (n int, err error)
	WriteAt(b []byte, off int64) (n int, err error)
	// Additional methods useful for FUSE filesystems
	Flush() error
	Release() error
	Node() Node
	GetLks(ctx context.Context) ([]*meta.FileLock, error)
	AcquireLock(ctx context.Context, lock *meta.FileLock) error
	Size() int64
}

// nullHandle implements all the missing methods
type nullHandle struct{}

func (h nullHandle) Chdir() error                                         { return ENOSYS }
func (h nullHandle) Chmod(mode os.FileMode) error                         { return ENOSYS }
func (h nullHandle) Chown(uid, gid int) error                             { return ENOSYS }
func (h nullHandle) Close() error                                         { return ENOSYS }
func (h nullHandle) Fd() uintptr                                          { return 0 }
func (h nullHandle) Name() string                                         { return "" }
func (h nullHandle) Read(b []byte) (n int, err error)                     { return 0, ENOSYS }
func (h nullHandle) ReadAt(b []byte, off int64) (n int, err error)        { return 0, ENOSYS }
func (h nullHandle) Readdir(n int) ([]FileInfo, error)                    { return nil, ENOSYS }
func (h nullHandle) Readdirnames(n int) (names []string, err error)       { return nil, ENOSYS }
func (h nullHandle) Seek(offset int64, whence int) (ret int64, err error) { return 0, ENOSYS }
func (h nullHandle) Stat() (FileInfo, error)                              { return nil, ENOSYS }
func (h nullHandle) Sync() error                                          { return nil }
func (h nullHandle) Truncate(size int64) error                            { return ENOSYS }
func (h nullHandle) Write(b []byte) (n int, err error)                    { return 0, ENOSYS }
func (h nullHandle) WriteAt(b []byte, off int64) (n int, err error)       { return 0, ENOSYS }
func (h nullHandle) WriteString(s string) (n int, err error)              { return 0, ENOSYS }
func (h nullHandle) Flush() (err error)                                   { return ENOSYS }
func (h nullHandle) Release() (err error)                                 { return ENOSYS }
func (h nullHandle) Node() Node                                           { return nil }
