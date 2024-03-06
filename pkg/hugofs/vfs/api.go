package vfs

import (
	"context"
	"os"
	"time"
)

// Value defines any value
type Value interface{}

// MetaValue defines any meta value
type MetaValue interface{}

// NodeVisitor function type for iterating over nodes
type NodeVisitor func(item Node) bool

type EdgeType string

// Tree represents a tree structure with leaf-nodes and branch-nodes.
type Tree interface {
	// Dir converts a leaf-Node to a branch-Node,
	// applying this on a branch-Node does no effect.
	Dir() Tree
	// FindByMeta finds a Node whose meta value matches the provided one by reflect.DeepEqual,
	// returns nil if not found.
	FindByMeta(meta MetaValue) Tree
	// FindByValue finds a Node whose value matches the provided one by reflect.DeepEqual,
	// returns nil if not found.
	FindByValue(value Value) Tree
	//  returns the last Node of a tree
	FindLastNode() Tree
	// String renders the tree or subtree as a string.
	ToPrint() string
	// Bytes renders the tree or subtree as byteslice.
	Bytes() []byte

	SetValue(value Value)
	// SetMetaValue(meta MetaValue)

	// VisitAll iterates over the tree, branches and nodes.
	// If need to iterate over the whole tree, use the root Node.
	// Note this method uses a breadth-first approach.
	VisitAll(fn NodeVisitor)
}
type FileInfo interface {
	os.FileInfo
	Inode() uint64
	Ctime() time.Time
	Atime() time.Time
	Nlink() int
	IsFile() bool
	Uid() uint32
	Gid() uint32
	Rdev() uint32
}

type Node interface {
	FileInfo
	SetSize(size int64) error
	SetModTime(modTime time.Time) error
	SetATime(atime time.Time) error
	SetCTime(ctime time.Time) error
	SetUID(uid uint32) error
	SetGID(gid uint32) error
	// SetMode set the golang's FileMode, the mode should contain the file's type and file perm
	SetMode(mode os.FileMode) error
	SetNlink(nlink int) error
	Sync() error
	Remove(ctx context.Context) error
	RemoveAll(ctx context.Context) error
	Flush(ctx context.Context) error
	Truncate(size int64) error
	Path() string
	SetSys(any)
	GetParent() Node
	Get(ino uint64) Node
	Reload(ctx context.Context) error

	VFS() *VFS
	Open(flags int) (Handle, error)
	Tree

	ID() string
}

// Nodes is a slice of Node
type Nodes []Node

// Sort functions
func (ns Nodes) Len() int           { return len(ns) }
func (ns Nodes) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns Nodes) Less(i, j int) bool { return ns[i].Inode() < ns[j].Inode() }

type attrSetKey struct{}

var (
	_attrSetKey = attrSetKey{}
)

func NewAttrContext(ctx context.Context, set int) context.Context {
	return context.WithValue(ctx, _attrSetKey, set)
}

func FromAttrContext(ctx context.Context) (set int, ok bool) {
	set, ok = ctx.Value(_attrSetKey).(int)
	return
}
