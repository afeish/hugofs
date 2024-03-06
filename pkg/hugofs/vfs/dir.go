package vfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	_ Node = (*Dir)(nil)
)

type Dir struct {
	vfs *VFS
	fs  *Fs

	inode uint64
	mu    sync.RWMutex

	Parent *Dir
	path   string

	sys   atomic.Value
	items *items

	read    time.Time
	embeded *DirtyAttr
	id      string

	lg *zap.Logger
}

type items struct {
	mu     sync.RWMutex
	nodes  map[string]Node
	health map[string]bool

	parent *Dir
	vfs    *VFS
}

func newItems(dir *Dir) *items {
	_items := &items{
		nodes:  make(map[string]Node),
		health: make(map[string]bool),
		parent: dir,
	}
	if dir != nil {
		_items.vfs = dir.vfs
	}
	return _items
}

func (i *items) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.nodes)
}

func (i *items) Delete(name string) (n Node, ok bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if n, ok = i.nodes[name]; !ok {
		return nil, false
	}
	delete(i.nodes, name)
	delete(i.health, name)
	return
}
func (i *items) Get(name string) (n Node, ok bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	n, ok = i.nodes[name]
	return
}

func (i *items) Set(name string, n Node) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.nodes[name] = n
	i.health[name] = true
}

func (i *items) updateOrInplace(entry *FsEntry) (n Node) {
	i.mu.Lock()
	defer i.mu.Unlock()

	name := entry.Name
	n, ok := i.nodes[name]
	if ok {
		if n.IsDir() {
			n.(*Dir).embeded.unwrap(entry)
		} else {
			n.(*File).embeded.unwrap(entry)
		}
		i.health[name] = true
		return n
	}

	if entry.IsDir() {
		n = newDirEntry(i.vfs, i.parent, entry)
	} else {
		n = newFileEntry(i.parent, i.parent._path(), entry)
	}

	i.nodes[name] = n
	i.health[name] = true
	return
}

func (i *items) invalidate() {
	i.mu.Lock()
	defer i.mu.Unlock()

	for name := range i.health {
		i.health[name] = false
	}
}

func (i *items) clean() {
	i.mu.Lock()
	defer i.mu.Unlock()
	maps.DeleteFunc(i.health, func(name string, exist bool) bool {
		if !exist {
			i.vfs.deleteNode(i.nodes[name].Inode(), name)
			delete(i.nodes, name)
			return true
		}
		return false
	})
}

func (i *items) Visit(fn NodeVisitor) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	for _, n := range i.nodes {
		if !fn(n) {
			return
		}
	}
}

func (i *items) SortItems() Nodes {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var nodes Nodes
	for _, n := range i.nodes {
		nodes = append(nodes, n)
	}

	sort.Stable(nodes)
	return nodes
}

// newDir Generates new tree
func newDirEntry(vfs *VFS, parent *Dir, fsEntry *FsEntry) *Dir {
	var (
		fs    *Fs
		lg    *zap.Logger
		dpath string = fsEntry.Name
	)
	if vfs == nil || vfs.Opt == nil || vfs.Opt.Logger == nil {
		lg = zap.NewNop()
	} else {
		lg = vfs.Opt.Logger.Named("<>")
	}

	if vfs != nil {
		fs = vfs.Fs()
	}

	n := &Dir{
		vfs:     vfs,
		fs:      fs,
		Parent:  parent,
		inode:   fsEntry.Inode,
		path:    dpath,
		embeded: fsEntry.ToDirtyAttr(vfs.fs),
		lg:      lg,

		id: NextSnowIDStr(),
	}
	n.items = newItems(n)
	return n
}

// newDir Generates new tree
func newDir(vfs *VFS) *Dir {
	var root *Dir

	if vfs != nil && vfs.root == nil {
		entry, _ := vfs.fs.Get(context.Background(), 1)
		entry.Name = "."
		vfs.fs.lg.Sugar().Debugf("init root: nlink=%d", entry.Nlink)
		root = newDirEntry(vfs, nil, entry)
		vfs.addNode(root)
	} else {
		umask := 022
		mode := os.ModeDir | os.FileMode(0777&^umask)
		root = &Dir{vfs: vfs, path: ".", inode: 1, embeded: &DirtyAttr{mode: mode}, items: newItems(nil)}
	}

	return root
}

func (d *Dir) Name() (name string) {
	// d.mu.RLock()
	name = path.Base(d.path)
	// d.mu.RUnlock()
	if name == "." {
		name = "/"
	}
	return name
}
func (d *Dir) Size() int64 {
	return 4096
}
func (d *Dir) Ctime() time.Time {
	return d.embeded.CTime()
}

func (d *Dir) Atime() time.Time {
	return d.embeded.ATime()
}
func (d *Dir) Mode() os.FileMode {
	return d.embeded.Mode()
}
func (d *Dir) ModTime() time.Time {
	return d.embeded.ModTime()
}
func (d *Dir) Nlink() int {
	return d.embeded.Nlink()
}

func (d *Dir) ID() string {
	return d.id
}

func (d *Dir) Reload(ctx context.Context) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d._reloadAttr(ctx)
}

func (d *Dir) _reloadAttr(ctx context.Context) error {
	entry, err := d.vfs.fs.Get(ctx, d.inode)
	if err != nil {
		s, ok := status.FromError(err)
		if ok && s.Code() == codes.NotFound {
			d.embeded.SetNlink(0)
			return nil
		} else {
			d.lg.Error("reload link attr error", zap.Error(err))
			return err
		}
	}

	d.embeded.unwrap(entry)
	return nil
}
func (d *Dir) IsDir() bool {
	return true
}
func (d *Dir) Sys() any {
	return d.sys.Load()
}
func (d *Dir) IsFile() bool {
	return false
}
func (d *Dir) Inode() uint64 {
	return d.inode
}

func (d *Dir) Uid() uint32 {
	return d.embeded.Uid()
}
func (d *Dir) Gid() uint32 {
	return d.embeded.Gid()
}

func (d *Dir) Rdev() uint32 {
	return d.embeded.Rdev()

}

// Node returns the Node associated with this - satisfies Noder interface
func (d *Dir) Node() Node {
	return d
}

func (d *Dir) SetModTime(modTime time.Time) error {
	return d.embeded.SetModTime(modTime)
}

func (d *Dir) SetMode(mode os.FileMode) error {
	return d.embeded.SetMode(mode)
}

func (d *Dir) SetATime(t time.Time) error {
	return d.embeded.SetATime(t)
}
func (d *Dir) SetCTime(t time.Time) error {
	return d.embeded.SetCTime(t)
}
func (d *Dir) SetUID(uid uint32) error {
	return d.embeded.SetUID(uid)
}
func (d *Dir) SetGID(gid uint32) error {
	return d.embeded.SetGID(gid)
}

func (d *Dir) SetSize(size int64) error {
	return d.embeded.SetSize(size)
}
func (d *Dir) SetNlink(nlink int) error {
	return d.embeded.SetNlink(nlink)
}

func (d *Dir) Sync() error {
	return nil
}

func (d *Dir) isEmpty() (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	err := d._readDir(false, "check dir empty or not")
	if err != nil {
		return false, err
	}

	return d.items.Len() == 0, nil
}

func (d *Dir) Truncate(size int64) error {
	return nil
}

func (d *Dir) Flush(ctx context.Context) (err error) {
	if err = d.embeded.Flush(ctx); err != nil {
		return
	}

	d.mu.Lock()
	d.read = time.Time{}
	d.mu.Unlock()
	return nil
}
func (d *Dir) Path() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d._path()
}

func (d *Dir) _path() string {
	parent := d.Parent
	if parent == nil {
		return d.path
	}
	return path.Join(parent.Path(), d.path)
}

func (d *Dir) SetSys(v any) {
	d.sys.Store(v)
}

func (d *Dir) delete(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.items.Delete(name)
	if ok {
		d.vfs.deleteNode(n.Inode(), name)
	}
}

func (d *Dir) add(n Node) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items.Set(n.Name(), n)
	d.vfs.addNode(n)
	d._reloadAttr(context.Background())
}

func (d *Dir) children() Nodes {
	return d.items.SortItems()
}

func (d *Dir) GetParent() Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Parent
}

func (d *Dir) Get(ino uint64) Node {
	if d.inode == ino {
		return d
	}
	var n Node
	d.VisitAll(func(item Node) bool {
		if item.Inode() == ino {
			n = item
			return false
		}
		return true
	})
	return n
}

func (d *Dir) Open(flags int) (Handle, error) {
	return newDirHandle(d), nil
}

func (d *Dir) VFS() *VFS {
	return d.vfs
}

func (d *Dir) Readdir(n int) ([]FileInfo, error) {
	return nil, nil
}

func (d *Dir) VisitAll(fn NodeVisitor) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	d._visit(fn)
}

func (d *Dir) _visit(fn NodeVisitor) {
	if d.Name() != "/" {
		fn(d)
	}

	d.items.Visit(func(node Node) bool {
		if dir, ok := node.(*Dir); ok {
			dir.VisitAll(fn)
		} else {
			fn(node)
		}
		return true
	})
}

// ReadDirAll reads the contents of the directory sorted
func (d *Dir) ReadDirAll() (items Nodes, err error) {
	// d.mu.Lock()
	err = d._readDir(false, "read dir all")
	if err != nil {
		d.lg.Error("Dir.ReadDirAll error", zap.Error(err))
		// d.mu.Unlock()
		return nil, err
	}

	items = d.children()
	// d.mu.Unlock()

	return items, nil
}

func (d *Dir) Stat(name string) (node Node, err error) {
	node, err = d.stat(name)
	if err != nil {
		if err != ENOENT {
			d.lg.Error("Dir.Stat error: %v", zap.Any("path", name))
		}
		return nil, err
	}
	return node, nil
}

// Create makes a new file node
func (d *Dir) Create(ctx context.Context, name string, mode, rdev uint32) (f *File, err error) {
	// This gets added to the directory when the file is opened for write
	d.mu.RLock()
	if _, ok := d.items.Get(name); ok {
		d.mu.RUnlock()
		return nil, EEXIST
	}
	d.mu.RUnlock()

	entry, err := d.vfs.fs.Create(ctx, d.inode, name, mode, rdev)
	if err != nil {
		return nil, err
	}
	d.lg.Sugar().Debugf("create file: %s", path.Join(d._path(), name))
	f = newFileEntry(d, d.Path(), entry)

	d.add(f)
	return
}

// Create makes a new file node
func (d *Dir) Symlink(ctx context.Context, target, linkName string) (f *File, err error) {
	// This gets added to the directory when the file is opened for write
	d.mu.RLock()
	if _, ok := d.items.Get(linkName); ok {
		d.mu.RUnlock()
		return nil, EEXIST
	}
	d.mu.RUnlock()

	entry, err := d.vfs.fs.Symlink(ctx, d.inode, target, linkName)
	if err != nil {
		return nil, err
	}
	d.lg.Sugar().Debugf("create symlink [ %s ] to [ %s ]", path.Join(d._path(), target), linkName)
	f = newFileEntry(d, d.Path(), entry)

	d.add(f)
	return
}

// Link makes a hard link node to the origin
func (d *Dir) Link(ctx context.Context, target Node, linkName string) (f *File, err error) {
	d.mu.RLock()
	if _, ok := d.items.Get(linkName); ok {
		d.mu.RUnlock()
		return nil, EEXIST
	}
	d.mu.RUnlock()

	entry, err := d.vfs.fs.Link(ctx, target.Inode(), d.inode, linkName)
	if err != nil {
		return nil, err
	}
	entry.Name = linkName
	f = newFileEntry(d, d.Path(), entry)

	link := path.Join(d._path(), linkName)
	d.lg.Sugar().Debugf("create hard link [ %s ] to [ %s ]", link, target.Path())
	target.(*File).embeded.unwrap(entry)

	d.add(f)
	return
}

func (d *Dir) Remove(ctx context.Context) error {
	// Check directory is empty first
	empty, err := d.isEmpty()
	if err != nil {
		return err
	}
	if !empty {
		return ENOTEMPTY
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// remove directory
	err = d.vfs.fs.Rmdir(ctx, d.Parent.Inode(), d.Name())
	if err != nil {
		return err
	}

	if d.VFS().Opt.Debug { // remove any possible files
		if err = os.RemoveAll(d._path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	// Remove the item from the parent directory listing
	if d.Parent != nil {
		d.Parent.delete(d.Name())
		return d.Parent.Reload(ctx)
	}
	return nil
}

func (d *Dir) RemoveAll(ctx context.Context) (err error) {
	nodes, err := d.ReadDirAll()
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if err = node.RemoveAll(ctx); err != nil {
			return
		}
	}
	return d.Remove(ctx)
}

// Mkdir creates a new directory
func (d *Dir) Mkdir(ctx context.Context, name string, mode uint32) (*Dir, error) {
	d.mu.RLock()
	if _, ok := d.items.Get(name); ok {
		d.mu.RUnlock()
		return nil, EEXIST
	}
	d.mu.RUnlock()

	path := path.Join(d._path(), name)

	d.lg.Sugar().Debugf("mkdir: %s, perm: %s", path, os.FileMode(mode).Perm())
	entry, err := d.vfs.fs.Mkdir(ctx, d.inode, name, mode)
	if err != nil {
		d.lg.Error(fmt.Sprintf("mkdir [ %s ] error", path), zap.Error(err))
		return nil, err
	}
	entry.Name = name
	dir := newDirEntry(d.vfs, d, entry)

	d.add(dir)
	return dir, nil
}

func (d *Dir) Rename(ctx context.Context, oldName, newName string, destDir *Dir) (err error) {
	oldPath := path.Join(d.Path(), oldName)
	newPath := path.Join(destDir.Path(), newName)

	d.lg.Sugar().Debugf("rename %s -> %s", oldPath, newPath)
	oldNode, ok := d.items.Get(oldName)
	if !ok {
		return ENOENT
	}

	if oldNode.IsDir() {
		if oldDir, ok := oldNode.(*Dir); ok { // move the entire old directory into the dest dir
			phantomNode, err := destDir.Stat(newName)
			if phantomNode != nil && err == nil {
				if phantomNode.IsFile() {
					return ENOTDIR
				}
				if phantomNode.IsDir() {
					isEmpty, _ := phantomNode.(*Dir).isEmpty()
					if !isEmpty {
						return ENOTEMPTY
					}
				}
			}

			if err = d.moveChildTo(ctx, oldName, destDir, newName); err != nil {
				d.lg.Error("move dir to dest error", zap.Error(err))
			}
			if d != destDir {
				if err = oldDir.moveTo(destDir); err != nil {
					return err
				}
			} else {
				d.lg.Sugar().Debugf("rename in same dir %s -> %s", oldPath, newPath)
				oldDir.path = newName
				destDir.add(oldDir)
			}
		}
	} else {
		if f, ok := oldNode.(*File); ok {
			if err = f.rename(ctx, destDir, newName); err != nil {
				d.lg.Error("rename file error", zap.Error(err))
				return err
			}
		}
	}

	d.read = time.Time{}
	d.ReadDirAll() // reload attr from backend as the nlink may changed when encountering the directory rename

	// force the reload of dest directory (MAYBE affect the opening file ???)
	destDir.read = time.Time{}
	destDir.ReadDirAll() // reload attr from backend as the nlink may changed when encountering the directory rename

	return nil
}

func (d *Dir) moveChildTo(ctx context.Context, oldName string, dstDir *Dir, newName string) error {
	if _, err := d.vfs.fs.Rename(ctx, d.inode, oldName, dstDir.inode, newName); err != nil {
		return err
	}
	return nil
}

// moveTo should be called after the directory is renamed
//
// Reset the directory to new state, discarding all the objects and
// reading everything again
func (d *Dir) moveTo(dstDir *Dir) error {
	d.ForgetAll()

	d.mu.Lock()
	d.Parent = dstDir
	d.read = time.Time{}
	d.mu.Unlock()

	dstDir.add(d)
	return nil
}

// ForgetAll forgets directory entries for this directory and any children.
//
// It does not invalidate or clear the cache of the parent directory.
//
// It returns true if the directory or any of its children had virtual entries
// so could not be forgotten. Children which didn't have virtual entries and
// children with virtual entries will be forgotten even if true is returned.
func (d *Dir) ForgetAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lg.Sugar().Debugf("forgetting directory [ %s ] cache", d._path())

	d.items.Visit(func(n Node) bool {
		if dir, ok := n.(*Dir); ok {
			dir.ForgetAll()
		}
		return true
	})

	d.read = time.Time{}
	d.items = newItems(d)
}

// stat a single item in the directory
//
// returns ENOENT if not found.
// returns a custom error if directory on a case-insen	sitive file system
// contains files with names that differ only by case.
func (d *Dir) stat(leaf string) (Node, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d._readDir(false, "stat dir")
	if err != nil {
		return nil, err
	}
	item, ok := d.items.Get(leaf)

	if !ok {
		return nil, ENOENT
	}
	return item, nil
}

func (d *Dir) readDirRecursive() error {
	// d.mu.Lock()
	// defer d.mu.Unlock()
	err := d._readDir(true, "read dir recrusive")
	if err != nil {
		return err
	}
	return nil
}

func (d *Dir) _readDir(recursive bool, reason string) error {
	when := time.Now()
	if age, stale := d._age(when); stale {
		if age != 0 {
			d.lg.Sugar().Debugf("[ %s ] Re-reading directory (%v old), ino=%d, name=%s, id=%s", reason, age, d.inode, d._path(), d.id)
		}
	} else {
		d.lg.Sugar().Debugf("[ %s ] Read existing directory (%v old),  ino=%d, name=%s, id=%s", reason, age, d.inode, d._path(), d.id)
		return nil
	}

	entries, err := d.vfs.fs.List(context.Background(), d.inode)
	if err != nil {
		return err
	}

	d.items.invalidate()

	var node Node
	for _, entry := range entries {
		if entry.IsDir() {
			node = d.items.updateOrInplace(&entry)
			if recursive {
				if err = node.(*Dir).readDirRecursive(); err != nil {
					return err
				}
			}
		} else {
			d.items.updateOrInplace(&entry)
		}
	}
	d.items.clean()
	d._reloadAttr(context.Background())

	d.read = when
	return nil
}

// age returns the duration since the last time the directory contents
// was read and the content is considered stale. age will be 0 and
// stale true if the last read time is empty.
// age must be called with d.mu held.
func (d *Dir) _age(when time.Time) (age time.Duration, stale bool) {
	if d.read.IsZero() {
		return age, true
	}
	age = when.Sub(d.read)
	stale = age > d.vfs.Opt.DirCacheTime
	return
}
