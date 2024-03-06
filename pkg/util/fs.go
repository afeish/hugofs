package util

import (
	"os"
	"syscall"
)

func MakeMode(perm uint32, ft os.FileMode) os.FileMode {
	var umask uint32 = 022
	return os.FileMode(perm&^umask) | ft
}

// ToSyscallMode is to transist the golang's file mode to kernel's file mode
func ToSyscallMode(mode os.FileMode) uint32 {
	r := mode.Perm()
	switch {
	default:
		r |= syscall.S_IFREG
	case mode&os.ModeDir != 0:
		r |= syscall.S_IFDIR
	case mode&os.ModeDevice != 0:
		if mode&os.ModeCharDevice != 0 {
			r |= syscall.S_IFCHR
		} else {
			r |= syscall.S_IFBLK
		}
	case mode&os.ModeNamedPipe != 0:
		r |= syscall.S_IFIFO
	case mode&os.ModeSymlink != 0:
		r |= syscall.S_IFLNK
	case mode&os.ModeSocket != 0:
		r |= syscall.S_IFSOCK
	}
	if mode&os.ModeSetuid != 0 {
		r |= syscall.S_ISUID
	}
	if mode&os.ModeSetgid != 0 {
		r |= syscall.S_ISGID
	}
	if mode&os.ModeSticky != 0 {
		r |= syscall.S_ISVTX
	}
	return uint32(r)
}

func ToSyscallType(mode os.FileMode) uint32 {
	switch mode & os.ModeType {
	case os.ModeDir:
		return syscall.S_IFDIR
	case os.ModeSymlink:
		return syscall.S_IFLNK
	case os.ModeNamedPipe:
		return syscall.S_IFIFO
	case os.ModeSocket:
		return syscall.S_IFSOCK
	case os.ModeDevice:
		return syscall.S_IFBLK
	case os.ModeCharDevice:
		return syscall.S_IFCHR
	default:
		return syscall.S_IFREG
	}
}

func ToPerm(mode uint32) uint32 {
	return mode &^ uint32(syscall.S_IFMT)
}

/*
Refer from `man 7 inode`

POSIX  refers  to  the stat.st_mode bits corresponding to the mask S_IFMT (see below) as the file type, the 12 bits corresponding to the mask 07777 as the file mode bits and the least significant 9 bits
(0777) as the file permission bits.

S_IFMT     0170000   bit mask for the file type bit field

S_IFSOCK   0140000   socket
S_IFLNK    0120000   symbolic link
S_IFREG    0100000   regular file
S_IFBLK    0060000   block device
S_IFDIR    0040000   directory
S_IFCHR    0020000   character device
S_IFIFO    0010000   FIFO

S_ISUID     04000   set-user-ID bit (see execve(2))
S_ISGID     02000   set-group-ID bit (see below)
S_ISVTX     01000   sticky bit (see below)

S_IRWXU     00700   owner has read, write, and execute permission
S_IRUSR     00400   owner has read permission
S_IWUSR     00200   owner has write permission
S_IXUSR     00100   owner has execute permission

S_IRWXG     00070   group has read, write, and execute permission
S_IRGRP     00040   group has read permission
S_IWGRP     00020   group has write permission
S_IXGRP     00010   group has execute permission

S_IRWXO     00007   others (not in group) have read, write, and  execute permission

S_IROTH     00004   others have read permission
S_IWOTH     00002   others have write permission
S_IXOTH     00001   others have execute permission
*/

// ToFileMode returns a Go os.FileMode from a Unix mode.
func ToFileMode(unixMode uint32) os.FileMode {
	mode := os.FileMode(unixMode & 0o777)
	switch unixMode & syscall.S_IFMT {
	case syscall.S_IFREG:
		// nothing
	case syscall.S_IFDIR:
		mode |= os.ModeDir
	case syscall.S_IFCHR:
		mode |= os.ModeCharDevice | os.ModeDevice
	case syscall.S_IFBLK:
		mode |= os.ModeDevice
	case syscall.S_IFIFO:
		mode |= os.ModeNamedPipe
	case syscall.S_IFLNK:
		mode |= os.ModeSymlink
	case syscall.S_IFSOCK:
		mode |= os.ModeSocket
	case 0:
		// apparently there's plenty of times when the FUSE request
		// does not contain the file type
		mode |= os.ModeIrregular
	default:
		// not just unavailable in the kernel codepath; known to
		// kernel but unrecognized by us
		mode |= os.ModeIrregular
	}
	if unixMode&syscall.S_ISUID != 0 {
		mode |= os.ModeSetuid
	}
	if unixMode&syscall.S_ISGID != 0 {
		mode |= os.ModeSetgid
	}
	if unixMode&syscall.S_ISVTX != 0 {
		mode |= os.ModeSticky
	}
	return mode
}
