package volume

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/block"
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/dustin/go-humanize"
	"github.com/pingcap/log"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	// "github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"go.uber.org/zap"
)

var _ PhysicalVolume = (*LocalPhysicalVolume)(nil)

// LocalPhysicalVolume maintains a directory which holds
// all the data blocks of the volume.
type LocalPhysicalVolume struct {
	mu       sync.RWMutex
	id       PhysicalVolumeID // immutable
	nodeId   StorageNodeID    // immutable
	stat     *VolumeStatistics
	location string
	slot     *Slot
	engine   engine.Engine // support component
	store    store.Store

	blockLru    *BlockLRU
	evitManager *EvitManager

	messenger *Messenger
	closech   ClosableCh

	ctx    context.Context
	cancel context.CancelFunc
	lg     *zap.Logger
}

func NewLocalPhysicalVolume(ctx context.Context, cfg *Config, messenger *Messenger, lg *zap.Logger) (*LocalPhysicalVolume, error) {
	ctx, cancel := context.WithCancel(ctx)
	lg = lg.Named("volume").With(zap.Uint64(VOL_ID, cfg.Id))

	eng, err := engine.NewProvider(ctx, engine.ProviderInfo{
		Type:   cfg.Engine,
		Config: &engine.Config{Prefix: cfg.Prefix, VolID: cfg.Id},
	})
	if err != nil {
		cancel()
		return nil, err
	}
	lg.Debug("open db", zap.String("name", cfg.DB.Name), zap.String("arg", cfg.DB.Arg))
	s, err := store.GetStore(cfg.DB.Name, mo.Some(cfg.DB.Arg))
	if err != nil {
		cancel()
		return nil, err
	}
	threhold := DefaultThrehold
	if cfg != nil && cfg.Threhold != nil {
		threhold = *cfg.Threhold
	}
	quota := PhysicalVolumeSize
	if cfg != nil && cfg.Quota != nil {
		quota = *cfg.Quota
	}
	blockSize := block.Size
	if cfg != nil && cfg.BlockSize != nil {
		blockSize = *cfg.BlockSize
	}
	slotCap := int(threhold * float64(quota) / float64(blockSize))
	slot := NewSlot(cfg.Id, uint64(slotCap), s)
	evitManager := NewEvitManager(ctx, cfg.Id, s, slot, eng)

	lg.Debug("slot cap", zap.String("cap", humanize.IBytes(uint64(threhold*float64(quota)))), zap.Any("block-size", blockSize), zap.Int("slot_cap", slotCap))

	lru, err := NewBlockLRU(slotCap, func(key int, value string) {
		lg.Debug("evit block", zap.Int(SLOT, key), zap.String(BLOCK_ID, fmt.Sprintf("%x", value)))
		evitManager.AddEvitBlock(lo.T2(key, value))
	})
	if err != nil {
		cancel()
		return nil, err
	}
	vol := &LocalPhysicalVolume{
		id:          cfg.Id,
		nodeId:      cfg.NodeId,
		location:    cfg.Prefix,
		slot:        slot,
		stat:        newVolumeStatistics(ctx, cfg.Id, quota, threhold, s, messenger.GetVolStatCh(), lg),
		engine:      eng,
		store:       s,
		blockLru:    lru,
		evitManager: evitManager,
		messenger:   messenger,
		ctx:         ctx,
		cancel:      cancel,
		closech:     NewClosableCh(),
		lg:          lg,
	}

	go vol.scheduleTask()

	return vol, nil
}

func (p *LocalPhysicalVolume) GetID(ctx context.Context) uint64 {
	return p.id
}
func (p *LocalPhysicalVolume) GetCap(ctx context.Context) (uint64, error) {
	return p.stat.GetCap(), nil
}
func (p *LocalPhysicalVolume) CalculateUsage(ctx context.Context) (uint64, error) {
	return p.stat.InuseBytes, nil
}
func (p *LocalPhysicalVolume) GetFreeBlockCnt(ctx context.Context) (uint64, error) {
	return p.slot.Remains(), nil
}
func (p *LocalPhysicalVolume) GetTotalBlockCnt(ctx context.Context) (uint64, error) {
	return p.slot.Size(), nil
}
func (p *LocalPhysicalVolume) ReadBlock(ctx context.Context, hash string) (data []byte, header *block.Header, err error) {
	ctx, span := tracing.Tracer.Start(ctx, "read block from vol", trace.WithAttributes(attribute.String("block-id", hash)))
	defer span.End()

	index, err := p.slot.GetSlot(hash)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "err read block")
	}

	p.lg.Debug("read block", zap.Uint64(SLOT, index), zap.String(BLOCK_ID, hash))
	if !p.slot.Occupied(index) {
		return nil, nil, fmt.Errorf("block %d is not occupied", index)
	}

	data, err = p.engine.GetBlockDataCb(ctx, p.getLocation(index), func(h *block.Header) error {
		header = h
		p.evitManager.Expire2Local(h.Index) // always check the pending expire entry first, if found any, exipre from the pending expire list
		p.blockLru.Put(int(h.Index), h.HashReadable())
		return nil
	})
	if err != nil {
		p.lg.Debug("not found block local, find it remote", zap.Uint64(BLOCK_ID, index))
		//TODO: download the neighboor blocks from s3
	}
	return

}
func (p *LocalPhysicalVolume) WriteBlock(ctx context.Context, container *engine.DataContainer) (blockID string, c chan struct{}, err error) {
	var (
		encodedHash = fmt.Sprintf("%x", container.Hash)
	)
	c = make(chan struct{})
	f := func() {
		p.messenger.SendBlockMeta(BlockMetaEvent{
			BlockID:    encodedHash,
			VolID:      p.GetID(ctx),
			Done:       c,
			LogicIndex: container.Index,
			CreatedAt:  time.Now(),
			Size:       uint64(len(container.Data)),
			Crc32:      container.Crc,
		})
	}

	ctx, span := tracing.Tracer.Start(ctx, "write block to vol", trace.WithAttributes(attribute.String("block-id", encodedHash)))
	defer span.End()

	slot, err := p.slot.GetSlot(encodedHash)
	if err == nil {
		f()
		return "", c, engine.ErrBlockExisted
	}
	// 1. select a free slot.
	slot, err = p.occupyNextFreeSlot(ctx, encodedHash)
	if err != nil {
		p.lg.Debug("block failed to occupy slot", zap.String(BLOCK_ID, encodedHash))
		return "", nil, err
	}
	p.lg.Debug("block occupy slot", zap.Uint64(SLOT, slot), zap.String(BLOCK_ID, encodedHash))

	defer func() {
		if err != nil {
			p.lg.Error("write block failed", zap.Uint64(SLOT, slot), zap.Error(err))
			if _, err2 := p.slot.Free(slot); err2 != nil && err2 != ErrSlotEmpty {
				err = err2
			}
			close(c) // release the channel
			return
		}
		f()
	}()

	// 2. write data to the slot.
	err = p.engine.WriteBlock(ctx, engine.NewLocation(p.id, slot), container)
	if err != nil {
		return "", c, err
	}
	p.evitManager.Expire2Local(slot) // always check the pending expire entry first, if found any, exipre from the pending expire list
	p.blockLru.Put(int(slot), encodedHash)
	p.stat.AddInUseBytes(len(container.Data))

	p.lg.Debug("write block succed", zap.Uint64(SLOT, slot), zap.String(BLOCK_ID, encodedHash))
	return encodedHash, c, nil
}

func (p *LocalPhysicalVolume) RemoveLocal(ctx context.Context, hash string) error {
	slot, err := p.slot.GetSlot(hash)
	if err == nil {
		p.lg.Debug("free slot", zap.Uint64(SLOT, slot), zap.String(BLOCK_ID, hash))
		p.slot.Free(slot)
	}

	err = p.engine.DeleteBlock(ctx, &engine.Location{
		VolID: p.id,
		Slot:  slot,
	})
	return err
}

func (p *LocalPhysicalVolume) CancelBlock(ctx context.Context, index BlockIndexInPV) (uint64, error) {
	if err := p.engine.CancelBlock(ctx, p.getLocation(index)); err != nil {
		return 0, err
	}
	panic("implement me")
}

func (p *LocalPhysicalVolume) getLocation(index BlockIndexInPV) *engine.Location {
	return engine.NewLocation(p.id, index)
}
func (p *LocalPhysicalVolume) occupyNextFreeSlot(ctx context.Context, hash string) (uint64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	slot, err := p.nextFreeSlot(ctx)
	if err != nil {
		return 0, err
	}
	if err = p.slot.Occupy(ctx, slot, hash); err != nil {
		p.lg.Error(" failed to occupy slot", zap.Uint64(SLOT, slot))
		return 0, err
	}
	return slot, nil
}
func (p *LocalPhysicalVolume) nextFreeSlot(ctx context.Context) (uint64, error) {
	for p.slot.Remains() == 0 { // no enough slot left
		slot, blockID, ok := p.blockLru.RemoveOldest()
		if ok {
			p.lg.Debug("no enough slot, removing slot", zap.Int(SLOT, slot), zap.String(BLOCK_ID, blockID))
			p.evitManager.Expire2Remote(ctx, uint64(slot))
			break
		}
		time.Sleep(time.Second) // TODO: make configurable
	}
	return p.slot.Next(), nil
}
func (p *LocalPhysicalVolume) CleanInvalidBlock(ctx context.Context, invalidBlocks ...string) (bool, error) {
	for _, hash := range invalidBlocks {
		slot, err := p.slot.GetSlot(hash)
		if err != nil {
			p.lg.Error("slot not found", zap.String(BLOCK_ID, hash))
		}
		//FIXME: make sure that slot occupied and block at storage in sync
		if err := p.engine.DeleteBlock(ctx, p.getLocation(slot)); err != nil { // delete block locally
			p.lg.Error("delete block err", zap.Uint64(SLOT, slot))
		}
		p.messenger.SendBlockMeta(BlockMetaEvent{
			BlockID:   hash,
			VolID:     p.GetID(ctx),
			EventType: BlockMetaEvent_Delete,
		})
		_, _ = p.slot.Free(slot) // finally, delete blockID from mem
	}
	return true, nil
}
func (p *LocalPhysicalVolume) scheduleTask() {
	statInterval := time.Second * 5
	statTimer := time.NewTimer(statInterval)
	defer statTimer.Stop()

	quotaInterval := time.Minute
	quotaTimer := time.NewTimer(quotaInterval)
	defer quotaTimer.Stop()

	ctx := p.ctx
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-p.needGC():
			if v {
				p.triggerGC(ctx)
			}
		case <-statTimer.C:
			if err := p.syncState(ctx); err != nil {
				log.Error("sync state failed", zap.Error(err))
			}
			statTimer.Reset(statInterval)
		case <-quotaTimer.C:
			if err := p.checkQuota(ctx); err != nil {
				log.Error("sync quota failed", zap.Error(err))
			}
			quotaTimer.Reset(quotaInterval)
		case <-p.closech.ClosedCh():
			return
		}
	}
}
func (p *LocalPhysicalVolume) triggerGC(ctx context.Context) error {
	//p.storageMetaClient.GetPhysicalVolumeUsage(ctx)
	panic("implement me")
}
func (p *LocalPhysicalVolume) needGC() <-chan bool {
	//  TODO: implement me
	return make(<-chan bool)
}

func (p *LocalPhysicalVolume) syncState(ctx context.Context) error {
	if err := p.slot.PersistState(ctx); err != nil {
		return err
	}

	if err := p.stat.Persist(ctx); err != nil {
		return err
	}
	return nil
}

func (p *LocalPhysicalVolume) checkQuota(ctx context.Context) error {
	if p.stat.isAboveWatermark() {
		// compact the in-active blocks and send them to s3
	}
	return nil
}

func (p *LocalPhysicalVolume) Shutdown(ctx context.Context) (err error) {
	p.closech.Close(func() {
		// more lifecyle handling logic
		if err = p.syncState(ctx); err != nil {
			return
		}

		if err = p.evitManager.Shutdown(); err != nil {
			return
		}
		err = p.store.Close()
	})

	return nil
}

func (p *LocalPhysicalVolume) GetExpiredBlocks(ctx context.Context) ([]lo.Tuple2[int, string], error) {
	return p.evitManager.GetPendingBlocks(), nil
}

func (p *LocalPhysicalVolume) ExpireBlock(ctx context.Context, index BlockIndexInPV) error {
	p.evitManager.Expire2Local(index)
	return nil
}
