package mount

import (
	"os"
	"path"
	"syscall"

	"github.com/afeish/hugo/pkg/hugofs/vfs"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// fileInfos is a slice of Node
type fileInfos []vfs.FileInfo

// Sort functions
func (ns fileInfos) Len() int           { return len(ns) }
func (ns fileInfos) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns fileInfos) Less(i, j int) bool { return ns[i].Name() < ns[j].Name() }

type dirStream struct {
	nodes fileInfos
	i     int
}

// HasNext indicates if there are further entries. HasNext
// might be called on already closed streams.
func (ds *dirStream) HasNext() bool {
	return ds.i < len(ds.nodes)
}

// Next retrieves the next entry. It is only called if HasNext
// has previously returned true.  The Errno return may be used to
// indicate I/O errors
func (ds *dirStream) Next() (de fuse.DirEntry, errno syscall.Errno) {
	fi := ds.nodes[ds.i]

	de = fuse.DirEntry{
		// Mode is the file's mode. Only the high bits (e.g. S_IFDIR)
		// are considered.
		Mode: getMode(fi),

		// Name is the basename of the file in the directory.
		Name: path.Base(fi.Name()),

		// Ino is the inode number.
		Ino: fi.Inode(),
	}
	ds.i++
	return de, 0
}

// Close releases resources related to this directory
// stream.
func (ds *dirStream) Close() {
}

var _ fusefs.DirStream = (*dirStream)(nil)

type dummyFileInfo struct {
	vfs.FileInfo
	name  string
	mode  os.FileMode
	inode uint64
}

func (fi *dummyFileInfo) Mode() os.FileMode {
	return fi.mode
}

func (fi *dummyFileInfo) Name() string {
	return fi.name
}

func (fi *dummyFileInfo) Inode() uint64 {
	return fi.inode
}

var _ vfs.FileInfo = (*dummyFileInfo)(nil)
