package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"

	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/engine"
	vol "github.com/afeish/hugo/pkg/storage/volume"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/afeish/hugo/pkg/util/size"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (v *Node) Read(ctx context.Context, req *storage.Read_Request) (*storage.Read_Response, error) {
	var (
		res storage.Read_Response
	)
	ctx, span := tracing.Tracer.Start(ctx, "read block from node", trace.WithAttributes(attribute.Int64("ino", int64(req.Ino)), attribute.StringSlice("ids", req.BlockIds)))
	defer span.End()

	g, gCtx := errgroup.WithContext(ctx)

	for _, blockId := range req.GetBlockIds() {
		blockId := blockId
		meta := v.selector.GetBlockMeta(blockId)
		if meta == nil {
			v.lg.Error("not found given block", zap.String(BLOCK_ID, blockId))
			return nil, engine.ErrBlockNotExisted
		}

		if meta.OnS3Only() {
			data, err := v.objectSyncer.Restore(meta)
			if err != nil {
				v.lg.Error("err restore block when read", zap.String(BLOCK_ID, blockId), zap.Error(err))
				continue
			}

			res.Blocks = append(res.Blocks, &storage.Block{
				Data:  data,
				Len:   uint32(len(data)),
				Index: meta.LogicIndex,
				Crc32: uint32(meta.Crc32),
				Id:    blockId})
			continue
		}

		vol, err := v.selector.SelectRead(blockId)
		if err != nil {
			v.lg.Error("not found given block", zap.String(BLOCK_ID, blockId))
			return nil, err
		}

		v.lg.Debug("select volume to read", zap.Uint64(VOL_ID, vol.GetID(gCtx)), zap.String(BLOCK_ID, blockId))
		g.Go(func() error {
			data, header, err := vol.ReadBlock(gCtx, blockId)
			if err != nil {
				v.lg.Error("not found given block", zap.Uint64(VOL_ID, vol.GetID(ctx)), zap.String(BLOCK_ID, blockId))
				return err
			}
			v.lg.Sugar().Debugf("header: %s, data: %s", header, size.ReadSummary(data, 10))
			res.Blocks = append(res.Blocks, &storage.Block{Data: data, Len: uint32(header.DataLen), Index: header.Index, Crc32: header.Crc32, Id: header.HashReadable()})
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		v.lg.Error("read block from volumes", zap.Error(err))
		return nil, err
	}
	return &res, nil
}

func (v *Node) StreamRead(request *storage.Read_Request, server storage.StorageService_StreamReadServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamRead not implemented")
}
func (v *Node) Write(ctx context.Context, req *storage.WriteBlock_Request) (*storage.WriteBlock_Response, error) {
	crc := crc32.ChecksumIEEE(req.BlockData)
	if crc != req.Crc32 {
		return nil, status.Errorf(codes.FailedPrecondition, "crc32 mismatch, expected %d, actual %d", req.Crc32, crc)
	}

	ctx, span := tracing.Tracer.Start(ctx, "write block to node", trace.WithAttributes(attribute.Int64("ino", int64(req.Ino)), attribute.Int64("index", int64(*req.BlockIdx))))
	defer span.End()

	h := sha256.New()
	h.Write(req.BlockData)
	hash := h.Sum(nil)
	hashStr := hex.EncodeToString(h.Sum(nil))

	existMeta := v.selector.GetBlockMeta(hashStr)
	if existMeta != nil { // FIX https://github.com/afeish00/hugo/-/issues/10
		v.lg.Debug("found existed block info", zap.Uint64("ino", req.Ino), zap.Uint64("req-index", lo.FromPtr(req.BlockIdx)), zap.Uint64("exist-index", existMeta.LogicIndex), zap.String(BLOCK_ID, existMeta.BlockID), zap.Uint64(VOL_ID, existMeta.VolID))
		return &storage.WriteBlock_Response{
			Ino:      req.Ino,
			BlockIdx: lo.FromPtr(req.BlockIdx),
			BlockId:  existMeta.BlockID,
			VolId:    existMeta.VolID,
			NodeId:   v.ID,
			Crc32:    crc,
			Len:      uint64(len(req.BlockData)),
		}, nil
	}

	vol, err := v.selector.SelectWrite()
	if err != nil {
		return nil, errors.Wrapf(err, "write block")
	}
	// write to a physical volume.
	blockId, done, err := vol.WriteBlock(ctx, &engine.DataContainer{Data: req.BlockData, Hash: hash, Index: *req.BlockIdx, Crc: crc})
	if err != nil {
		return nil, errors.Wrapf(err, "write block")
	}
	<-done // wait for the block meta stored in the node

	v.lg.Debug("write block success", zap.Uint64("ino", req.Ino), zap.Uint64("index", lo.FromPtr(req.BlockIdx)), zap.String(BLOCK_ID, blockId), zap.Uint64(VOL_ID, vol.GetID(ctx)))
	return &storage.WriteBlock_Response{
		Ino:      req.Ino,
		BlockIdx: lo.FromPtr(req.BlockIdx),
		BlockId:  blockId,
		VolId:    vol.GetID(ctx),
		NodeId:   v.ID,
		Crc32:    crc,
		Len:      uint64(len(req.BlockData)),
	}, nil
}
func (v *Node) StreamWrite(server storage.StorageService_StreamWriteServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamWrite not implemented")
}
func (v *Node) DeleteBlock(ctx context.Context, req *storage.DeleteBlock_Request) (*storage.DeleteBlock_Response, error) {
	var (
		res storage.DeleteBlock_Response
	)
	g, gCtx := errgroup.WithContext(ctx)

	m := make(map[uint64][]string)
	for _, blockId := range req.GetBlockIds() {
		blockId := blockId
		meta := v.selector.GetBlockMeta(blockId)
		if meta == nil {
			v.lg.Warn("when delete block, not found given block", zap.String(BLOCK_ID, blockId))
			continue
		}
		blockIds, ok := m[meta.VolID]
		if ok {
			blockIds = append(blockIds, blockId)
		} else {
			blockIds = []string{blockId}
		}
		m[meta.VolID] = blockIds
	}

	for volId, blockIds := range m {
		vol, err := v.selector.Select(volId)
		if err != nil {
			v.lg.Error("when delete block, err select volume", zap.Uint64(VOL_ID, volId), zap.Error(err))
			return nil, err
		}
		blockIds := blockIds

		g.Go(func() error {
			_, err = vol.CleanInvalidBlock(gCtx, blockIds...)
			if err != nil {
				v.lg.Error("when delete block", zap.Uint64(VOL_ID, vol.GetID(ctx)), zap.Error(err))
				return err
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		v.lg.Error("delete block from volumes", zap.Error(err))
		return nil, err
	}
	return &res, nil
}
func (v *Node) ListBlocks(ctx context.Context, req *storage.ListBlocks_Request) (*storage.ListBlocks_Response, error) {
	var (
		res    storage.ListBlocks_Response
		nodeId = v.ID
	)
	blockMetas := v.selector.GetBlockMetas()
	lo.ForEach(blockMetas, func(item *vol.BlockMetaEvent, index int) {
		res.Blocks = append(res.Blocks, &storage.ListBlocks_BlockMeta{
			BlockId: item.BlockID,
			NodeId:  nodeId,
			Size:    item.Size,
			VolId:   item.VolID,
			State:   item.State,
			Crc:     item.Crc32,
		})
	})
	return &res, nil
}

func (v *Node) EvictBlock(ctx context.Context, req *storage.EvictBlock_Request) (*storage.EvictBlock_Response, error) {
	blockMetas := v.selector.ListStaleBlockMetas(int64(req.Expired))

	for _, meta := range blockMetas {
		v.lg.Debug("sync block to remote", zap.String(BLOCK_ID, meta.BlockID))
		v.objectSyncer.Sync(meta)
	}

	return &storage.EvictBlock_Response{}, nil
}

func (v *Node) RecycleSpace(ctx context.Context, req *storage.RecycleSpace_Request) (*storage.RecycleSpace_Response, error) {
	blockMetas := v.selector.Recycle()

	for _, meta := range blockMetas {
		v.lg.Debug("delete locally", zap.String(BLOCK_ID, meta.BlockID))
		vol, _ := v.selector.Select(meta.VolID)
		vol.RemoveLocal(ctx, meta.BlockID)
		v.selector.MarkBlockFreeLocal(meta.BlockID)
	}

	return &storage.RecycleSpace_Response{}, nil
}

func (v *Node) Warmup(ctx context.Context, req *storage.Warmup_Request) (*storage.Warmup_Response, error) {
	blockMetas := v.selector.Warmup()

	blockIds := make([]string, 0, len(blockMetas))
	for _, meta := range blockMetas {
		v.lg.Debug("warmup block", zap.String(BLOCK_ID, meta.BlockID))
		if _, err := v.objectSyncer.Restore(meta); err != nil {
			return nil, err
		}
		blockIds = append(blockIds, meta.BlockID)
	}

	return &storage.Warmup_Response{
		BlockIds: blockIds,
	}, nil
}
