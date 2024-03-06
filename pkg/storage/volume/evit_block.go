package volume

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/store"
	"github.com/pingcap/log"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// EvitManager used to manage evit of block from local to remote storage
type EvitManager struct {
	sync.RWMutex
	volID         PhysicalVolumeID
	pendingBlocks []lo.Tuple2[int, string] // FIXME: better structure?

	store  store.Store
	ctx    context.Context
	cancel context.CancelFunc

	slot *Slot
	eng  engine.Engine
}

func NewEvitManager(ctx context.Context, volID PhysicalVolumeID, store store.Store, slot *Slot, eng engine.Engine) *EvitManager {
	ctx, cancel := context.WithCancel(ctx)
	em := &EvitManager{
		pendingBlocks: make([]lo.Tuple2[int, string], 0),
		store:         store,
		volID:         volID,
		cancel:        cancel,
		ctx:           ctx,
		slot:          slot,
		eng:           eng,
	}

	data, err := store.Get(ctx, em.Format())
	if err == nil && data != nil {
		_ = json.Unmarshal(data, &em.pendingBlocks)
	}

	go em.Loop()
	return em
}

func (e *EvitManager) AddEvitBlock(block lo.Tuple2[int, string]) {
	e.Lock()
	defer e.Unlock()
	e.pendingBlocks = append(e.pendingBlocks, block)
}

func (e *EvitManager) Loop() {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-timer.C:
			e.Lock()
			if len(e.pendingBlocks) == 0 {
				e.Unlock()
				continue
			}
			block_id := e.pendingBlocks[0].A
			log.Debug("sync block into s3", zap.Int("block_id", block_id))
			e.pendingBlocks = e.pendingBlocks[1:]
			e.slot.Free(uint64(block_id))
			e.Unlock()
		}
	}
}

func (e *EvitManager) Format() string {
	return fmt.Sprintf("evit_block/%d", e.volID)
}

func (e *EvitManager) Shutdown() error {
	e.cancel()

	blocks := e.GetPendingBlocks()
	log.Debug("evit manager shutdown, persisting pending blocks")
	for _, block := range blocks {
		log.Debug("pending block", zap.Int("block_id", block.A), zap.String("block_hash", block.B))
	}
	j, err := json.Marshal(blocks)
	if err != nil {
		return err
	}
	return e.store.Save(context.TODO(), e.Format(), j)
}

func (e *EvitManager) GetPendingBlocks() []lo.Tuple2[int, string] {
	e.RLock()
	defer e.RUnlock()
	return e.pendingBlocks
}

// Expire2Local move the cache entry out
func (e *EvitManager) Expire2Local(blockID uint64) {
	e.Lock()
	defer e.Unlock()
	var idx = -1
	for i, tuple := range e.pendingBlocks {
		if uint64(tuple.A) == blockID {
			idx = i
			break
		}
	}
	if idx != -1 {
		e.pendingBlocks = append(e.pendingBlocks[:idx], e.pendingBlocks[idx+1:]...)
	}
}

func (e *EvitManager) Expire2Remote(ctx context.Context, blockID uint64) bool {
	e.Lock()

	var idx = -1
	var blockEntry lo.Tuple2[int, string]
	for i, block := range e.pendingBlocks {
		if uint64(block.A) == e.volID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false
	}
	e.pendingBlocks = append(e.pendingBlocks[:idx], e.pendingBlocks[idx+1:]...)
	e.Unlock()

	log.Debug("sync block into s3", zap.Uint64("block_id", blockID), zap.String("block_hash", blockEntry.B))

	// TODO: read from engine
	// e.eng.GetBlockData(...)
	// sync to s3 ...
	if err := e.eng.DeleteBlock(ctx, engine.NewLocation(e.volID, blockID)); err != nil {
		log.Error("delete block error", zap.Uint64("block_id", blockID))
		return false
	}
	e.slot.Free(blockID)
	return true
}

func (e *EvitManager) BlockExpired(blockID uint64) bool {
	e.RLock()
	defer e.RUnlock()
	var idx = -1
	for i, tuple := range e.pendingBlocks {
		if uint64(tuple.A) == blockID {
			idx = i
			break
		}
	}
	return idx == -1
}
