package vfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"go.uber.org/zap"
)

type File struct {
	inode   uint64
	mu      sync.RWMutex
	leaf    string
	writers []Handle // writers for this file
	readers []Handle // readers for this file

	// pendingModTime   time.Time                       // will be applied once o becomes available, i.e. after file was written
	pendingRenameFun func(ctx context.Context) error // will be run/renamed after all writers close

	d     *Dir
	dPath string

	sys     atomic.Value
	embeded *DirtyAttr
	Tree

	id string

	lg *zap.Logger
}

func newFileEntry(d *Dir, dPath string, fsEntry *FsEntry) *File {
	f := &File{
		d:       d,
		dPath:   dPath,
		leaf:    fsEntry.Name,
		inode:   fsEntry.Inode,
		embeded: fsEntry.ToDirtyAttr(d.vfs.fs),
		writers: make([]Handle, 0),
		readers: make([]Handle, 0),

		lg: d.lg.Named("file"),
		id: NextSnowIDStr(),
	}
	return f
}

func (f *File) Name() string {
	return f.leaf
}
func (f *File) Size() int64 {
	return f.embeded.Size()
}
func (f *File) Mode() os.FileMode {
	return f.embeded.Mode()
}
func (f *File) ModTime() time.Time {
	return f.embeded.ModTime()
}
func (f *File) Ctime() time.Time {
	return f.embeded.CTime()
}
func (f *File) Atime() time.Time {
	return f.embeded.ATime()
}
func (f *File) Nlink() int {
	return f.embeded.Nlink()
}
func (f *File) IsDir() bool {
	return f.Mode().IsDir()
}

func (f *File) Link() string {
	return f.embeded.LinkTarget()
}

func (f *File) Reload(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f._reloadAttr(ctx)
}

func (f *File) _reloadAttr(ctx context.Context) error {
	d := f.d
	entry, err := d.vfs.fs.Get(ctx, f.inode)
	if err != nil {
		s, ok := status.FromError(err)
		if ok && s.Code() == codes.NotFound {
			f.embeded.SetNlink(0)
			return nil
		} else {
			f.lg.Error("reload link attr error", zap.Error(err))
			return err
		}
	}

	f.lg.Debug("reload attr", zap.Any("inode", f.inode), zap.Any("id", f.id), zap.Any("nlink", entry.Nlink), zap.Any("size", entry.Size))
	f.embeded.unwrap(entry)
	return nil
}
func (f *File) Sys() any {
	return f.sys.Load()
}
func (f *File) IsFile() bool {
	return f.Mode().IsRegular()
}
func (f *File) Inode() uint64 {
	return f.inode
}
func (f *File) Uid() uint32 {
	return f.embeded.Uid()
}
func (f *File) Gid() uint32 {
	return f.embeded.Gid()
}

func (f *File) Rdev() uint32 {
	return f.embeded.Rdev()
}

func (f *File) ID() string {
	return f.id
}

func (f *File) SetModTime(modTime time.Time) error {
	return f.embeded.SetModTime(modTime)
}

func (f *File) SetMode(mode os.FileMode) error {
	return f.embeded.SetMode(mode)
}

func (f *File) SetATime(atime time.Time) error {
	return f.embeded.SetATime(atime)
}
func (f *File) SetCTime(ctime time.Time) error {
	return f.embeded.SetCTime(ctime)
}
func (f *File) SetUID(uid uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.embeded.SetUID(uid)
}
func (f *File) SetGID(gid uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.embeded.SetGID(gid)
}

func (f *File) SetSize(size int64) error {
	return f.embeded.SetSize(size)
}

func (f *File) SetNlink(nlink int) error {
	return f.embeded.SetNlink(nlink)
}

func (f *File) Sync() error {
	return nil
}

/*
man 2 unlink
unlink() deletes a name from the filesystem.  If that name was the last link to a file and no processes have the file open, the file is deleted and the space it was using is made available for reuse.
If the name was the last link to a file but any processes still have the file open, the file will remain in existence until the last file descriptor referring to it is closed.
If the name referred to a symbolic link, the link is removed.
If the name referred to a socket, FIFO, or device, the name for it is removed but processes which have the object open may continue to use it
*/
func (f *File) Remove(ctx context.Context) (err error) {
	f.mu.RLock()
	d := f.d
	f.mu.RUnlock()

	d.delete(f.leaf)

	f.mu.Lock()
	defer f.mu.Unlock()
	if err = f.d.vfs.fs.Remove(ctx, d.Inode(), f.leaf); err != nil {
		return
	}

	if f.VFS().Opt.Debug { // remove any possible files
		if err = os.Remove(path.Join(f.dPath, f.leaf)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	if f.embeded.Nlink() == 1 { // unlink/14.t
		f.embeded.SetNlink(0)
	}
	// f.decNlink()
	return f.d.Reload(ctx)
}

func (f *File) RemoveAll(ctx context.Context) error {
	return f.Remove(ctx)
}
func (f *File) Truncate(size int64) (err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if size == f.embeded.Size() { // size unchanged
		return
	}

	f.embeded.SetSize(size)
	for _, h := range f.writers {
		if err = h.Flush(); err != nil {
			return
		}
	}
	return nil
}
func (f *File) Flush(ctx context.Context) (err error) {
	return f.embeded.Flush(ctx)
}
func (f *File) Path() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f._path()
}

func (f *File) _path() string {
	return path.Join(f.dPath, f.leaf)
}

func (f *File) SetSys(v any) {
	f.sys.Store(v)
}

func (f *File) Children() []Node {
	return nil
}
func (f *File) GetParent() Node {
	return f.d
}

func (f *File) String() string {
	return f.leaf
}

func (f *File) Get(ino uint64) Node {
	if f.inode == ino {
		return f
	}
	return nil
}

func (f *File) Open(flags int) (h Handle, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lg.Sugar().Debugf("open [ %s ] with flags: %s", f._path(), openFlags(flags))
	h = newFileHandle(f, flags)

	if flags&os.O_WRONLY != 0 || flags&os.O_RDWR != 0 {
		f.writers = append(f.writers, h)
	} else {
		f.readers = append(f.readers, h)
	}
	return
}

func (f *File) VFS() *VFS {
	return f.d.vfs
}

func (f *File) rename(ctx context.Context, destDir *Dir, newName string) error {
	f.mu.RLock()
	pendingRenameFun := f.pendingRenameFun
	f.mu.RUnlock()

	oldPath := f.Path()
	oldName := f.leaf
	newPath := path.Join(destDir.Path(), newName)

	if oldPath == newPath {
		f.lg.Sugar().Debugf("no need to rename as the old and new paths are the same: %s -> %s", oldPath, newPath)
		return nil
	}

	renameCall := func(ctx context.Context) (err error) {
		if pendingRenameFun != nil {
			if err := pendingRenameFun(ctx); err != nil {
				return err
			}
		}

		f.mu.RLock()
		d := f.d
		f.mu.RUnlock()

		oldNode, _ := destDir.Stat(newName)

		entry, err := d.vfs.fs.Rename(ctx, d.inode, oldName, destDir.inode, newName)
		if err != nil {
			return err
		}
		f.embeded.unwrap(entry)

		// rename the file object
		dPath := destDir.Path()
		f.mu.RLock()

		// if the dst file existed, and the dst file have link to it
		if oldNode != nil {
			nodes := d.vfs.getNodes(oldNode.Inode())
			for _, n := range nodes {
				if err = n.Reload(ctx); err != nil {
					return err
				}
			}
		}

		f.d = destDir
		f.dPath = dPath
		f.leaf = newName
		f.pendingRenameFun = nil
		f.mu.RUnlock()

		f.lg.Sugar().Debugf("success rename %s -> %s", oldPath, newPath)
		return nil
	}

	f.mu.Lock()
	writing := f._writingInProgress()
	f.mu.Unlock()

	if writing {
		f.lg.Sugar().Debugf("File is currently open, delaying rename %p", f)
		f.mu.Lock()
		f.pendingRenameFun = renameCall
		f.mu.Unlock()
		return nil
	}

	return renameCall(ctx)
}

// _writingInProgress returns true of there are any open writers
func (f *File) _writingInProgress() bool {
	return len(f.writers) != 0
}

func (f *File) releaseHandle(h Handle) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.writers = slices.DeleteFunc(f.writers, func(_h Handle) bool {
		return _h == h
	})
}

var (
	openFlagNames = map[int64]string{
		int64(os.O_WRONLY):        "WRONLY",
		int64(os.O_RDWR):          "RDWR",
		int64(os.O_APPEND):        "APPEND",
		int64(syscall.O_ASYNC):    "ASYNC",
		int64(os.O_CREATE):        "CREAT",
		int64(os.O_EXCL):          "EXCL",
		int64(syscall.O_NOCTTY):   "NOCTTY",
		int64(syscall.O_NONBLOCK): "NONBLOCK",
		int64(os.O_SYNC):          "SYNC",
		int64(os.O_TRUNC):         "TRUNC",

		int64(syscall.O_CLOEXEC):   "CLOEXEC",
		int64(syscall.O_DIRECTORY): "DIRECTORY",
	}
)

func openFlags(flags int) string {
	return fmt.Sprintf("{%s}", flagString(openFlagNames, int64(flags), "O_RDONLY"))
}

func flagString(names map[int64]string, fl int64, def string) string {
	s := []string{}
	for k, v := range names {
		if fl&k != 0 {
			s = append(s, v)
			fl ^= k
		}
	}
	if len(s) == 0 && def != "" {
		s = []string{def}
	}
	if fl != 0 {
		s = append(s, fmt.Sprintf("0x%x", fl))
	}

	return strings.Join(s, ",")
}
