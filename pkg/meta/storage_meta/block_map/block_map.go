package block_map

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/tracing"

	"github.com/samber/lo"
)

// InoBlockMapManager manages the mapping between ino and blockMeta.
// This is responsible for handling modification of InoBlockMap.
type InoBlockMapManager struct {
	metaStore store.Store // this is a reference
}

func NewInoBlockMapManager(metaStore store.Store) (*InoBlockMapManager, error) {
	return &InoBlockMapManager{
		metaStore: metaStore,
	}, nil
}

// GetInoBlockMap get the BlockMap of the specified Ino.
func (m *InoBlockMapManager) GetInoBlockMap(ctx context.Context, ino Ino) (*InoBlockMap, error) {
	ctx, span := tracing.Tracer.Start(ctx, "get ino block map by ino")
	defer span.End()

	blockMetaMaps, err := store.DoGetValuesByPrefix[BlockMeta](ctx, m.metaStore, &BlockMeta{
		BlockMeta: &pb.BlockMeta{
			Ino: ino,
		},
	})
	if err != nil {
		return nil, err
	}

	// TODO: optimize me
	out := &InoBlockMap{
		Ino:    ino,
		Blocks: make(map[uint64]*BlockMeta),
	}
	for _, e := range blockMetaMaps {
		out.Blocks[e.BlockIndexInFile] = e
		if out.VirtualVolumeID == 0 {
			out.VirtualVolumeID = e.VirtualVolumeId
		}
	}

	return out, nil
}

func (m *InoBlockMapManager) GetHistoryBlockMap(ctx context.Context, ino Ino) (*InoBlockMap, error) {
	// TODO: get all version of ino block map
	blockMetaMaps, err := store.DoGetValuesByPrefix[BlockMeta](ctx, m.metaStore, &BlockMeta{
		BlockMeta: &pb.BlockMeta{
			Ino: ino,
		},
	})
	if err != nil {
		return nil, err
	}

	// TODO: optimize me
	out := &InoBlockMap{
		Ino:    ino,
		Blocks: make(map[uint64]*BlockMeta),
	}
	for _, e := range blockMetaMaps {
		out.Blocks[e.BlockIndexInFile] = e
		if out.VirtualVolumeID == 0 {
			out.VirtualVolumeID = e.VirtualVolumeId
		}
	}

	return out, nil
}

// GetInoBlockMeta get the BlockMeta of the specified Ino and BlockIndexInFile.
func (m *InoBlockMapManager) GetInoBlockMeta(ctx context.Context, ino Ino, index BlockIndexInFile) (*BlockMeta, error) {
	ctx, span := tracing.Tracer.Start(ctx, "get ino block map by ino + index")
	defer span.End()
	blockMeta, err := store.DoGetValueByKey[BlockMeta](ctx, m.metaStore, &BlockMeta{
		BlockMeta: &pb.BlockMeta{
			Ino:              ino,
			BlockIndexInFile: index,
		},
	})
	if err != nil {
		return nil, err
	}

	return blockMeta, nil
}

// UpdateInoBlockMap update the BlockMap of the specified Ino.
//
// You should call it when modify the mapping.
// This function should better be invoked in the transaction.
func (m *InoBlockMapManager) UpdateInoBlockMap(ctx context.Context, blockMeta *BlockMeta) error {
	ctx, span := tracing.Tracer.Start(ctx, "update ino block map")
	defer span.End()
	return store.DoSaveInKVPair[BlockMeta](ctx, m.metaStore, blockMeta)
}

func (m *InoBlockMapManager) ArchiveBlockMeta(ctx context.Context, blockMeta *BlockMeta) error {
	history := BlockMetaHistoryFromBlockMeta(blockMeta)
	return store.DoSaveInKVPair[BlockMetaHistory](ctx, m.metaStore, history)
}

// BatchUpdateInoBlockMap update the BlockMap of the specified Ino.
func (m *InoBlockMapManager) BatchUpdateInoBlockMap(ctx context.Context, list []*BlockMeta) error {
	return store.DoBatchSaveStrictly(ctx, m.metaStore, lo.Map(list, func(item *BlockMeta, _ int) store.Serializer[BlockMeta] {
		return item
	}))
}

// DeleteInoBlockMap will delete all the BlockMaps of the specified Ino.
func (m *InoBlockMapManager) DeleteInoBlockMap(ctx context.Context, ino Ino) error {
	return store.DoDeleteByKey[BlockMeta](ctx, m.metaStore, &BlockMeta{
		BlockMeta: &pb.BlockMeta{
			Ino: ino,
		},
	})
}

type (
	InoBlockMap struct {
		Ino             Ino
		VirtualVolumeID uint64
		Blocks          map[BlockIndexInFile]*BlockMeta
	}
	BlockMeta struct {
		store.UnimplementedSerializer
		*pb.BlockMeta
	}
	BlockMetaHistory struct {
		store.UnimplementedSerializer
		*pb.BlockMeta
		Timestamp int64
	}
	LogicBlockAddr struct {
		Ino   Ino
		Index BlockIndexInFile
	}
	PhysicalBlockAddr struct {
		PhysicalVolumeID PhysicalVolumeID
		Index            BlockIndexInPV
	}
)

// FormatKey format the key of the InoBlockMap.
// key: ino_block_map/<ino>/<BlockIndexInFile>
// value: BlockMeta
func (b *BlockMeta) FormatKey() string {
	return fmt.Sprintf("%s%d", b.FormatPrefix(), b.BlockIndexInFile)
}
func (b *BlockMeta) FormatPrefix() string {
	return fmt.Sprintf("ino_block_map/%d/", b.Ino)
}
func (b *BlockMeta) Serialize() ([]byte, error) {
	return json.Marshal(b)
}
func (b *BlockMeta) Deserialize(bytes []byte) (*BlockMeta, error) {
	var out BlockMeta
	if err := json.Unmarshal(bytes, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (b *BlockMeta) Self() *BlockMeta {
	return b
}
func BlockMetaFromPB(meta *pb.BlockMeta) *BlockMeta {
	return &BlockMeta{
		BlockMeta: meta,
	}
}

var _ store.Serializer[BlockMeta] = (*BlockMeta)(nil)

// FormatKey format the key of the InoBlockMap.
// key: ino_block_map_history/<ino>/<BlockIndexInFile>/<archive-timestamp>
// value: BlockMeta
func (b *BlockMetaHistory) FormatKey() string {
	return fmt.Sprintf("%s%d", b.FormatPrefix(), b.Timestamp)
}
func (b *BlockMetaHistory) FormatPrefix() string {
	return fmt.Sprintf("ino_block_map_history/%d/%d", b.Ino, b.BlockIndexInFile)
}
func (b *BlockMetaHistory) Serialize() ([]byte, error) {
	return json.Marshal(b)
}
func (b *BlockMetaHistory) Deserialize(bytes []byte) (*BlockMetaHistory, error) {
	var out BlockMetaHistory
	if err := json.Unmarshal(bytes, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (b *BlockMetaHistory) Self() *BlockMetaHistory {
	return b
}
func BlockMetaHistoryFromPB(meta *pb.BlockMeta) *BlockMetaHistory {
	return &BlockMetaHistory{
		BlockMeta: meta,
	}
}
func BlockMetaHistoryFromBlockMeta(meta *BlockMeta) *BlockMetaHistory {
	return &BlockMetaHistory{
		BlockMeta: meta.BlockMeta,
		Timestamp: time.Now().Unix(),
	}
}

var _ store.Serializer[BlockMetaHistory] = (*BlockMetaHistory)(nil)
