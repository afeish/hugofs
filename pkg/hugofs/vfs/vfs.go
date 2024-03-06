package vfs

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/pkg/hugofs/vfs/buffer"
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"go.uber.org/zap"
)

type Options struct {
	DirCacheTime time.Duration
}

type nodeKey string

func newNodeKey(ino uint64, name string) nodeKey {
	return nodeKey(fmt.Sprintf("%d-%s", ino, name))
}

func (k nodeKey) belongTo(ino uint64) bool {
	return strings.HasPrefix(string(k), fmt.Sprintf("%d-", ino))
}

type VFS struct {
	fs   *Fs
	root *Dir

	mu       sync.RWMutex
	nodeMap  map[nodeKey]Node
	memThrot *buffer.Throttler

	Opt *mountlib.Options
}

func New(fs *Fs, opts *mountlib.Options) *VFS {
	vfs := &VFS{
		fs:      fs,
		nodeMap: make(map[nodeKey]Node),
		Opt:     opts,
	}

	vfs.root = newDir(vfs)

	vfs.memThrot = buffer.NewThrottler(context.Background(), opts.Env.Cache.MemSize.Int()/opts.Env.BlockSize.Int(), opts.Logger)
	meta.SetWritePageCtx(meta.NewPageContext(int64(opts.Env.Cache.FileWriteSize), int64(opts.Env.PageSize)))
	meta.SetReadPageCtx(meta.NewPageContext(int64(opts.Env.Cache.FileReadSize), int64(opts.Env.PageSize)))
	return vfs
}

func (vfs *VFS) Fs() *Fs {
	return vfs.fs
}

func (vfs *VFS) Root() (*Dir, error) {
	return vfs.root, nil
}

// Stat find the given path recursively to the last element of path.
//
// It takes a path string as a parameter.
// It returns a Node and an error.
func (vfs *VFS) Stat(path string) (node Node, err error) {
	path = strings.Trim(path, "/")
	node = vfs.root
	for path != "" {
		i := strings.IndexRune(path, '/')
		var name string
		if i < 0 {
			name, path = path, ""
		} else {
			name, path = path[:i], path[i+1:]
		}
		if name == "" {
			continue
		}
		dir, ok := node.(*Dir)
		if !ok {
			// We need to look in a directory, but found a file
			return nil, ENOENT
		}
		node, err = dir.Stat(name)
		if err != nil {
			return nil, err
		}
	}
	return
}

// StatParent finds the parent directory and the leaf name of a path
func (vfs *VFS) StatParent(name string) (dir *Dir, leaf string, err error) {
	name = strings.Trim(name, "/")
	parent, leaf := path.Split(name)
	node, err := vfs.Stat(parent)
	if err != nil {
		return nil, "", err
	}
	if node.IsFile() {
		return nil, "", os.ErrExist
	}
	dir = node.(*Dir)
	return dir, leaf, nil
}

// Walk traverse the given path recursively in the VFS.
//
// It takes a path string as a parameter.
// It returns a Node and an error.
func (vfs *VFS) Walk(path string) (node Node, err error) {
	node, err = vfs.Stat(path)
	if node == nil || node.IsFile() {
		return
	}
	dir := node.(*Dir)
	err = dir.readDirRecursive()
	if err != nil {
		return
	}
	return dir, nil
}

// ReadDir reads the directory named by dirname and returns
// a list of directory entries sorted by filename.
func (vfs *VFS) ReadDir(dirname string) ([]FileInfo, error) {
	f, err := vfs.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, nil
}

// accessModeMask masks off the read modes from the flags
const accessModeMask = (os.O_RDONLY | os.O_WRONLY | os.O_RDWR)

// Open opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode O_RDONLY.
func (vfs *VFS) Open(name string) (Handle, error) {
	return vfs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile a file according to the flags and perm provided
func (vfs *VFS) OpenFile(name string, flags int, perm os.FileMode) (fd Handle, err error) {

	// http://pubs.opengroup.org/onlinepubs/7908799/xsh/open.html
	// The result of using O_TRUNC with O_RDONLY is undefined.
	// Linux seems to truncate the file, but we prefer to return EINVAL
	if flags&accessModeMask == os.O_RDONLY && flags&os.O_TRUNC != 0 {
		return nil, EINVAL
	}

	node, err := vfs.Stat(name)
	if err != nil {
		if err != ENOENT || flags&os.O_CREATE == 0 {
			return nil, err
		}
		// If not found and O_CREATE then create the file
		dir, leaf, err := vfs.StatParent(name)
		if err != nil {
			return nil, err
		}
		node, err = dir.Create(context.Background(), leaf, uint32(perm), 0)
		if err != nil {
			return nil, err
		}
	}
	return node.Open(flags)
}

// Mkdir creates a new directory with the specified name and permission bits
// (before umask).
func (vfs *VFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	dir, leaf, err := vfs.StatParent(name)
	if err != nil {
		return err
	}
	_, err = dir.Mkdir(ctx, leaf, uint32(perm))
	if err != nil {
		return err
	}
	return nil
}

// Rmdir remove a directory
func (vfs *VFS) Rmdir(ctx context.Context, name string, recursive bool) error {
	dir, err := vfs.Stat(name)
	if err != nil {
		return err
	}
	if !recursive {
		return dir.Remove(ctx)
	}

	return dir.RemoveAll(ctx)
}

// Remove remove a file
func (vfs *VFS) Remove(ctx context.Context, name string) error {
	file, err := vfs.Stat(name)
	if err != nil {
		return err
	}
	return file.Remove(ctx)
}

func (vfs *VFS) Refresh(ctx context.Context, ino uint64) error {
	vfs.root.lg.Debug("refresh vfs...", zap.Any("ino", ino))

	vfs.root.VisitAll(func(n Node) bool {
		if n.Inode() == ino {
			n.Reload(ctx)
		}
		return true
	})
	return nil
}

// func (vfs *VFS) dumpNodeMap() string {
// 	vfs.mu.RLock()
// 	defer vfs.mu.RUnlock()

// 	var sb strings.Builder
// 	sb.WriteRune('[')
// 	for key := range vfs.nodeMap {
// 		sb.WriteString(fmt.Sprintf("node: %s, ", key))
// 	}
// 	sb.WriteRune(']')
// 	return sb.String()
// }

func (vfs *VFS) addNode(n Node) {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()
	key := newNodeKey(n.Inode(), n.Name())
	_, ok := vfs.nodeMap[key]
	if !ok {
		vfs.nodeMap[key] = n
	}
}

func (vfs *VFS) deleteNode(inode uint64, name string) {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()
	key := newNodeKey(inode, name)
	delete(vfs.nodeMap, key)
}

func (vfs *VFS) getNodes(inode uint64) []Node {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	nodes := make([]Node, 0)
	for key, n := range vfs.nodeMap {
		if key.belongTo(inode) {
			nodes = append(nodes, n)
		}
	}
	return nodes
}
