package meta

import (
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
)

// RWBuffer represents a read-write buffer for a file.
// It consists of a map of blocks and each block has a fixed size.
type RWBuffer interface {
	Read(p []byte, off int64) (count int, err error)
	Write(p []byte, off int64) (writeCnt int, err error)
	Size() (int, error)
	Flush() error
	Idle(d time.Duration) bool
	Destroy()
	GetBlockMap() (map[BlockIndexInFile]Block, error)
}
