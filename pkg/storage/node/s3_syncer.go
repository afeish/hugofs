package node

import (
	"context"
	"encoding/hex"
	"hash/crc32"
	"sync"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/storage/oss"
	vol "github.com/afeish/hugo/pkg/storage/volume"
	"github.com/afeish/hugo/pkg/util/executor"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type ObjectSyncer struct {
	syncer oss.CloudKV

	mu         sync.RWMutex
	processing map[BlockID]struct{}
	executor   *executor.LimitedConcurrentExecutor

	selector *Selector

	lg *zap.Logger
}

func NewObjectSyncer(ctx context.Context, selector *Selector, cfg oss.Config, lg *zap.Logger) *ObjectSyncer {
	if err := cfg.Check(); err != nil {
		panic(err)
	}
	kv, err := oss.BuildCloudKV(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	return &ObjectSyncer{
		syncer:     kv,
		selector:   selector,
		processing: make(map[string]struct{}),
		executor:   executor.NewLimitedConcurrentExecutor(32),
		lg:         lg,
	}
}

func (s *ObjectSyncer) Sync(meta *vol.BlockMetaEvent) {
	s.mu.Lock()
	if _, ok := s.processing[meta.BlockID]; ok {
		s.mu.Unlock()
		return
	}
	s.processing[meta.BlockID] = struct{}{}
	s.mu.Unlock()

	s.executor.Execute(func() {
		blockId := meta.BlockID
		vol, err := s.selector.SelectRead(blockId)
		if err != nil {
			s.lg.Error("not found given block", zap.String(BLOCK_ID, blockId))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if _, err = s.syncer.Get(ctx, blockId); err == nil {
			s.selector.MarkBlockDestaged(blockId)
			return
		}

		data, _, err := vol.ReadBlock(ctx, blockId)
		if err != nil {
			s.lg.Error("not found given block", zap.Uint64(VOL_ID, vol.GetID(ctx)), zap.String(BLOCK_ID, blockId))
			return
		}
		err = s.syncer.Put(ctx, string(meta.BlockID), data)
		if err != nil {
			s.lg.Error("err put object to object storage", zap.Error(err))
			return
		}
		s.selector.MarkBlockDestaged(blockId)
	})

	s.mu.Lock()
	delete(s.processing, meta.BlockID)
	s.mu.Unlock()
}

func (s *ObjectSyncer) Restore(meta *vol.BlockMetaEvent) (theData []byte, theErr error) {
	blockId := meta.BlockID
	s.executor.Execute(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		data, err := s.syncer.Get(ctx, blockId)
		if err != nil {
			s.lg.Error("err restore block from object storage", zap.Error(err))
			theErr = errors.Wrapf(err, "restore block")
			return
		}

		theData = data
		vol, err := s.selector.Select(meta.VolID)
		if err != nil {
			s.lg.Error("err select vol", zap.Error(err))
			theErr = errors.Wrapf(err, "restore block")
			return
		}

		s.lg.Debug("restoring block ...", zap.String(BLOCK_ID, blockId))
		hash, err := hex.DecodeString(blockId)
		if err != nil {
			s.lg.Error("invalid block id", zap.Error(err))
			theErr = errors.Wrapf(err, "restore block")
			return
		}

		_, done, err := vol.WriteBlock(ctx, &engine.DataContainer{
			Index: meta.LogicIndex,
			Data:  data,
			Hash:  hash,
			Crc:   crc32.ChecksumIEEE(data),
		})
		if err != nil {
			theErr = errors.Wrapf(err, "restore block")
			return
		}
		<-done // wait for the block meta stored in the node

	})
	return
}
