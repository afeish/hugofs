package vfs

import (
	"context"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

type DirtyAttr struct {
	fs  *Fs
	ino uint64

	sync.RWMutex
	mtime time.Time
	atime time.Time
	ctime time.Time

	mode       os.FileMode
	size       int64
	nlink      int
	linkTarget string
	dev        uint32

	uid uint32
	gid uint32

	dirty bool
}

func (a *DirtyAttr) Flush(ctx context.Context) (err error) {
	if !a.IsDirty() {
		return nil
	}

	set, ok := FromAttrContext(ctx)
	if !ok {
		return
	}

	entry, err := a.fs.UpdateAttr(ctx, a.ino, set, a.wrap())
	if err != nil {
		a.fs.lg.Debug("update attr error", zap.Error(err))
		return err
	}
	a.Lock()
	defer a.Unlock()
	a.dirty = false
	a.size = int64(entry.Size)
	a.atime = entry.Atime
	a.ctime = entry.Ctime
	a.mtime = entry.Mtime
	a.nlink = entry.Nlink
	a.uid = entry.Uid
	a.gid = entry.Gid
	a.mode = entry.Mode
	a.linkTarget = entry.LinkTarget
	a.dev = entry.Dev
	return
}

func (a *DirtyAttr) IsDirty() bool {
	a.RLock()
	defer a.RUnlock()
	return a.dirty
}
func (a *DirtyAttr) SetModTime(modTime time.Time) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.mtime = modTime
	return nil
}
func (a *DirtyAttr) ModTime() time.Time {
	a.RLock()
	defer a.RUnlock()
	return a.mtime
}

func (a *DirtyAttr) SetATime(atime time.Time) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.atime = atime
	return nil
}
func (a *DirtyAttr) ATime() time.Time {
	a.RLock()
	defer a.RUnlock()
	return a.atime
}
func (a *DirtyAttr) SetCTime(ctime time.Time) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.ctime = ctime
	return nil
}
func (a *DirtyAttr) CTime() time.Time {
	a.RLock()
	defer a.RUnlock()
	return a.ctime
}
func (a *DirtyAttr) SetNlink(nlink int) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.nlink = nlink
	return nil
}
func (a *DirtyAttr) Nlink() int {
	a.RLock()
	defer a.RUnlock()
	return a.nlink
}
func (a *DirtyAttr) Mode() os.FileMode {
	a.RLock()
	defer a.RUnlock()
	return a.mode
}
func (a *DirtyAttr) SetMode(mode os.FileMode) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.mode = mode
	return nil
}
func (a *DirtyAttr) Size() int64 {
	a.RLock()
	defer a.RUnlock()
	return a.size
}
func (a *DirtyAttr) SetSize(size int64) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.size = size
	return nil
}

func (a *DirtyAttr) Uid() uint32 {
	a.RLock()
	defer a.RUnlock()
	return a.uid
}

func (a *DirtyAttr) SetUID(uid uint32) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.uid = uid
	return nil
}

func (a *DirtyAttr) Gid() uint32 {
	a.RLock()
	defer a.RUnlock()
	return a.gid
}

func (a *DirtyAttr) SetGID(gid uint32) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.gid = gid
	return nil

}

func (a *DirtyAttr) LinkTarget() string {
	a.RLock()
	defer a.RUnlock()
	return a.linkTarget
}

func (a *DirtyAttr) Rdev() uint32 {
	a.RLock()
	defer a.RUnlock()
	return a.dev
}

func (a *DirtyAttr) SetRdev(dev uint32) error {
	a.Lock()
	defer a.Unlock()
	a.dirty = true
	a.dev = dev
	return nil

}

func (a *DirtyAttr) wrap() *FsEntry {
	a.RLock()
	defer a.RUnlock()
	return &FsEntry{
		Inode:       a.ino,
		Mode:        a.mode,
		Size:        uint64(a.size),
		IsDirectory: a.mode.IsDir(),

		Nlink:      int(a.nlink),
		LinkTarget: a.linkTarget,
		Dev:        a.dev,
		Ctime:      a.ctime,
		Mtime:      a.mtime,
		Atime:      a.atime,
		Uid:        a.uid,
		Gid:        a.gid,
	}
}

func (a *DirtyAttr) unwrap(e *FsEntry) {
	a.Lock()
	defer a.Unlock()

	if a.dirty {
		return
	}

	a.atime = e.Atime
	a.mtime = e.Mtime
	a.ctime = e.Ctime
	a.nlink = e.Nlink
	a.linkTarget = e.LinkTarget
	a.dev = e.Dev
	a.size = int64(e.Size)
	a.mode = e.Mode
	a.uid = e.Uid
	a.gid = e.Gid
}
