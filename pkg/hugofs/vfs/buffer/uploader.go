package buffer

import (
	"os"
	"sync"
	"sync/atomic"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/buffer/block"
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/util/executor"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

var (
	memBlockCounter int64
)

type UploadPipeline struct {
	ino       Ino
	blockLock sync.Mutex
	storage   adaptor.StorageBackend
	monitor   PatternMonitor
	uploaders *executor.LimitedConcurrentExecutor
	lg        *zap.Logger

	uploaderCount    int32
	writeBlocks      map[int64]meta.Block
	sealedBlocks     map[int64]*SealedBlock
	retiredBlocks    map[int64]*SealedBlock
	activeReadBlocks map[int64]int

	uploaderCountCond *sync.Cond
	readerCountCond   *sync.Cond

	swapFile *block.SwapFile
	opts     meta.Options
}

type SealedBlock struct {
	block    meta.Block
	refCount int32
}

func (s *SealedBlock) Release() {
	atomic.AddInt32(&s.refCount, -1)
	if atomic.LoadInt32(&s.refCount) == 0 {
		s.block.Destroy()
	}
}

func NewUploadPipeline(ino uint64, storage adaptor.StorageBackend, monitor PatternMonitor, lg *zap.Logger, opts ...Option[*meta.Options]) *UploadPipeline {
	envCfg := GetEnvCfg()
	dir := envCfg.Cache.Dir
	os.MkdirAll(dir, os.ModePerm&^022)

	up := &UploadPipeline{
		ino:               ino,
		storage:           storage,
		writeBlocks:       make(map[int64]meta.Block),
		sealedBlocks:      make(map[int64]*SealedBlock),
		retiredBlocks:     make(map[int64]*SealedBlock),
		activeReadBlocks:  make(map[int64]int),
		uploaderCountCond: sync.NewCond(&sync.Mutex{}),
		monitor:           monitor,
		lg:                lg,
		opts:              meta.DefaultOptions,
	}

	ApplyOptions(&up.opts, opts...)
	if up.opts.BlockSize < up.opts.PageSize {
		up.opts.PageSize = up.opts.BlockSize
	}
	up.uploaders = executor.NewLimitedConcurrentExecutor(up.opts.Limit)
	up.readerCountCond = sync.NewCond(&up.blockLock)
	up.swapFile = block.NewSwapFile(ino, storage, lg, opts...)

	return up
}

func (up *UploadPipeline) WriteAt(p []byte, off int64) (writeCnt int, err error) {
	up.blockLock.Lock()
	defer up.blockLock.Unlock()

	blockSize := up.opts.BlockSize
	blockIdx := int64(off) / blockSize

	n := 0

	for len(p) > 0 {
		key := meta.NewBlockKey(up.ino, uint64(blockIdx))
		_block, found := up.writeBlocks[blockIdx]
		if !found {
			_block = up.swapFile.GetBlock(key)
			if _block == nil {
				if len(up.writeBlocks) > up.opts.Limit {
					candidateBlockIdx, largestWritten := int64(-1), int64(0)
					for lci, mc := range up.writeBlocks {
						written := mc.Written()
						if largestWritten < written {
							candidateBlockIdx = lci
							largestWritten = written
						}
					}
					up.lg.Info("write block limit reached, moving to sealed block", zap.Int("writeBlocksCounter", len(up.writeBlocks)), zap.String("largest-written", humanize.IBytes(uint64(largestWritten))))
					up.move2Sealed(up.writeBlocks[candidateBlockIdx], candidateBlockIdx)
				}

				counter := atomic.LoadInt64(&memBlockCounter)

				key := meta.NewBlockKey(up.ino, uint64(blockIdx))
				if len(up.writeBlocks) < up.opts.Limit &&
					counter < 4*int64(up.opts.Limit) {
					_block = block.NewMemBlockForWrite(up.storage, key, blockSize, up.lg)
					up.writeBlocks[blockIdx] = _block
				} else {
					_block, err = up.swapFile.NewFileBlock(key)
					if err != nil {
						return 0, err
					}
				}

			}
		}

		if _block.IsDestroyed() {
			delete(up.writeBlocks, blockIdx)
			continue
		}

		n, err = _block.WriteAt(p, int64(off))
		if err != nil {
			return
		}

		if _block.IsFull() {
			up.move2Sealed(_block, blockIdx)
		}

		writeCnt += n
		off += int64(n)
		blockIdx++
		p = p[n:]
	}
	return

}

func (up *UploadPipeline) ReadAt(p []byte, off int64) (n int, err error) {
	blockIdx := int64(off) / up.opts.BlockSize

	key := meta.NewBlockKey(up.ino, uint64(blockIdx))

	up.blockLock.Lock()
	defer func() {
		// up.readerCountCond.Signal()
		up.blockLock.Unlock()
	}()

	retiringBlock, found := up.retiredBlocks[blockIdx]
	if found {
		n, err = retiringBlock.block.ReadAt(p, int64(off))
		up.lg.Sugar().Infof("%s read retiring block [%d,%d)", key.String(), off, off+int64(n))
		if err != nil {
			return
		}
	}

	sealedBlock, found := up.sealedBlocks[blockIdx]
	if found {
		n, err = sealedBlock.block.ReadAt(p, int64(off))
		up.lg.Sugar().Infof("%s read sealed block [%d,%d)", key.String(), off, off+int64(n))
		if err != nil {
			return
		}
	}

	writeBlock, found := up.writeBlocks[blockIdx]
	if !found {
		writeBlock = up.swapFile.GetBlock(key)
		if writeBlock == nil {
			return
		}
	}
	n, err = writeBlock.ReadAt(p, int64(off))
	if n > 0 {
		up.lg.Sugar().Infof("%s read sealed block [%d,%d)", key.String(), off, off+int64(n))
	}
	return
}

func (up *UploadPipeline) move2Sealed(b meta.Block, idx int64) {
	atomic.AddInt32(&up.uploaderCount, 1)
	count := atomic.LoadInt32(&up.uploaderCount)
	up.lg.Sugar().Debugf("%d uploaderCount %d ++> %d", up.ino, count-1, count)

	if oldBlock, found := up.sealedBlocks[idx]; found {
		oldBlock.Release()
	}

	sealedBlock := &SealedBlock{
		block:    b,
		refCount: 1,
	}
	up.sealedBlocks[idx] = sealedBlock
	delete(up.writeBlocks, idx)

	up.blockLock.Unlock()
	up.uploaders.Execute(func() {
		sealedBlock.block.Flush()
		atomic.AddInt32(&up.uploaderCount, -1)
		count := atomic.LoadInt32(&up.uploaderCount)
		up.lg.Sugar().Debugf("%d uploaderCount %d --> %d", up.ino, count+1, count)
		up.uploaderCountCond.L.Lock()
		up.uploaderCountCond.Broadcast()
		up.uploaderCountCond.L.Unlock()

		up.blockLock.Lock()
		defer up.blockLock.Unlock()
		for up.IsLocked(idx) {
			up.readerCountCond.Wait()
		}

		delete(up.sealedBlocks, idx)
		sealedBlock.Release()

		// shrinkRetiredBlocks := func() {
		// 	for idx, block := range up.retiredBlocks {
		// 		delete(up.retiredBlocks, idx)
		// 		block.Release()
		// 	}
		// }

		// if len(up.writeBlocks)+len(up.sealedBlocks)+len(up.retiredBlocks) <= up.writeBlockLimit {
		// 	up.retiredBlocks[idx] = sealedBlock
		// } else {
		// 	sealedBlock.Release()
		// 	shrinkRetiredBlocks()
		// }

	})
	up.blockLock.Lock()
}

func (up *UploadPipeline) LockForRead(start, end int64) {
	blockSize := up.opts.BlockSize
	startIdx := int64(start) / blockSize
	endIdx := int64(end) / blockSize
	if end%blockSize > 0 {
		endIdx += 1
	}
	up.blockLock.Lock()
	defer up.blockLock.Unlock()

	for i := startIdx; i < endIdx; i++ {
		if count, found := up.activeReadBlocks[i]; found {
			up.activeReadBlocks[i] = count + 1
		} else {
			up.activeReadBlocks[i] = 1
		}
	}
}

func (up *UploadPipeline) UnlockForRead(start, end int64) {
	blockSize := up.opts.BlockSize
	startIdx := int64(start) / blockSize
	endIdx := int64(end) / blockSize
	if end%blockSize > 0 {
		endIdx += 1
	}
	up.blockLock.Lock()
	defer up.blockLock.Unlock()

	for i := startIdx; i < endIdx; i++ {
		if count, found := up.activeReadBlocks[i]; found {
			if count == 1 {
				delete(up.activeReadBlocks, i)
				up.readerCountCond.Signal()
			} else {
				up.activeReadBlocks[i] = count - 1
			}
		}
	}
}

func (up *UploadPipeline) IsLocked(idx int64) bool {
	if count, found := up.activeReadBlocks[idx]; found {
		return count > 0
	}
	return false
}

func (up *UploadPipeline) waitForCurrentWritersToComplete() {
	up.uploaderCountCond.L.Lock()
	t := int32(100)
	for {
		t = atomic.LoadInt32(&up.uploaderCount)
		if t <= 0 {
			break
		}
		up.uploaderCountCond.Wait()
	}
	up.uploaderCountCond.L.Unlock()
}

func (up *UploadPipeline) FlushAll() {
	up.blockLock.Lock()
	defer up.blockLock.Unlock()

	for idx, block := range up.writeBlocks {
		up.move2Sealed(block, idx)
	}

	up.waitForCurrentWritersToComplete()
}

func (up *UploadPipeline) Destory() {
	up.lg.Debug("destroy upload pipeline", zap.Uint64("ino", up.ino))

	up.blockLock.Lock()
	defer up.blockLock.Unlock()
	for idx, block := range up.retiredBlocks {
		delete(up.retiredBlocks, idx)
		block.Release()
	}
	for idx, block := range up.sealedBlocks {
		delete(up.sealedBlocks, idx)
		block.Release()
	}

	up.swapFile.FreeResource()
}
