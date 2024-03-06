package vfs

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/afeish/hugo/cmd/mountlib"
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/avast/retry-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"

	"github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/util"
)

type FsEntry struct {
	Inode       uint64
	Name        string
	Mode        os.FileMode
	Size        uint64
	IsDirectory bool `yaml:"is_directory"`
	Entries     []FsEntry

	Nlink      int
	LinkTarget string
	Dev        uint32

	Ctime time.Time
	Mtime time.Time
	Atime time.Time

	Uid uint32
	Gid uint32
}

func (e *FsEntry) IsDir() bool {
	return e.Mode.IsDir()
}

func (e *FsEntry) ToDirtyAttr(fs *Fs) *DirtyAttr {
	return &DirtyAttr{
		fs:         fs,
		ino:        e.Inode,
		size:       int64(e.Size),
		mode:       e.Mode,
		ctime:      e.Ctime,
		mtime:      e.Mtime,
		atime:      e.Mtime,
		nlink:      e.Nlink,
		linkTarget: e.LinkTarget,
		dev:        e.Dev,
		uid:        e.Uid,
		gid:        e.Gid,
	}
}

func (e *FsEntry) String() string {
	bs, _ := json.Marshal(e)
	return string(bs)
}

type DirEntries []FsEntry

type Fs struct {
	adaptor.MetaServiceClient
	adaptor.StorageBackend
	opts *mountlib.Options
	lg   *zap.Logger
}

func NewFs(meta adaptor.MetaServiceClient, storage adaptor.StorageBackend, opts *mountlib.Options) *Fs {
	if opts == nil {
		opts = &mountlib.DefaultOption
	}
	fs := &Fs{MetaServiceClient: meta, StorageBackend: storage, opts: opts, lg: opts.Logger.Named("fs")}
	return fs
}

func (fs *Fs) UpdateAttr(ctx context.Context, ino uint64, set int, entry *FsEntry) (newEntry *FsEntry, err error) {
	err = fs.retry(ctx, func() error {
		r, err := fs.UpdateInodeAttr(ctx, &meta.UpdateInodeAttr_Request{
			Ino: ino,
			Set: Ptr(uint32(set)),
			Attr: &meta.ModifyAttr{
				Size:    &entry.Size,
				Mode:    (*uint32)(&entry.Mode),
				Uid:     &entry.Uid,
				Gid:     &entry.Gid,
				Atime:   Ptr(entry.Atime.Unix()),
				Atimens: Ptr(uint32(entry.Atime.Nanosecond())),
				Mtime:   Ptr(entry.Mtime.Unix()),
				Mtimens: Ptr(uint32(entry.Mtime.Nanosecond())),
				Ctime:   Ptr(entry.Ctime.Unix()),
				Ctimens: Ptr(uint32(entry.Ctime.Nanosecond())),
			},
		})
		if err != nil {
			return err
		}
		newEntry = wrap(r.Attr)
		return nil
	})
	return
}

func (fs *Fs) List(ctx context.Context, ino uint64) (entries DirEntries, err error) {
	err = fs.retry(ctx, func() error {
		r, err := fs.ListItemUnderInode(ctx, &meta.ListItemUnderInode_Request{Ino: ino, IsPlusMode: true})
		if err != nil {
			entries = DirEntries{}
			return err
		}
		for _, item := range r.Items {
			entry := wrap(item.Attr)
			entry.Name = item.Name
			entries = append(entries, *entry)
		}
		return nil
	})
	return
}

func (fs *Fs) Get(ctx context.Context, ino uint64) (entry *FsEntry, err error) {
	err = fs.retry(ctx, func() error {
		r, err := fs.GetInodeAttr(ctx, &meta.GetInodeAttr_Request{Ino: ino})
		if err != nil {
			return err
		}
		entry = wrap(r.Attr)
		return nil
	})
	return
}

func (fs *Fs) Mkdir(ctx context.Context, parent uint64, name string, mode uint32) (entry *FsEntry, err error) {
	// mode = util.ToSyscallType(os.ModeDir) | mode&^uint32(fs.opts.Umask)
	osMode := os.ModeDir | os.FileMode(mode)
	return fs._createInode(ctx, parent, name, uint32(osMode), "", 0)
}

func (fs *Fs) Create(ctx context.Context, parent uint64, name string, mode, rdev uint32) (entry *FsEntry, err error) {
	return fs._createInode(ctx, parent, name, uint32(util.ToFileMode(mode)), "", rdev)
}

func (fs *Fs) Symlink(ctx context.Context, parent uint64, target, linkName string) (entry *FsEntry, err error) {
	// mode := util.ToSyscallType(os.ModeSymlink) | uint32(os.ModePerm)&^uint32(fs.opts.Umask)
	osMode := os.ModeSymlink | os.ModePerm&^fs.opts.Umask
	return fs._createInode(ctx, parent, linkName, uint32(osMode), target, 0)
}

func (fs *Fs) Link(ctx context.Context, targetIno, parent uint64, linkName string) (entry *FsEntry, err error) {
	r, err := fs.MetaServiceClient.Link(ctx, &meta.Link_Request{Ino: targetIno, NewParentIno: parent, NewName: linkName})
	if err != nil {
		fs.lg.Error("Link: update remote old path node attr failed", zap.Error(err))
		return nil, err
	}
	entry = wrap(r.Attr)
	entry.Name = linkName
	return
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (fs *Fs) Rmdir(ctx context.Context, parent uint64, name string) (err error) {
	if _, err = fs.DeleteDirInode(ctx, &meta.DeleteDirInode_Request{
		ParentIno:   parent,
		Name:        name,
		Recursively: false,
	}); err != nil {
		return
	}
	return
}

func (fs *Fs) Remove(ctx context.Context, parent uint64, name string) (err error) {
	if _, err = fs.DeleteInode(ctx, &meta.DeleteInode_Request{ParentIno: parent, Name: name}); err != nil {
		return err
	}
	return
}

func (fs *Fs) Rename(ctx context.Context, oldParent uint64, oldName string, newParent uint64, newName string) (entry *FsEntry, err error) {
	r, err := fs.MetaServiceClient.Rename(ctx, &meta.Rename_Request{ParentSrcIno: oldParent, OldName: oldName, ParentDstIno: newParent, NewName: newName})
	if err != nil {
		return nil, err
	}

	return wrap(r.NewEntry.Attr), nil
}

func (fs *Fs) Statistics(ctx context.Context) (r *meta.Statistics_Response, err error) {
	err = fs.retry(ctx, func() error {
		r, err = fs.MetaServiceClient.Statistics(ctx, &meta.Statistics_Request{})
		if err != nil {
			return err
		}

		return nil
	})
	return
}

func (fs *Fs) GetLks(ctx context.Context, ino Ino) (locks []*meta.FileLock, err error) {
	err = fs.retry(ctx, func() error {
		r, err := fs.MetaServiceClient.GetFileLocks(ctx, &meta.GetFileLocks_Request{Ino: ino})
		if err != nil {
			return err
		}
		locks = r.Locks
		return nil
	})
	return
}

func (fs *Fs) SetLk(ctx context.Context, lock *meta.FileLock) (locks *meta.FileLock, err error) {
	err = fs.retry(ctx, func() error {
		r, err := fs.MetaServiceClient.AcquireFileLock(ctx, &meta.AcquireFileLock_Request{Lock: lock})
		if err != nil {
			return err
		}
		locks = r.Lock
		return nil
	})
	return
}

func (fs *Fs) retry(ctx context.Context, f func() error) error {
	return retry.Do(f,
		retry.Delay(retry.DefaultDelay),
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.Context(ctx))
}

func (fs *Fs) _createInode(ctx context.Context, parent uint64, name string, mode uint32, target string, rdev uint32) (entry *FsEntry, err error) {
	uid, gid := fs.extractCaller(ctx)
	now := time.Now()
	r, err := fs.CreateInode(ctx, &meta.CreateInode_Request{
		ParentIno: parent,
		ItemName:  name,
		Attr: &meta.ModifyAttr{
			Atime:      Ptr(now.Unix()),
			Ctime:      Ptr(now.Unix()),
			Mtime:      Ptr(now.Unix()),
			Atimens:    Ptr(uint32(now.Nanosecond())),
			Ctimens:    Ptr(uint32(now.Nanosecond())),
			Mtimens:    Ptr(uint32(now.Nanosecond())),
			Uid:        Ptr(uid),
			Gid:        Ptr(gid),
			Mode:       Ptr(mode),
			LinkTarget: Ptr(target),
			Rdev:       Ptr(rdev),
		},
	})
	if err != nil {
		return nil, err
	}
	entry = wrap(r.Attr)
	entry.Name = name
	return
}

func (fs *Fs) extractCaller(ctx context.Context) (uid, gid uint32) {
	defer func() {
		if uid == 0 {
			uid = fs.opts.Mount.UID
		}
		if gid == 0 {
			gid = fs.opts.Mount.GID
		}
	}()
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		for k, v := range md {
			switch k {
			case "uid":
				_uid, _ := strconv.Atoi(v[0])
				uid = uint32(_uid)

			case "gid":
				_gid, _ := strconv.Atoi(v[0])
				gid = uint32(_gid)
			default:
			}
		}
	}
	return
}

func wrap(attr *meta.Attr) *FsEntry {
	mode := os.FileMode(attr.Mode)
	size := attr.Size
	if mode&os.ModeSymlink == os.ModeSymlink {
		size = uint64(len(attr.LinkTarget))
	}
	return &FsEntry{
		Inode:       attr.Inode,
		Mode:        mode,
		Size:        size,
		IsDirectory: mode.IsDir(),

		Nlink:      int(attr.Nlink),
		LinkTarget: attr.LinkTarget,
		Dev:        attr.Rdev,
		Ctime:      time.Unix(attr.Ctime, int64(attr.Ctimens)),
		Atime:      time.Unix(attr.Atime, int64(attr.Atimens)),
		Mtime:      time.Unix(attr.Mtime, int64(attr.Mtimens)),
		Uid:        attr.Uid,
		Gid:        attr.Gid,
	}
}
