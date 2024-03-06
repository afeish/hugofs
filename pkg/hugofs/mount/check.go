package mount

import (
	"syscall"

	"github.com/afeish/hugo/pb/meta"
)

func checkLength(name string) syscall.Errno {
	if len(name) >= 256 { // /usr/include/linux/limits.h
		return syscall.ENAMETOOLONG
	}
	if name == "" {
		return syscall.ENOTSUP
	}
	return syscall.F_OK
}

func checkInoName(ino uint64, name string) syscall.Errno {
	if len(name) >= 256 {
		return syscall.ENAMETOOLONG
	}
	if name == "" {
		return syscall.ENOTSUP
	}
	if ino == rootID && IsSpecialName(name) {
		return syscall.EPERM
	}
	return syscall.F_OK
}

func IsSpecialNode(ino uint64) bool {
	return ino >= minInternalNode
}

const (
	rootID          = 1
	minInternalNode = 0x7FFFFFFF00000000
	logInode        = minInternalNode + 1
	controlInode    = minInternalNode + 2
	statsInode      = minInternalNode + 3
	configInode     = minInternalNode + 4
	trashInode      = 0x7FFFFFFF10000000
)

func IsSpecialName(name string) bool {
	if name[0] != '.' {
		return false
	}
	for _, n := range internalNodes {
		if name == n.name {
			return true
		}
	}
	return false
}

type internalNode struct {
	inode uint64
	name  string
	attr  *meta.Attr
}

var internalNodes = []*internalNode{
	{controlInode, ".control", &meta.Attr{Inode: controlInode, Mode: 0666}},
	{logInode, ".accesslog", &meta.Attr{Inode: logInode, Mode: 0400}},
	{statsInode, ".stats", &meta.Attr{Inode: statsInode, Mode: 0444}},
	{configInode, ".config", &meta.Attr{Inode: configInode, Mode: 0400}},
	{trashInode, ".trash", &meta.Attr{Inode: trashInode, Mode: 0555}},
}
