package volume

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/RoaringBitmap/roaring/roaring64"
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/store"
	"github.com/pingcap/log"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var (
	ErrSlotOverflow = errors.New("slot overflow")
	ErrSlotOccupied = errors.New("slot occupied")
	ErrSlotEmpty    = errors.New("slot empty")
	ErrSlotNotFound = errors.New("slot not found")
)

type Slot struct {
	bits *roaring64.Bitmap
	cap  uint64

	volID PhysicalVolumeID // immutable

	mu        sync.RWMutex
	store     store.Store // take a reference
	hashPairs map[string]uint64
}

func NewSlot(volID uint64, cap uint64, store store.Store) *Slot {
	slot := &Slot{
		volID:     volID,
		bits:      roaring64.NewBitmap(),
		cap:       cap,
		store:     store,
		hashPairs: make(map[string]uint64),
	}
	_ = slot.load()
	return slot
}

func (s *Slot) Occupy(ctx context.Context, offset uint64, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if offset+1 > s.cap {
		return ErrSlotOverflow
	}

	if !s.bits.CheckedAdd(offset) {
		return ErrSlotOccupied
	}
	s.hashPairs[hash] = offset
	if err := s.persistSlot(ctx, offset, hash); err != nil {
		log.Error("persist slot state", zap.Uint64("vol-id", s.volID))
		return err
	}
	return nil
}

func (s *Slot) Free(offset uint64) (string, error) {
	if offset > s.cap {
		return "", ErrSlotOverflow
	}
	if !s.bits.CheckedRemove(offset) {
		return "", ErrSlotEmpty
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, found := lo.FindKey(s.hashPairs, offset)
	if !found {
		return "", ErrSlotEmpty
	}
	if err := store.DoDeleteByKey[blockHash](context.Background(), s.store, newBlockHashQuery(s.volID, offset)); err != nil {
		return "", err
	}
	return hash, nil
}

func (s *Slot) Occupied(offset uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bits.Contains(offset)
}

func (s *Slot) GetHash(offset uint64) (string, error) {
	if !s.Occupied(offset) {
		return "", ErrSlotEmpty
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, found := lo.FindKey(s.hashPairs, offset)
	if !found {
		return "", ErrSlotEmpty
	}
	return key, nil
}

func (s *Slot) GetSlot(hash string) (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	slot, ok := s.hashPairs[hash]
	if !ok {
		return 0, ErrSlotEmpty
	}
	return slot, nil
}

func (s *Slot) Next() uint64 {
	//FIXME: bitmap iterator failed during add and remove
	s.mu.Lock()
	defer s.mu.Unlock()
	var idx uint64

	for {
		it := s.bits.Iterator()
		for it.HasNext() {
			if idx != it.Next() {
				break
			}
			idx++
		}
		if idx < s.cap {
			return idx
		}
	}
}

func (s *Slot) Remains() uint64 {
	return s.cap - s.bits.GetCardinality()
}

func (s *Slot) Size() uint64 {
	return s.bits.GetCardinality()
}

func (s *Slot) PersistState(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s._persistState(ctx)
}

func (s *Slot) _persistState(ctx context.Context) error {
	// persist bitmap into store
	data, err := s.bits.MarshalBinary()
	if err != nil {
		return err
	}

	if err := s.store.Save(ctx, s.Format(), data); err != nil {
		return err
	}
	return nil
}

func (s *Slot) persistSlot(ctx context.Context, offset uint64, hash string) error {
	store.DoSaveInKVPair[blockHash](ctx, s.store, newBlockHash(s.volID, offset, hash))
	//TODO: sync the bitmap in sync way really needed??? Refactor needed here,since sync the bitmap in sync way may greatly deduce the performance
	return s._persistState(ctx)
}

func (s *Slot) load() error {
	// load bitmap from store
	data, err := s.store.Get(context.Background(), s.Format())
	if err != nil {
		return err
	}

	if err := s.bits.UnmarshalBinary(data); err != nil {
		return err
	}

	s.hashPairs = make(map[string]uint64)
	blockHashMap, err := store.DoGetValuesByPrefix[blockHash](context.Background(), s.store, &blockHash{VolId: s.volID})
	if err != nil {
		return err
	}

	s.hashPairs = lo.MapEntries(blockHashMap, func(key string, value *blockHash) (string, uint64) {
		return value.Hash, value.Slot
	})
	return nil
}

func (s *Slot) Format() string {
	return fmt.Sprintf("slot_%d", s.volID)
}

type blockHash struct {
	store.UnimplementedSerializer
	VolId uint64
	Slot  uint64
	Hash  string
}

func newBlockHash(volId uint64, slot uint64, hash string) *blockHash {
	return &blockHash{
		VolId: volId,
		Slot:  slot,
		Hash:  hash,
	}
}

func newBlockHashQuery(volId uint64, slot uint64) *blockHash {
	return &blockHash{
		VolId: volId,
		Slot:  slot,
	}
}

func (s *blockHash) FormatKey() string {
	return fmt.Sprintf("%s%d", s.FormatPrefix(), s.Slot)
}
func (s *blockHash) FormatPrefix() string {
	return fmt.Sprintf("slot_hash_%d/", s.VolId)
}
func (s *blockHash) Serialize() ([]byte, error) {
	return json.Marshal(s)
}
func (s *blockHash) Deserialize(bytes []byte) (*blockHash, error) {
	var tmp blockHash
	if err := json.Unmarshal(bytes, &tmp); err != nil {
		return nil, err
	}
	return &tmp, nil
}
func (s *blockHash) Self() *blockHash {
	return s
}
