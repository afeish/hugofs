package node

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/engine"
	vol "github.com/afeish/hugo/pkg/storage/volume"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var (
	ErrSelectVolume = errors.New("failed to select volume")
)

type Selector struct {
	mu         sync.RWMutex
	candidates map[PhysicalVolumeID]vol.PhysicalVolume
	volStats   map[PhysicalVolumeID]vol.VolumeStatistics
	blockMetas map[string]*vol.BlockMetaEvent
	messenger  *vol.Messenger
	ctx        context.Context
	store      store.Store
	lg         *zap.Logger

	closech ClosableCh
}

func NewSelector(ctx context.Context, store store.Store, messenger *vol.Messenger, logger *zap.Logger) *Selector {
	s := &Selector{
		candidates: make(map[PhysicalVolumeID]vol.PhysicalVolume),
		volStats:   make(map[uint64]vol.VolumeStatistics),
		messenger:  messenger,
		ctx:        ctx,
		store:      store,
		closech:    NewClosableCh(),
		lg:         logger,
	}

	if err := s.loadBlockMeta(); err != nil {
		s.lg.Error("loading block meta", zap.Error(err))
	}

	go s.statLoop()
	return s
}

func (s *Selector) Sync(volId PhysicalVolumeID, volume vol.PhysicalVolume) {
	quota, _ := volume.GetCap(s.ctx)
	inuseBytes, _ := volume.CalculateUsage(s.ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.candidates[volId] = volume
	s.volStats[volId] = vol.VolumeStatistics{VolID: volId, Quota: size.SizeSuffix(quota), InuseBytes: inuseBytes} //anyway just initialize it with default empty values
}

// SelectWrite choose a physical volume to write block
func (s *Selector) SelectWrite() (vol.PhysicalVolume, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	volStats := lo.Values(s.volStats)
	sort.SliceStable(volStats, func(i, j int) bool {
		return volStats[i].RemainCap() > volStats[j].RemainCap()
	})
	return s.candidates[volStats[0].VolID], nil
}

// SelectRead choose a physical volume to read block
func (s *Selector) SelectRead(blockID BlockID) (vol.PhysicalVolume, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.blockMetas[blockID]
	if !ok {
		return nil, engine.ErrBlockNotExisted
	}
	return s.candidates[meta.VolID], nil
}

func (s *Selector) Select(volId PhysicalVolumeID) (vol.PhysicalVolume, error) {
	v, ok := s.candidates[volId]
	if !ok {
		return nil, ErrSelectVolume
	}
	return v, nil
}

func (s *Selector) Exists(volId PhysicalVolumeID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.candidates[volId] != nil
}

func (s *Selector) statLoop() error {
	statCh := s.messenger.GetVolStatCh()
	blockMetaCh := s.messenger.GetBlockMetaCh()
	for {
		select {
		case stat := <-statCh:
			// s.lg.Debug("gather stat", zap.Uint64(VOL_ID, stat.VolID), zap.String("inuse", humanize.IBytes(stat.InuseBytes)), zap.String("remain", humanize.IBytes(stat.RemainCap())))
			s.setVolStat(stat.VolID, stat)
		case meta := <-blockMetaCh:
			// s.lg.Debug("gather meta change", zap.Uint64(VOL_ID, meta.VolID))
			if err := s.handleBlockMeta(meta); err != nil {
				s.lg.Error("persist block meta", zap.Uint64(VOL_ID, meta.VolID), zap.String(vol.BLOCK_ID, meta.BlockID))
			}
		case <-s.ctx.Done():
			return nil
		case <-s.closech.ClosedCh():
			return nil
		}
	}
}

func (s *Selector) setVolStat(volId PhysicalVolumeID, volStat vol.VolumeStatistics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.volStats[volId] = volStat
}

func (s *Selector) GetVolStats() []vol.VolumeStatistics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.Values(s.volStats)
}

func (s *Selector) GetVolStat(volId PhysicalVolumeID) vol.VolumeStatistics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.volStats[volId]
}

func (s *Selector) loadBlockMeta() error {
	metaMap, err := store.DoGetValuesByPrefix[vol.BlockMetaEvent](s.ctx, s.store, &vol.BlockMetaEvent{})
	if err != nil {
		return err
	}
	s.blockMetas = lo.MapKeys(metaMap, func(value *vol.BlockMetaEvent, key string) string {
		return value.BlockID
	})
	return nil
}

func (s *Selector) handleBlockMeta(meta vol.BlockMetaEvent) error {
	switch meta.EventType {
	case vol.BlockMetaEvent_Delete:
		return s.deleteBlockMeta(meta)
	default:
		return s.addBlockMeta(meta)
	}
}

func (s *Selector) addBlockMeta(meta vol.BlockMetaEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.blockMetas[meta.BlockID] != nil && s.blockMetas[meta.BlockID].OnS3Only() {
		meta.State = storage.BlockState_LOCAL_S3
	}
	if err := store.DoSaveInKVPair[vol.BlockMetaEvent](s.ctx, s.store, &meta); err != nil {
		return err
	}
	s.blockMetas[meta.BlockID] = &meta
	close(meta.Done)
	return nil
}

func (s *Selector) deleteBlockMeta(meta vol.BlockMetaEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.blockMetas, meta.BlockID)
	if err := store.DoDeleteByKey[vol.BlockMetaEvent](s.ctx, s.store, &meta); err != nil {
		return err
	}
	return nil
}

func (s *Selector) GetBlockMeta(blockID BlockID) *vol.BlockMetaEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.blockMetas[blockID]
	if !ok {
		return nil
	}
	return meta
}

func (s *Selector) MarkBlockDestaged(blockID BlockID) error {
	return s._makeBlockState(blockID, storage.BlockState_LOCAL_S3)
}

func (s *Selector) MarkBlockFreeLocal(blockID BlockID) error {
	return s._makeBlockState(blockID, storage.BlockState_S3_ONLY)
}

func (s *Selector) _makeBlockState(blockID BlockID, state storage.BlockState) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.blockMetas[blockID]
	if !ok {
		return nil
	}
	meta.State = state
	if err := store.DoSaveInKVPair[vol.BlockMetaEvent](s.ctx, s.store, meta); err != nil {
		return err
	}
	return nil
}

func (s *Selector) Recycle() []*vol.BlockMetaEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	evts := make([]*vol.BlockMetaEvent, 0, len(s.blockMetas))
	for _, meta := range s.blockMetas {
		if meta.OnBoth() {
			evts = append(evts, meta)
		}
	}
	// if err := store.DoBatchDeleteStrictly[vol.BlockMetaEvent](s.ctx, s.store, lo.Map(evts, func(item *vol.BlockMetaEvent, index int) store.Serializer[vol.BlockMetaEvent] {
	// 	return item
	// })); err != nil {
	// 	return nil
	// }

	return evts
}

func (s *Selector) Warmup() []*vol.BlockMetaEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	evts := make([]*vol.BlockMetaEvent, 0, len(s.blockMetas))
	for _, meta := range s.blockMetas {
		if meta.OnS3Only() {
			evts = append(evts, meta)
		}
	}
	return evts
}

func (s *Selector) GetBlockMetas() []*vol.BlockMetaEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.Values(s.blockMetas)
}

func (s *Selector) Shutdown(ctx context.Context) error {
	s.closech.Close(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for volId, vol := range s.candidates {
			s.lg.Info("shutdown volume", zap.Uint64(VOL_ID, volId))
			if err := vol.Shutdown(ctx); err != nil {
				s.lg.Error("shutdown volume error", zap.Uint64(VOL_ID, volId), zap.Error(err))
			}
		}

	})
	return nil
}

func (s *Selector) ListStaleBlockMetas(expired int64) []*vol.BlockMetaEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	evts := make([]*vol.BlockMetaEvent, 0, len(s.blockMetas))
	for _, meta := range s.blockMetas {
		if meta.OnLocal() && time.Now().Add(-time.Duration(expired)*time.Second).After(meta.CreatedAt) {
			evts = append(evts, meta)
		}
	}
	return evts
}
