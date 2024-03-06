package mount

import (
	"context"
	"syscall"
	"time"

	"os"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/hugofs/vfs"
	"github.com/afeish/hugo/pkg/util"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pingcap/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// FS represents the top level filing system
type FS struct {
	VFS *vfs.VFS
	f   *vfs.Fs
	opt *mountlib.Options

	statisticsCache statisticsCache
	isOverQuota     bool
	lg              *zap.Logger
}

// NewFS creates a pathfs.FileSystem from the fs.Fs passed in
func NewFS(ctx context.Context, VFS *vfs.VFS, opt *mountlib.Options) *FS {
	fsys := &FS{
		VFS: VFS,
		f:   VFS.Fs(),
		opt: opt,
		lg:  opt.Logger.Named("mount-fs"),
	}
	go fsys.Core(ctx)
	return fsys
}

// Root returns the root node
func (f *FS) Root() (node *hugoInode, err error) {
	root, err := f.VFS.Root()
	if err != nil {
		return nil, err
	}
	return newhugoInode(f, root), nil
}

// get the Mode from a vfs Node
func getMode(node os.FileInfo) uint32 {
	return util.ToSyscallMode(node.Mode())
}

// fill in attr from node
func setAttr(node vfs.Node, attr *fuse.Attr) {
	Size := uint64(node.Size())
	var BlockSize = uint64(size.KibiByte.Int() * 512)
	Blocks := (Size + BlockSize - 1) / BlockSize
	modTime := node.ModTime()
	aTime := node.Atime()
	cTime := node.Ctime()
	// set attributes
	attr.Ino = node.Inode()
	attr.Owner.Uid = node.Uid()
	attr.Owner.Gid = node.Gid()
	attr.Mode = getMode(node)
	attr.Size = Size
	attr.Nlink = uint32(node.Nlink())
	attr.Blocks = Blocks
	attr.Blksize = uint32(BlockSize) // not supported in freebsd/darwin, defaults to 4k if not set
	attr.Atime = uint64(aTime.Unix())
	attr.Atimensec = uint32(aTime.Nanosecond())
	attr.Mtime = uint64(modTime.Unix())
	attr.Mtimensec = uint32(modTime.Nanosecond())
	attr.Ctime = uint64(cTime.Unix())
	attr.Ctimensec = uint32(cTime.Nanosecond())
	attr.Rdev = node.Rdev()
}

// fill in AttrOut from node
func (f *FS) setAttrOut(node vfs.Node, out *fuse.AttrOut) {
	setAttr(node, &out.Attr)
	out.SetTimeout(f.opt.AttrTimeout)
}

// fill in EntryOut from node
func (f *FS) setEntryOut(node vfs.Node, out *fuse.EntryOut) {
	setAttr(node, &out.Attr)
	out.SetEntryTimeout(f.opt.AttrTimeout)
	out.SetAttrTimeout(f.opt.AttrTimeout)
}

func (f *FS) Core(ctx context.Context) error {
	for _, fn := range []func(ctx context.Context) error{
		func(ctx context.Context) error {
			//fs.checkQuota(ctx) FIXME: this method will panic
			return nil
		},
	} {
		go func(fn func(ctx context.Context) error) {
			defer func() {
				// prevent panic
				if err := recover(); err != nil {
					log.Error("Core panic: ", zap.Any("recover", err))
				}
				// todo: do recover
			}()
			if err := fn(ctx); err != nil {
				return
			}
		}(fn)
	}

	return nil
}

func (f *FS) Statistics() (*meta.Statistics_Response, error) {
	statCache := f.statisticsCache
	if statCache.lastCachedTime < time.Now().Unix()-20 {
		resp, err := f.f.Statistics(context.Background())
		if err != nil {
			log.Error("read filesystem status failed", zap.Error(err))
			return nil, err
		}
		statCache.v = resp
		statCache.lastCachedTime = time.Now().Unix()
	}
	return statCache.v, nil
}

func (f *FS) checkQuota(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Minute):
			resp, err := f.Statistics()
			if err != nil {
				log.Error("load quota failed", zap.Error(err))
			}

			log.Info("quota", zap.Uint64("used", resp.Usage), zap.Uint64("total", resp.TotalFileCnt))
			f.isOverQuota = resp.Usage > f.opt.Quota
		}
	}
}

// Translate errors from mountlib into Syscall error numbers
func translateError(err error) syscall.Errno {
	if err == nil {
		return 0
	}
	switch errors.Cause(err) {
	case vfs.OK:
		return 0
	case vfs.ENOENT:
		return syscall.ENOENT
	case vfs.EEXIST:
		return syscall.EEXIST
	case vfs.EPERM:
		return syscall.EPERM
	case vfs.ECLOSED:
		return syscall.EBADF
	case vfs.ENOTEMPTY:
		return syscall.ENOTEMPTY
	case vfs.ENOTDIR:
		return syscall.ENOTDIR
	case vfs.ESPIPE:
		return syscall.ESPIPE
	case vfs.EBADF:
		return syscall.EBADF
	case vfs.EROFS:
		return syscall.EROFS
	case vfs.ENOSYS:
		return syscall.ENOSYS
	case vfs.EINVAL:
		return syscall.EINVAL
	}
	return syscall.EIO
}

type statisticsCache struct {
	v              *meta.Statistics_Response
	lastCachedTime int64
}
