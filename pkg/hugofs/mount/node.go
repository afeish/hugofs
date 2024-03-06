package mount

import (
	"context"
	"math"
	"os"
	"path"
	"sort"
	"syscall"

	"github.com/afeish/hugo/pkg/hugofs/vfs"
	"github.com/afeish/hugo/pkg/util"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.uber.org/zap"
)

// hugoInode represents a directory or file
type hugoInode struct {
	fusefs.Inode
	vfsNode vfs.Node
	fsys    *FS

	lg *zap.Logger
}

// Node types must be InodeEmbedders
var _ fusefs.InodeEmbedder = (*hugoInode)(nil)

// newhugoInode creates a new fusefs.Node from a vfs Node
func newhugoInode(fsys *FS, vfsNode vfs.Node) (node *hugoInode) {
	// Check the vfsNode to see if it has a fuse Node cached
	// We must return the same fuse nodes for vfs Nodes
	node, ok := vfsNode.Sys().(*hugoInode)
	if ok {
		return node
	}
	node = &hugoInode{
		vfsNode: vfsNode,
		fsys:    fsys,
		lg:      fsys.opt.Logger.Named("node"),
	}
	// Cache the node for later
	vfsNode.SetSys(node)
	return node
}

func (n *hugoInode) StableAttr() fusefs.StableAttr {
	return fusefs.StableAttr{Mode: getMode(n.vfsNode), Ino: n.vfsNode.Inode()}
}

// String used for pretty printing.
func (n *hugoInode) String() string {
	return n.vfsNode.Path()
}

// lookup a Node in a directory
func (n *hugoInode) lookupVfsNodeInDir(leaf string) (vfsNode vfs.Node, errno syscall.Errno) {
	dir, ok := n.vfsNode.(*vfs.Dir)
	if !ok {
		return nil, syscall.ENOTDIR
	}
	vfsNode, err := dir.Stat(leaf)
	return vfsNode, translateError(err)
}

// Statfs implements statistics for the filesystem that holds this
// Inode. If not defined, the `out` argument will zeroed with an OK
// result.  This is because OSX filesystems must Statfs, or the mount
// will not work.
func (n *hugoInode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	const blockSize = 4096

	statCache, _ := n.fsys.Statistics()
	if statCache == nil {
		const fsBlocks = (1 << 50) / blockSize
		out.Blocks = fsBlocks  // Total data blocks in file system.
		out.Bfree = fsBlocks   // Free blocks in file system.
		out.Bavail = fsBlocks  // Free blocks in file system if you're not root.
		out.Files = 1e9        // Total files in file system.
		out.Ffree = 1e9        // Free files in file system.
		out.Bsize = blockSize  // Block size
		out.NameLen = 255      // Maximum file name length?
		out.Frsize = blockSize // Fragment size, smallest addressable data size in the file system.
		return 0
	}

	totalDiskSize := statCache.Cap
	usedDiskSize := statCache.Usage
	actualFileCount := statCache.TotalFileCnt
	// Compute the total number of available blocks
	out.Blocks = totalDiskSize / blockSize
	if out.Blocks <= 0 {
		out.Blocks = 1
	}
	// Compute the number of used blocks
	numBlocks := usedDiskSize / blockSize
	remainingBlocks := int64(out.Blocks) - int64(numBlocks)
	if remainingBlocks < 0 {
		remainingBlocks = 0
	}
	// Report the number of free and available blocks for the block size
	out.Bfree = uint64(remainingBlocks)
	out.Bavail = uint64(remainingBlocks)
	out.Bsize = uint32(blockSize)

	// Report the total number of possible files in the file system (and those free)
	out.Files = math.MaxInt64
	out.Ffree = math.MaxInt64 - actualFileCount

	// Report the maximum length of a name and the minimum fragment size
	out.NameLen = 255
	out.Frsize = uint32(blockSize)
	return 0
}

var _ = (fusefs.NodeStatfser)((*hugoInode)(nil))

// Getattr reads attributes for an Inode. The library will ensure that
// Mode and Ino are set correctly. For files that are not opened with
// FOPEN_DIRECTIO, Size should be set so it can be read correctly.  If
// returning zeroed permissions, the default behavior is to change the
// mode of 0755 (directory) or 0644 (files). This can be switched off
// with the Options.NullPermissions setting. If blksize is unset, 4096
// is assumed, and the 'blocks' field is set accordingly.
func (n *hugoInode) Getattr(ctx context.Context, f fusefs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	n.fsys.setAttrOut(n.vfsNode, out)
	return 0
}

var _ = (fusefs.NodeGetattrer)((*hugoInode)(nil))

// Setattr sets attributes for an Inode.
func (n *hugoInode) Setattr(ctx context.Context, f fusefs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) (errno syscall.Errno) {
	var err error
	size, ok := in.GetSize()
	if ok {
		err = n.vfsNode.Truncate(int64(size))
		if err != nil {
			n.lg.Error("truncate file error", zap.Error(err))
			return translateError(err)
		}
	}
	mtime, ok := in.GetMTime()
	if ok {
		err = n.vfsNode.SetModTime(mtime)
		if err != nil {
			return translateError(err)
		}
	}
	atime, ok := in.GetATime()
	if ok {
		err = n.vfsNode.SetATime(atime)
		if err != nil {
			return translateError(err)
		}
	}

	ctime, ok := in.GetCTime()
	if ok {
		err = n.vfsNode.SetCTime(ctime)
		if err != nil {
			return translateError(err)
		}
	}

	_, ok = in.GetMode() // the return mode is permission bits(e.g. 0644)
	if ok {
		n.lg.Sugar().Debugf("change mode to %#o", in.Mode)
		err = n.vfsNode.SetMode(util.ToFileMode(in.Mode))
		if err != nil {
			return translateError(err)
		}
	}

	uid, ok := in.GetUID()
	if ok {
		err = n.vfsNode.SetUID(uid)
		if err != nil {
			return translateError(err)
		}
	}

	gid, ok := in.GetGID()
	if ok {
		err = n.vfsNode.SetGID(gid)
		if err != nil {
			return translateError(err)
		}
	}
	ctx = NewGrpcContext(ctx, n.fsys.opt)
	ctx = vfs.NewAttrContext(ctx, int(in.Valid))
	err = n.vfsNode.Flush(ctx)
	if err != nil {
		return translateError(err)
	}

	n.fsys.setAttrOut(n.vfsNode, out)
	return 0
}

var _ = (fusefs.NodeSetattrer)((*hugoInode)(nil))

// Open opens an Inode (of regular file type) for reading. It
// is optional but recommended to return a FileHandle.
func (n *hugoInode) Open(ctx context.Context, flags uint32) (fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if n.fsys.isOverQuota {
		return nil, fuseFlags, syscall.ENOSPC
	}

	// fuse flags are based off syscall flags as are os flags, so
	// should be compatible
	handle, err := n.vfsNode.Open(int(flags))
	if err != nil {
		return nil, 0, translateError(err)
	}
	// // If size unknown then use direct io to read
	// if entry := n.node.DirEntry(); entry != nil && entry.Size() < 0 {
	// 	fuseFlags |= fuse.FOPEN_DIRECT_IO
	// }
	return newFileHandle(handle, n.fsys), fuseFlags, 0
}

var _ = (fusefs.NodeOpener)((*hugoInode)(nil))

// Lookup should find a direct child of a directory by the child's name.  If
// the entry does not exist, it should return ENOENT and optionally
// set a NegativeTimeout in `out`. If it does exist, it should return
// attribute data in `out` and return the Inode for the child. A new
// inode can be created using `Inode.NewInode`. The new Inode will be
// added to the FS tree automatically if the return status is OK.
//
// If a directory does not implement NodeLookuper, the library looks
// for an existing child with the given name.
//
// The input to a Lookup is {parent directory, name string}.
//
// Lookup, if successful, must return an *Inode. Once the Inode is
// returned to the kernel, the kernel can issue further operations,
// such as Open or Getxattr on that node.
//
// A successful Lookup also returns an EntryOut. Among others, this
// contains file attributes (mode, size, mtime, etc.).
//
// FUSE supports other operations that modify the namespace. For
// example, the Symlink, Create, Mknod, Link methods all create new
// children in directories. Hence, they also return *Inode and must
// populate their fuse.EntryOut arguments.
func (n *hugoInode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (inode *fusefs.Inode, errno syscall.Errno) {
	// check if the pathname is valid
	if errno = checkLength(name); errno != syscall.F_OK {
		return
	}
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return nil, errno
	}
	newNode := newhugoInode(n.fsys, vfsNode)
	n.fsys.setEntryOut(vfsNode, out)
	return n.NewInode(ctx, newNode, newNode.StableAttr()), 0
}

var _ = (fusefs.NodeLookuper)((*hugoInode)(nil))

// Opendir opens a directory Inode for reading its
// contents. The actual reading is driven from Readdir, so
// this method is just for performing sanity/permission
// checks. The default is to return success.
func (n *hugoInode) Opendir(ctx context.Context) syscall.Errno {
	if !n.vfsNode.IsDir() {
		return syscall.ENOTDIR
	}
	return 0
}

var _ = (fusefs.NodeOpendirer)((*hugoInode)(nil))

// Readdir opens a stream of directory entries.
//
// Readdir essentially returns a list of strings, and it is allowed
// for Readdir to return different results from Lookup. For example,
// you can return nothing for Readdir ("ls my-fuse-mount" is empty),
// while still implementing Lookup ("ls my-fuse-mount/a-specific-file"
// shows a single file).
//
// If a directory does not implement NodeReaddirer, a list of
// currently known children from the tree is returned. This means that
// static in-memory file systems need not implement NodeReaddirer.
func (n *hugoInode) Readdir(ctx context.Context) (ds fusefs.DirStream, errno syscall.Errno) {
	if !n.vfsNode.IsDir() {
		return nil, syscall.ENOTDIR
	}
	fh, err := n.vfsNode.Open(os.O_RDONLY)
	if err != nil {
		return nil, translateError(err)
	}
	defer func() {
		closeErr := fh.Close()
		if errno == 0 && closeErr != nil {
			errno = translateError(closeErr)
		}
	}()
	items, err := fh.Readdir(-1)
	if err != nil {
		return nil, translateError(err)
	}

	var (
		curDir    = &dummyFileInfo{name: ".", mode: n.vfsNode.Mode(), inode: n.vfsNode.Inode()}
		parentDir = &dummyFileInfo{name: "..", mode: os.ModeDir | os.ModePerm, inode: 0}
	)

	finfos := fileInfos{curDir, parentDir}
	finfos = append(finfos, items...)
	sort.Stable(finfos)
	return &dirStream{
		nodes: finfos,
	}, 0
}

var _ = (fusefs.NodeReaddirer)((*hugoInode)(nil))

// Mkdir is similar to Lookup, but must create a directory entry and Inode.
// Default is to return EROFS.
func (n *hugoInode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (inode *fusefs.Inode, errno syscall.Errno) {
	if n.fsys.isOverQuota {
		return nil, syscall.ENOSPC
	}
	// check if the pathname is valid
	if errno = checkLength(name); errno != syscall.F_OK {
		return
	}
	dir, ok := n.vfsNode.(*vfs.Dir)
	if !ok {
		return nil, syscall.ENOTDIR
	}
	path := path.Join(dir.Path(), name)
	if len(path) >= 4096 { // /usr/include/linux/limits.h
		return nil, syscall.ENAMETOOLONG
	}
	ctx = NewGrpcContext(ctx, n.fsys.opt)
	newDir, err := dir.Mkdir(ctx, name, mode)
	if err != nil {
		return nil, translateError(err)
	}
	newNode := newhugoInode(n.fsys, newDir)
	n.fsys.setEntryOut(newNode.vfsNode, out)
	newInode := n.NewInode(ctx, newNode, newNode.StableAttr())
	return newInode, 0
}

var _ = (fusefs.NodeMkdirer)((*hugoInode)(nil))

// Create is similar to Lookup, but should create a new
// child. It typically also returns a FileHandle as a
// reference for future reads/writes.
// Default is to return EROFS.
func (n *hugoInode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (node *fusefs.Inode, fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if n.fsys.isOverQuota {
		return nil, nil, flags, syscall.ENOSPC
	}
	// check if the pathname is valid
	if errno = checkLength(name); errno != syscall.F_OK {
		return
	}
	ctx = NewGrpcContext(ctx, n.fsys.opt)
	dir, ok := n.vfsNode.(*vfs.Dir)
	if !ok {
		return nil, nil, 0, syscall.ENOTDIR
	}
	path := path.Join(dir.Path(), name)
	if len(path) >= 4096 { // /usr/include/linux/limits.h
		return nil, nil, 0, syscall.ENAMETOOLONG
	}
	// translate the fuse flags to os flags
	osFlags := int(flags) | os.O_CREATE
	file, err := dir.Create(ctx, name, mode, 0)
	if err != nil {
		return nil, nil, 0, translateError(err)
	}
	handle, err := file.Open(osFlags)
	if err != nil {
		return nil, nil, 0, translateError(err)
	}
	fh = newFileHandle(handle, n.fsys)
	// FIXME
	// fh = &fusefs.WithFlags{
	// 	File: fh,
	// 	//FuseFlags: fuse.FOPEN_NONSEEKABLE,
	// 	OpenFlags: flags,
	// }

	// Find the created node
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return nil, nil, 0, errno
	}
	n.fsys.setEntryOut(vfsNode, out)
	newNode := newhugoInode(n.fsys, vfsNode)
	newInode := n.NewInode(ctx, newNode, newNode.StableAttr())
	return newInode, fh, 0, 0
}

var _ = (fusefs.NodeCreater)((*hugoInode)(nil))

// Mknod is similar to Lookup, but must create a device entry and Inode.
// Default is to return EROFS.
func (n *hugoInode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (node *fusefs.Inode, errno syscall.Errno) {
	if n.fsys.isOverQuota {
		return nil, syscall.ENOSPC
	}
	// check if the pathname is valid
	if errno = checkLength(name); errno != syscall.F_OK {
		return
	}
	ctx = NewGrpcContext(ctx, n.fsys.opt)
	dir, ok := n.vfsNode.(*vfs.Dir)
	if !ok {
		return nil, syscall.ENOTDIR
	}
	path := path.Join(dir.Path(), name)
	if len(path) >= 4096 { // /usr/include/linux/limits.h
		return nil, syscall.ENAMETOOLONG
	}
	// translate the fuse flags to os flags
	_, err := dir.Create(ctx, name, mode, dev)
	if err != nil {
		return nil, translateError(err)
	}
	// Find the created node
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return nil, errno
	}
	n.fsys.setEntryOut(vfsNode, out)
	newNode := newhugoInode(n.fsys, vfsNode)
	node = n.NewInode(ctx, newNode, newNode.StableAttr())
	return
}

var _ = (fusefs.NodeMknoder)((*hugoInode)(nil))

// Link is similar to Lookup, but must create a new link to an existing Inode.
// Default is to return EROFS.

func (n *hugoInode) Link(ctx context.Context, target fusefs.InodeEmbedder, name string, out *fuse.EntryOut) (link *fusefs.Inode, errno syscall.Errno) {
	if n.fsys.isOverQuota {
		return nil, syscall.ENOSPC
	}
	// check if the pathname is valid
	if errno = checkLength(name); errno != syscall.F_OK {
		return
	}
	dir, ok := n.vfsNode.(*vfs.Dir)
	if !ok {
		return nil, syscall.ENOTDIR
	}
	origNode, ok := target.(*hugoInode)
	if !ok {
		return nil, syscall.EIO
	}
	origFile, ok := origNode.vfsNode.(*vfs.File)
	if !ok {
		return nil, syscall.EPERM // try link a dir
	}
	ctx = NewGrpcContext(ctx, n.fsys.opt)
	linkNode, err := dir.Link(ctx, origFile, name)
	if err != nil {
		return nil, translateError(err)
	}

	n.fsys.setEntryOut(linkNode, out)
	newNode := newhugoInode(n.fsys, linkNode)
	link = n.NewInode(ctx, newNode, newNode.StableAttr())
	return link, 0
}

var _ = (fusefs.NodeLinker)((*hugoInode)(nil))

// Symlink is similar to Lookup, but must create a new symbolic link.
// Default is to return EROFS.
func (n *hugoInode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fusefs.Inode, errno syscall.Errno) {
	if n.fsys.isOverQuota {
		return nil, syscall.ENOSPC
	}
	// check if the pathname is valid
	if errno = checkLength(name); errno != syscall.F_OK {
		return
	}
	dir, ok := n.vfsNode.(*vfs.Dir)
	if !ok {
		return nil, syscall.ENOTDIR
	}

	ctx = NewGrpcContext(ctx, n.fsys.opt)
	symlink, err := dir.Symlink(ctx, target, name)
	if err != nil {
		return nil, translateError(err)
	}
	newNode := newhugoInode(n.fsys, symlink)
	n.fsys.setEntryOut(newNode.vfsNode, out)

	node = n.NewInode(ctx, newNode, newNode.StableAttr())
	return
}

var _ = (fusefs.NodeSymlinker)((*hugoInode)(nil))

// Readlink reads the content of a symlink.
func (n *hugoInode) Readlink(ctx context.Context) (out []byte, errno syscall.Errno) {
	symlink, ok := n.vfsNode.(*vfs.File)
	if !ok {
		return nil, syscall.ENOLINK
	}

	out = []byte(symlink.Link())
	return
}

var _ = (fusefs.NodeReadlinker)((*hugoInode)(nil))

// reload will iterate the tree and reload the attribute of the file. It's called by Unlink. Since when unlink changed, the hardlink needs to refresh
// to get the valid attritue.
func (n *hugoInode) reload(ctx context.Context, inode *fusefs.Inode, ino uint64) { // ugly solution to update the hardlink's attribute
	for _, child := range inode.Children() {
		node := child.Operations().(*hugoInode).vfsNode
		if node.Inode() == ino {
			node.(*vfs.File).Reload(ctx)
		}
		if node.IsDir() {
			n.reload(ctx, child, ino)
		}
	}
}

// Unlink should remove a child from this directory.  If the
// return status is OK, the Inode is removed as child in the
// FS tree automatically. Default is to return EROFS.
func (n *hugoInode) Unlink(ctx context.Context, name string) (errno syscall.Errno) {
	// check if the pathname is valid
	if errno = checkLength(name); errno != syscall.F_OK {
		return
	}
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return errno
	}
	if vfsNode.IsDir() {
		return syscall.EISDIR
	}
	ctx = NewGrpcContext(ctx, n.fsys.opt)
	err := vfsNode.Remove(ctx)
	if err != nil {
		n.lg.Sugar().Errorf("unlink [ %s ] error: %v", name, err)
	}
	if vfsNode.Nlink() > 1 {
		n.reload(ctx, n.Root(), vfsNode.Inode())
	}
	return translateError(err)
}

var _ = (fusefs.NodeUnlinker)((*hugoInode)(nil))

// Rmdir is like Unlink but for directories.
// Default is to return EROFS.
func (n *hugoInode) Rmdir(ctx context.Context, name string) (errno syscall.Errno) {
	// check if the pathname is valid
	if errno = checkLength(name); errno != syscall.F_OK {
		return
	}
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return errno
	}
	ctx = NewGrpcContext(ctx, n.fsys.opt)
	return translateError(vfsNode.Remove(ctx))
}

var _ = (fusefs.NodeRmdirer)((*hugoInode)(nil))

// Rename should move a child from one directory to a different
// one. The change is effected in the FS tree if the return status is
// OK. Default is to return EROFS.
func (n *hugoInode) Rename(ctx context.Context, oldName string, newParent fusefs.InodeEmbedder, newName string, flags uint32) (errno syscall.Errno) {
	if errno = checkInoName(n.vfsNode.Inode(), oldName); errno != syscall.F_OK {
		return
	}
	oldDir, ok := n.vfsNode.(*vfs.Dir)
	if !ok {
		return syscall.ENOTDIR
	}
	newParentNode, ok := newParent.(*hugoInode)
	if !ok {
		return syscall.EIO
	}

	newDir, ok := newParentNode.vfsNode.(*vfs.Dir)
	if !ok {
		return syscall.ENOTDIR
	}
	if errno = checkInoName(newDir.Inode(), newName); errno != syscall.F_OK {
		return
	}
	ctx = NewGrpcContext(ctx, n.fsys.opt)
	return translateError(oldDir.Rename(ctx, oldName, newName, newDir))
}

var _ = (fusefs.NodeRenamer)((*hugoInode)(nil))

func (n *hugoInode) Access(ctx context.Context, mask uint32) (errno syscall.Errno) {

	return 0
}

var _ = (fusefs.NodeAccesser)((*hugoInode)(nil))
