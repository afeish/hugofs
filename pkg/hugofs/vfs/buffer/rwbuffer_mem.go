package buffer

import (
	"io"
	"sync"
	"time"

	"github.com/samber/lo"
	"go.uber.org/zap"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/buffer/block"
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
)

var (
	_ meta.RWBuffer = (*DirtyRWBuffer)(nil)

	_ io.ReaderAt = (*DirtyRWBuffer)(nil)
	_ io.WriterAt = (*DirtyRWBuffer)(nil)
	_ io.WriterTo = (*DirtyRWBuffer)(nil)
)

// DirtyRWBuffer just writes data into the buffer, do nothing about the underlying storage backend.
type DirtyRWBuffer struct {
	mu           sync.RWMutex
	ino          Ino
	blockSize    int64                                // blockSize may be changed
	blockMap     map[BlockIndexInFile]*block.MemBlock // it is dirty
	totalSize    int
	lastAccessNs int64
}

func NewDirtyRWBuffer(ino Ino, blockSize int) *DirtyRWBuffer {
	if blockSize <= 0 {
		blockSize = int(DefaultBlockSize)
	}

	return &DirtyRWBuffer{
		ino:       ino,
		blockSize: int64(blockSize),
		blockMap:  make(map[BlockIndexInFile]*block.MemBlock),
	}
}
func (b *DirtyRWBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	return b.Read(p, off)
}

func (b *DirtyRWBuffer) Read(p []byte, off int64) (count int, err error) {
	if cap(p) == 0 {
		return 0, io.EOF
	}

	b.lastAccessNs = time.Now().UnixNano()

	// Determine the range of blocks to read.
	startBlockIdx := off / b.blockSize
	endBlockIdx := (off + int64(len(p))) / b.blockSize
	if (off+int64(len(p)))%b.blockSize == 0 {
		endBlockIdx--
	}
	if endBlockIdx < 0 {
		return 0, io.EOF
	}

	// Iterate through each block in the range and read its contents.
	var n int64
	blockOff := off % b.blockSize
	for blockIdx := startBlockIdx; blockIdx <= endBlockIdx; blockIdx++ {
		blockEntry, ok := b.blockMap[uint64(blockIdx)]
		if !ok {
			break
		}
		data := blockEntry.GetData()
		dataLen := int64(len(data))
		spaceLeft := int64(len(p)) - n
		if blockOff > dataLen {
			return int(n), io.EOF
		}
		if spaceLeft > (int64(len(data)) - blockOff) { // enough remain space left on dst buffer, read all the remain data
			copy(p[n:], data[blockOff:])
			n += (int64(len(data)) - blockOff)
		} else { // no enough space left on dst buffer, fill up the rest of dst buffer
			copy(p[n:], data[blockOff:blockOff+spaceLeft])
			n += spaceLeft
		}
		blockOff = 0
	}

	return int(n), nil
}

func (b *DirtyRWBuffer) WriteAt(p []byte, off int64) (n int, err error) {
	return b.Write(p, off)
}
func (b *DirtyRWBuffer) Write(p []byte, off int64) (writeCnt int, err error) {
	blockIdx := BlockIndexInFile(off / b.blockSize)
	blockOff := off % b.blockSize

	b.lastAccessNs = time.Now().UnixNano()
	for len(p) > 0 {
		// Determine how much data we can write to the current block
		spaceLeft := b.blockSize - blockOff // how many bytes can be written to the current block
		if int64(len(p)) < spaceLeft {      // we can write all the data to the current block
			spaceLeft = int64(len(p))
		}

		entry, ok := b.blockMap[blockIdx]
		// Check if we need to allocate a new block for writing
		if !ok {
			// 1. full block size
			// 2. data length which is less than block size
			entry = block.NewMemBlockForWrite(adaptor.NewMemStorageBackend(b.blockSize), adaptor.NewBlockKey(b.ino, blockIdx), b.blockSize, zap.NewNop())
		} else {
			// find previous partial block, enlarge it
			newBlockData := make([]byte, blockOff+spaceLeft)
			copy(newBlockData[0:blockOff], entry.GetData())
			entry.WriteAt(newBlockData, 0)
		}

		// Clone the new data into the block
		copy(entry.GetData()[blockOff:blockOff+spaceLeft], p[:spaceLeft])
		b.blockMap[blockIdx] = entry

		// Update the counters and slice for the next iteration
		writeCnt += int(spaceLeft)
		p = p[spaceLeft:]
		blockIdx++
		blockOff = 0
	}

	b.totalSize += writeCnt
	return writeCnt, nil
}
func (b *DirtyRWBuffer) Size() (int, error) {
	return b.totalSize, nil
}

func (b *DirtyRWBuffer) ReadFrom(r io.ReaderAt) (n int, err error) {
	var written int
	for {
		reader := io.NewSectionReader(r, int64(written), int64(b.blockSize))
		writer := io.NewOffsetWriter(b, int64(written))
		n, err := io.Copy(writer, reader)
		if err != nil {
			if err != io.EOF {
				break
			}
			return int(written), err
		}
		if n == 0 {
			break
		}
		written += int(n)
	}
	return written, nil
}

func (b *DirtyRWBuffer) WriteTo(w io.Writer) (n int64, err error) {
	totalWritten := int64(0)
	indexes := SortR(lo.Keys(b.blockMap), func(a, b BlockIndexInFile) int {
		return int(a - b)
	})

	for _, index := range indexes {
		n, err := w.Write(b.blockMap[index].GetData())
		if err != nil && err != io.EOF {
			return 0, err
		}
		totalWritten += int64(n)
	}

	return totalWritten, nil
}

func (b *DirtyRWBuffer) GetBlockMap() (r map[BlockIndexInFile]meta.Block, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return lo.MapEntries(b.blockMap, func(key BlockIndexInFile, value *block.MemBlock) (BlockIndexInFile, meta.Block) {
		return key, value
	}), nil
}

func (b *DirtyRWBuffer) Flush() (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, entry := range b.blockMap {
		err = entry.Flush()
		if err != nil {
			return
		}
	}
	return nil
}

func (b *DirtyRWBuffer) Idle(d time.Duration) bool {
	return time.Since(time.Unix(0, b.lastAccessNs)) > d
}

func (b *DirtyRWBuffer) Destroy() {
}
