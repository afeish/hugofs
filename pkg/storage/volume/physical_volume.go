package volume

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/block"
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/dustin/go-humanize"
	"github.com/jinzhu/copier"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

const (
	PhysicalVolumeSize size.SizeSuffix = 40 * size.GibiByte // 40GB
	SlotSize                           = PhysicalVolumeSize / block.Size
	DefaultThrehold                    = 0.75
)

type PhysicalVolume interface {
	GetID(ctx context.Context) uint64
	CalculateUsage(ctx context.Context) (uint64, error)
	GetFreeBlockCnt(ctx context.Context) (uint64, error)
	GetCap(ctx context.Context) (uint64, error)           // maybe not needed.
	GetTotalBlockCnt(ctx context.Context) (uint64, error) // maybe not needed.
	ReadBlock(ctx context.Context, hash string) ([]byte, *block.Header, error)
	WriteBlock(ctx context.Context, container *engine.DataContainer) (string, chan struct{}, error)
	CancelBlock(ctx context.Context, index BlockIndexInPV) (uint64, error)
	RemoveLocal(ctx context.Context, hash string) error
	CleanInvalidBlock(ctx context.Context, invalidBlocks ...string) (bool, error)
	GetExpiredBlocks(ctx context.Context) ([]lo.Tuple2[int, string], error)
	ExpireBlock(ctx context.Context, index BlockIndexInPV) error
	Shutdown(ctx context.Context) error
}

type Config struct {
	store.UnimplementedSerializer

	Engine engine.EngineType `copier:"Type"`
	Prefix string
	Id     PhysicalVolumeID
	NodeId StorageNodeID

	Quota     *size.SizeSuffix
	BlockSize *size.SizeSuffix
	Threhold  *float64

	DB        adaptor.DbOption
	CreatedAt uint64
}

func (s *Config) FormatKey() string {
	return fmt.Sprintf("%s%d", s.FormatPrefix(), s.Id)
}
func (s *Config) FormatPrefix() string {
	return "vol_cfg/"
}
func (s *Config) Serialize() ([]byte, error) {
	return json.Marshal(s)
}
func (s *Config) Deserialize(bytes []byte) (*Config, error) {
	var tmp Config
	if err := json.Unmarshal(bytes, &tmp); err != nil {
		return nil, err
	}
	return &tmp, nil
}
func (s *Config) Self() *Config {
	return s
}

func (s *Config) ToPhysicalVolume() *storage.PhysicalVolume {

	var vol = &storage.PhysicalVolume{
		Id:        s.Id,
		Type:      storage.EngineType(storage.EngineType_value[s.Engine.String()]),
		Prefix:    s.Prefix,
		Quota:     uint64(s.Quota.Int()),
		Threhold:  *s.Threhold,
		CreatedAt: s.CreatedAt,
	}
	return vol
}

type ConfigQuery struct {
	store.UnimplementedSerializer

	Engine engine.EngineType
	Prefix string
	ID     PhysicalVolumeID
}

func (s *ConfigQuery) FormatKey() string {
	return fmt.Sprintf("%s%s:%s", s.FormatPrefix(), s.Engine, s.Prefix)
}
func (s *ConfigQuery) FormatPrefix() string {
	return "vol_cfg_query/"
}
func (s *ConfigQuery) Serialize() ([]byte, error) {
	return json.Marshal(s)
}
func (s *ConfigQuery) Deserialize(bytes []byte) (*ConfigQuery, error) {
	var tmp ConfigQuery
	if err := json.Unmarshal(bytes, &tmp); err != nil {
		return nil, err
	}
	return &tmp, nil
}
func (s *ConfigQuery) Self() *ConfigQuery {
	return s
}

type VolumeStatistics struct {
	mu    *sync.RWMutex `json:"-"`
	store store.Store   `json:"-"`

	VolID PhysicalVolumeID `json:"-"`

	Quota      size.SizeSuffix
	Threhold   float64
	InuseBytes uint64

	notifyCh chan VolumeStatistics
	lg       *zap.Logger
}

func newVolumeStatistics(ctx context.Context, volID PhysicalVolumeID, quota size.SizeSuffix, threhold float64, s store.Store, notifyCh chan VolumeStatistics, lg *zap.Logger) *VolumeStatistics {
	volStat := &VolumeStatistics{
		mu:       &sync.RWMutex{},
		VolID:    volID,
		Quota:    quota,
		Threhold: threhold,
		store:    s,
		notifyCh: notifyCh,
		lg:       lg,
	}

	if err := volStat.Load(ctx); err != nil && err != store.ErrNotFound {
		lg.Warn("err load vol stat", zap.Error(err))
	}
	notifyCh <- volStat.Clone()
	return volStat
}

func (v *VolumeStatistics) GetInUseBytes() uint64 {
	return v.InuseBytes
}

func (v *VolumeStatistics) AddInUseBytes(delta int) {
	v.mu.Lock()
	if delta < 0 {
		v.InuseBytes -= uint64(delta)
	} else {
		v.InuseBytes += uint64(delta)
	}
	if err := v._persist(context.Background()); err != nil {
		v.lg.Error("err persist vol stat")
	}
	v.lg.Debug("inuseBytes changed", zap.String("inuse-bytes", humanize.IBytes(v.InuseBytes)))
	v.mu.Unlock()
	v.notifyCh <- v.Clone()
}

func (v *VolumeStatistics) RemainCap() uint64 {
	return uint64(v.Quota) - v.InuseBytes
}

func (v *VolumeStatistics) Format() string {
	return fmt.Sprintf("vol_stat/%d", v.VolID)
}

func (v *VolumeStatistics) Persist(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v._persist(ctx)
}

func (v *VolumeStatistics) _persist(ctx context.Context) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	// v.lg.Debug("persist", zap.String("json", string(buf)))
	return v.store.Save(ctx, v.Format(), buf)
}

func (v *VolumeStatistics) Load(ctx context.Context) error {
	buf, err := v.store.Get(ctx, v.Format())
	if err != nil {
		return err
	}
	// v.lg.Debug("load", zap.String("json", string(buf)))
	if err := json.Unmarshal(buf, &v); err != nil {
		return err
	}
	return nil
}

func (vs *VolumeStatistics) isAboveWatermark() bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.InuseBytes > uint64(float64(vs.Quota)*vs.Threhold)
}

func (vs *VolumeStatistics) GetCap() uint64 {
	return uint64(float64(vs.Quota) * vs.Threhold)
}

func (vs *VolumeStatistics) Clone() VolumeStatistics {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	c := VolumeStatistics{}
	copier.Copy(&c, vs)
	c.mu = &sync.RWMutex{}
	c.store = nil
	c.notifyCh = nil
	return c
}

func (vs *VolumeStatistics) String() string {
	var buf bytes.Buffer

	e := json.NewEncoder(&buf)
	e.SetIndent("", "  ")

	if err := e.Encode(vs); err != nil {
		return "unable to encode as JSON: " + err.Error()
	}
	return buf.String()
}

const (
	BlockMetaEvent_Add BlockMetaEventType = iota
	BlockMetaEvent_Delete
)

type BlockMetaEventType int

type BlockMetaEvent struct {
	store.UnimplementedSerializer
	VolID      PhysicalVolumeID
	BlockID    BlockID
	Size       uint64
	LogicIndex uint64
	Crc32      uint32
	Done       chan struct{} `json:"-"`
	EventType  BlockMetaEventType
	State      storage.BlockState
	CreatedAt  time.Time
}

func (s *BlockMetaEvent) FormatKey() string {
	return fmt.Sprintf("%s%s", s.FormatPrefix(), s.BlockID)
}
func (s *BlockMetaEvent) FormatPrefix() string {
	return "block/"
}
func (s *BlockMetaEvent) Serialize() ([]byte, error) {
	return json.Marshal(s)
}
func (s *BlockMetaEvent) Deserialize(bytes []byte) (*BlockMetaEvent, error) {
	var tmp BlockMetaEvent
	if err := json.Unmarshal(bytes, &tmp); err != nil {
		return nil, err
	}
	return &tmp, nil
}
func (s *BlockMetaEvent) Self() *BlockMetaEvent {
	return s
}

func (s *BlockMetaEvent) OnS3Only() bool {
	return s.State == storage.BlockState_S3_ONLY
}

func (s *BlockMetaEvent) OnLocal() bool {
	return s.State == storage.BlockState_LOCAL
}

func (s *BlockMetaEvent) OnBoth() bool {
	return s.State == storage.BlockState_LOCAL_S3
}
