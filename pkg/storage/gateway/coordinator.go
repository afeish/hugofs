package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	pbStorage "github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/node"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/dustin/go-humanize"
	"github.com/jellydator/ttlcache/v3"
	"github.com/pingcap/log"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	_ adaptor.StorageBackend = (*VolumeCoordinator)(nil)
)

// VolumeCoordinator is the entry point of the Storage VolumeCoordinator.
type VolumeCoordinator struct {
	NodeSelector NodeSelector
	cfg          *adaptor.Config

	metaProxy *MetaProxy

	blockMetaMu    *sync.Mutex // protect against block volume
	blockMetaCache *ttlcache.Cache[Ino, *pb.InoBlockMap]

	writeCh *WriteCh
	lg      *zap.Logger
}

func NewVolumeCoordinator(ctx context.Context, cfg *adaptor.Config, lg *zap.Logger) (*VolumeCoordinator, error) {
	c := &VolumeCoordinator{
		cfg:         cfg,
		blockMetaMu: &sync.Mutex{},
		lg:          lg.Named("coordinator"),
	}
	if err := c.init(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *VolumeCoordinator) init(ctx context.Context) error {
	cfg := s.cfg
	switch cfg.IO.Transport {
	case adaptor.TransportTypeMOCK:
		s.NodeSelector = NewMockNodeSelector()
	case adaptor.TransportTypeTCP:
		s.NodeSelector = NewTcpNodeSelector()
	default:
		s.NodeSelector = NewDefaultNodeSelector()
	}
	s.metaProxy = NewMetaProxy(ctx, cfg)
	s.blockMetaCache = ttlcache.New(
		ttlcache.WithTTL[Ino, *pb.InoBlockMap](30 * time.Minute),
	)

	go s.blockMetaCache.Start()

	s.writeCh = NewWriteCh(s.lg, s)

	return nil
}

func (s *VolumeCoordinator) Start(ctx context.Context) error {
	return nil
}

func (s *VolumeCoordinator) BatchRead(ctx context.Context, ino Ino, blockIdxList []uint64) (r map[uint64][]byte, err error) {
	ctx, span := tracing.Tracer.Start(ctx, "batch read block by ino + blockIdx", trace.WithAttributes(
		attribute.Int64("ino", int64(ino)),
		attribute.Int64Slice("index-slice", lo.Map(blockIdxList, func(item uint64, _ int) int64 { return int64(item) }))),
	)
	defer span.End()

	inoBlockMap, err := s.ensureBlockMetaCached(ctx, ino)
	if err != nil {
		return nil, err
	}

	inoBlockMap = lo.PickByKeys(inoBlockMap, blockIdxList)
	if len(inoBlockMap) == 0 {
		return nil, nil
	}

	blockMetaGroup := lo.GroupBy(lo.Values(inoBlockMap), func(meta *BlockMeta) string {
		return fmt.Sprintf("%d-%d", meta.PrimaryVolumeId, meta.SecondaryVolumeIds[0]) //TODO:
	})

	r = make(map[uint64][]byte, 0)

	for _, list := range blockMetaGroup {
		meta := list[0]
		s.blockMetaMu.Lock()
		volIds := lo.Shuffle(append(meta.SecondaryVolumeIds, meta.PrimaryVolumeId))
		blockIds := lo.Map(list, func(item *BlockMeta, index int) lo.Tuple2[uint64, string] {
			return lo.Tuple2[uint64, string]{A: item.BlockIndexInFile, B: item.BlockId}
		})
		volId0 := volIds[0]
		volId1 := volIds[1]
		s.blockMetaMu.Unlock()

		m, err := s.doRead(ctx, volId0, blockIds)
		if err != nil {
			m, err = s.doRead(ctx, volId1, blockIds)
			if err != nil {
				return nil, err
			}
		}
		join(r, m)
	}

	return
}

func join(dst, src map[uint64][]byte) {
	for k, v := range src {
		dst[k] = v
	}
}

func (s *VolumeCoordinator) ensureBlockMetaCached(ctx context.Context, ino Ino) (m map[uint64]*BlockMeta, err error) {
	r0 := s.blockMetaCache.Get(ino)
	if r0 != nil {
		m = r0.Value().Map
		return m, err
	}
	r, err := s.metaProxy.GetInoBlockMap(ctx, &pb.GetInoBlockMap_Request{Ino: ino})
	if err != nil {
		return nil, err
	}
	s.blockMetaCache.Set(ino, r.InoBlockMap, ttlcache.DefaultTTL)
	m = r.InoBlockMap.Map
	return m, err
}

// Read
// 1. get the ino block map from metadata server
// 2. let the virtual volume read the data
func (s *VolumeCoordinator) Read(ctx context.Context, key adaptor.BlockKey) (data []byte, err error) {
	ino, blockIdx := key.Split()
	ctx, span := tracing.Tracer.Start(ctx, "read block by ino + blockIdx", trace.WithAttributes(attribute.Int64("ino", int64(ino)), attribute.Int64("index", int64(blockIdx))))
	defer span.End()

	inoBlockMap, err := s.ensureBlockMetaCached(ctx, ino)
	if err != nil {
		return nil, err
	}
	inoBlockMap = lo.PickByKeys(inoBlockMap, []uint64{blockIdx})
	if len(inoBlockMap) == 0 {
		err = adaptor.ErrBlockNotFound
		return
	}
	meta := inoBlockMap[blockIdx]
	s.blockMetaMu.Lock()
	volIds := lo.Shuffle(append(meta.SecondaryVolumeIds, meta.PrimaryVolumeId))
	idPairs := newBlockIdPairs(blockIdx, meta.BlockId)
	volId0 := volIds[0]
	volId1 := volIds[1]
	s.blockMetaMu.Unlock()

	m, err := s.doRead(ctx, volId0, idPairs)
	if err != nil {
		m, err = s.doRead(ctx, volId1, idPairs)
		if err != nil {
			return nil, err
		}
	}
	return m[meta.BlockIndexInFile], nil
}

type blockIdPairs []lo.Tuple2[uint64, string]

func newBlockIdPairs(index uint64, blockId string) blockIdPairs {
	return []lo.Tuple2[uint64, string]{
		{A: index, B: blockId},
	}
}

func (p blockIdPairs) Ids() []string {
	ids := make([]string, 0, len(p))
	for _, item := range p {
		ids = append(ids, item.B)
	}
	return ids
}

func (p blockIdPairs) Index(id string) uint64 {
	for _, item := range p {
		if item.B == id {
			return item.A
		}
	}
	return 0
}

func (s *VolumeCoordinator) doRead(ctx context.Context, volId uint64, blockIds blockIdPairs) (data map[uint64][]byte, err error) {
	node, err := s.metaProxy.GetNode(volId)
	if err != nil {
		return
	}
	client, err := s.NodeSelector.Select(ctx, node)
	if err != nil {
		return
	}
	res, err := client.Read(ctx, &pbStorage.Read_Request{BlockIds: blockIds.Ids()})
	if err != nil {
		return
	}
	data = make(map[uint64][]byte)
	for _, block := range res.Blocks {
		data[blockIds.Index(block.Id)] = block.Data
	}

	return
}

func (s *VolumeCoordinator) Write(ctx context.Context, key adaptor.BlockKey, data []byte) (err error) {
	ino, blockIdx := key.Split()
	ctx, span := tracing.Tracer.Start(ctx, "write block by ino + blockIdx", trace.WithAttributes(attribute.Int64("ino", int64(ino)), attribute.Int64("block-index", int64(blockIdx))))
	defer span.End()

	nodes, err := s.metaProxy.GetTopNPlus(ctx, s.cfg.IO.Replica)
	if err != nil {
		return err
	}
	return s.doWrite(ctx, key, data, nodes)
}

func (s *VolumeCoordinator) BatchWrite(ctx context.Context, dataMap map[adaptor.BlockKey][]byte) error {
	ctx, span := tracing.Tracer.Start(ctx, "batch write block by ino + blockIdx-map")
	defer span.End()

	nodes, err := s.metaProxy.GetTopNPlus(ctx, s.cfg.IO.Replica)
	if err != nil {
		return err
	}

	g, gCtx := errgroup.WithContext(ctx)
	for key, data := range dataMap {
		key := adaptor.BlockKey(key)
		data := data
		g.Go(func() error {
			return s.doWrite(gCtx, key, data, nodes)
		})
	}
	if err = g.Wait(); err != nil {
		return err
	}
	return err
}

func (s *VolumeCoordinator) WriteAsync(ctx context.Context, key adaptor.BlockKey, data []byte) adaptor.Future {
	req := requestPool.Get().(*request)
	req.reset()
	req.Block = data
	req.Key = key
	req.Future = adaptor.NewDefaultFuture()
	req.IncrRef()
	s.writeCh.sendReq(req)
	return req.Future
}

func (s *VolumeCoordinator) FlushAsync(ctx context.Context, ino Ino) {
	s.writeCh.flushImmediate()
}

func (s *VolumeCoordinator) doWrite(ctx context.Context, key adaptor.BlockKey, data []byte, nodes []*StorageNode) (err error) {
	s.lg.Debug("write to storage", zap.Int("storage-cnt", len(nodes)))

	g, gCtx := errgroup.WithContext(ctx)

	crc := crc32.ChecksumIEEE(data)

	var (
		storages     []adaptor.StorageClient
		mu           sync.Mutex
		writeResults = make(map[*pb.StorageNode]*pbStorage.WriteBlock_Response)
	)

	for _, node := range nodes {
		node := node
		storage, err := s.NodeSelector.Select(ctx, node)
		if err != nil {
			return err
		}
		storages = append(storages, storage)
		g.Go(func() error {
			primary, err := storage.Write(gCtx, &pbStorage.WriteBlock_Request{
				Ino:       key.GetIno(),
				BlockIdx:  lo.ToPtr(key.GetIndex()),
				BlockData: data,
				Crc32:     crc,
			})
			if err != nil {
				return err
			}
			mu.Lock()
			writeResults[node] = primary
			mu.Unlock()
			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return err
	}

	primary := writeResults[nodes[0]]
	delete(writeResults, nodes[0])

	secondaryVolIds := make([]uint64, 0)
	for _, r := range lo.Values(writeResults) {
		secondaryVolIds = append(secondaryVolIds, r.VolId)
	}

	s.lg.Debug("write block succed", zap.Uint64("ino", primary.Ino), zap.String("block-id", primary.BlockId), zap.Uint64("block-index", primary.BlockIdx),
		zap.Uint64("primary-vol", primary.VolId), zap.String("len", humanize.IBytes(primary.Len)))
	if _, err := s.metaProxy.UpdateBlockLocation(ctx, &pb.UpdateBlockLocation_Request{Meta: &pb.BlockMeta{
		Ino:                primary.Ino,
		BlockIndexInFile:   primary.BlockIdx,
		BlockId:            primary.BlockId,
		PrimaryVolumeId:    primary.VolId,
		SecondaryVolumeIds: secondaryVolIds,
		Crc32:              primary.Crc32,
		Len:                primary.Len,
	}}); err != nil {
		s.lg.Error("update block location failed", zap.Error(err))
		for _, storage := range storages {
			storage.DeleteBlock(ctx, &pbStorage.DeleteBlock_Request{
				Ino:      primary.Ino,
				BlockIds: []string{primary.BlockId},
			})
		}
		return err
	}

	for _, node := range nodes {
		s.metaProxy.trackUsage(ctx, node.Id, uint64(len(data)))
	}
	s.blockMetaCache.Delete(key.GetIno())
	return nil
}

func (s *VolumeCoordinator) WriteAt(ctx context.Context, key adaptor.BlockKey, interval *adaptor.IntervalList[*adaptor.BlockWriteView], data []byte) (err error) {
	ino, blockIdx := key.Split()
	inoBlockMap, err := s.ensureBlockMetaCached(ctx, ino)
	if err != nil { // not found any block yet
		return s.Write(ctx, key, data)
	}

	inoBlockMap = lo.PickByKeys(inoBlockMap, []uint64{blockIdx})
	if len(inoBlockMap) == 0 { // found some blocks, but not the given blockIdx
		return s.Write(ctx, key, data)
	}
	s.blockMetaMu.Lock()
	meta := inoBlockMap[blockIdx]
	idPairs := newBlockIdPairs(blockIdx, meta.BlockId)
	volIds := lo.Shuffle(append(meta.SecondaryVolumeIds, meta.PrimaryVolumeId))
	volId0 := volIds[0]
	volId1 := volIds[1]
	m, err := s.doRead(ctx, volId0, idPairs)
	if err != nil {
		bs, _ := json.Marshal(meta)
		s.lg.Warn("corrupted data", zap.Uint64("ino", ino), zap.Uint64("block-index", blockIdx), zap.Uint64("vol-id", volId0), zap.String("meta", string(bs)), zap.Error(err))
		m, err = s.doRead(ctx, volId1, idPairs)
		if err != nil {
			log.Warn("corrupted data", zap.Uint64("ino", ino), zap.Uint64("block-index", blockIdx), zap.Uint64("vol-id", volId1))
			return err
		}
	}

	blockData := m[blockIdx]
	s.lg.Debug("reuse exist block data", zap.Uint64("ino", ino), zap.String("block-id", meta.BlockId), zap.Uint64("block-index", blockIdx), zap.Int("orig-data-len", len(blockData)), zap.Int("new-data-len", len(data)))
	_, err = interval.Copy(blockData, data)
	if err != nil {
		bs, _ := json.Marshal(meta)
		s.lg.Info("reuse exist block data", zap.Uint64("ino", ino), zap.Uint64("block-index", blockIdx), zap.Int("orig-data-len", len(blockData)), zap.Int("new-data-len", len(data)), zap.Uint64("vol-id", volId0), zap.String("meta", string(bs)), zap.Error(err))
		return err
	}

	var nodes []*StorageNode
	node, err := s.metaProxy.GetNode(meta.PrimaryVolumeId)
	if err != nil {
		return err
	}
	nodes = append(nodes, node)

	for _, volId := range meta.SecondaryVolumeIds {
		node, err := s.metaProxy.GetNode(volId)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	s.blockMetaMu.Unlock()
	return s.doWrite(ctx, key, blockData, nodes)
}

func (s *VolumeCoordinator) Delete(ctx context.Context, ino Ino) error {
	r, err := s.metaProxy.GetInoBlockMap(ctx, &pb.GetInoBlockMap_Request{Ino: ino})
	if err != nil {
		return err
	}

	m := make(map[uint64][]string)

	for _, block := range r.InoBlockMap.Map {
		metas, ok := m[block.PrimaryVolumeId]
		if ok {
			metas = append(metas, block.BlockId)
		} else {
			metas = []string{block.BlockId}
		}
		m[block.PrimaryVolumeId] = metas
		for _, volId := range block.SecondaryVolumeIds {
			metas, ok = m[volId]
			if ok {
				metas = append(metas, block.BlockId)
			} else {
				metas = []string{block.BlockId}
			}
			m[volId] = metas
		}

	}

	f := func(ctx context.Context, volId uint64, blockIds []string) error {
		node, err := s.metaProxy.GetNode(volId)
		if err != nil {
			return err
		}
		client, err := s.NodeSelector.Select(ctx, node)
		if err != nil {
			return err
		}
		// TODO: a blockId may be referenced by multi files. So blockId should be reference-counting
		_, err = client.DeleteBlock(ctx, &pbStorage.DeleteBlock_Request{BlockIds: blockIds})
		if err != nil {
			return err
		}
		return nil
	}

	g, gCtx := errgroup.WithContext(ctx)

	for volId, blockIds := range m {
		volId := volId
		blockIds := blockIds
		g.Go(func() error { return f(gCtx, volId, blockIds) })
	}

	if err = g.Wait(); err != nil {
		return err
	}

	return nil
}

func (s *VolumeCoordinator) GetBlockViews(ctx context.Context, ino Ino) *adaptor.IntervalList[*adaptor.BlockKeyView] {
	inoBlockMap, err := s.ensureBlockMetaCached(ctx, ino)
	if err != nil { // not found any block yet
		return nil
	}

	list := adaptor.NewIntervalList[*adaptor.BlockKeyView]()

	for k, v := range inoBlockMap {
		view := adaptor.NewBlockKeyView(ino, k)
		start, end := int64(k)*int64(s.cfg.IO.BlockSize), int64(k)*int64(s.cfg.IO.BlockSize)+int64(v.Len)
		view.SetStartStop(start, end)
		interval := adaptor.NewInterval[*adaptor.BlockKeyView](start, end)
		interval.Value = view
		list.AddInterval(interval)
	}
	return list
}

func (s *VolumeCoordinator) Stop() error {
	return nil
}

func (s *VolumeCoordinator) Destory() {
	s.blockMetaCache.Stop()
	s.NodeSelector.Close()
	s.metaProxy.Shutdown()

	if s.cfg.Test.Enabled {
		ResetStorageClientMock()
		node.ResetMetaClientMock()
		store.CloseAll()
	}
}
