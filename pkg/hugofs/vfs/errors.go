package vfs

import (
	"fmt"
	"os"
)

// Error describes low level errors in a cross platform way.
type Error byte

// Low level errors
const (
	OK Error = iota
	ENOTEMPTY
	ENOTDIR
	ESPIPE
	EBADF
	EROFS
	ENOSYS
)

// Errors which have exact counterparts in os
var (
	ENOENT  = os.ErrNotExist
	EEXIST  = os.ErrExist
	EPERM   = os.ErrPermission
	EINVAL  = os.ErrInvalid
	ECLOSED = os.ErrClosed
)

var errorNames = []string{
	OK:        "Success",
	ENOTEMPTY: "Directory not empty",
	ENOTDIR:   "Not a directory",
	ESPIPE:    "Illegal seek",
	EBADF:     "Bad file descriptor",
	EROFS:     "Read only file system",
	ENOSYS:    "Function not implemented",
}

// Error renders the error as a string
func (e Error) Error() string {
	if int(e) >= len(errorNames) {
		return fmt.Sprintf("Low level error %d", e)
	}
	return errorNames[e]
}
