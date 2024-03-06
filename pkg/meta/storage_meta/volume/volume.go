package volume

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore //lint:ignore ST1001 ignore this!
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/meta/cluster"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/tracing"

	"github.com/pingcap/log"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/proto"
)

// Manager manages all metadata about volumes no matter physical or virtual.
type Manager struct {
	metaStore      store.Store // take a reference
	clusterManager *cluster.Service
}

func NewManager(clusterManager *cluster.Service, s store.Store) (*Manager, error) {
	return &Manager{
		metaStore:      s,
		clusterManager: clusterManager,
	}, nil
}

// ListVirtualVolumes returns all virtual volumes.
func (m *Manager) ListVirtualVolumes(ctx context.Context) ([]*VirtualVolume, error) {
	vols, err := store.DoGetValuesByPrefix[VirtualVolume](ctx, m.metaStore, &VirtualVolume{})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		log.Error("failed to get virtual volumes", zap.Error(err))
		return nil, err
	}
	return maps.Values(vols), nil
}

// ListPhysicalVolumes returns all physical volumes.
func (m *Manager) ListPhysicalVolumes(ctx context.Context) ([]*PhysicalVolume, error) {
	ctx, span := tracing.Tracer.Start(ctx, "list physical volumes")
	defer span.End()
	vols, err := store.DoGetValuesByPrefix[PhysicalVolume](ctx, m.metaStore, &PhysicalVolume{})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		log.Error("failed to get physical volumes", zap.Error(err))
		return nil, err
	}
	return maps.Values(vols), nil
}

// ListPhysicalVolumesInNode returns all physical volumes in the specified node.
func (m *Manager) ListPhysicalVolumesInNode(ctx context.Context, id StorageNodeID) ([]*PhysicalVolume, error) {
	ctx, span := tracing.Tracer.Start(ctx, "list physical volumes by storage node id")
	defer span.End()
	baseErr := fmt.Errorf("failed to get physical volumes under node %d", id)
	node, err := m.clusterManager.GetStorageNode(ctx, &pb.GetStorageNode_Request{Arg: &pb.GetStorageNode_Request_Id{Id: id}})
	if err != nil {
		return nil, errors.Join(baseErr, err)
	}

	// FIXME: optimize me
	vols, err := store.DoGetValuesByPrefix[PhysicalVolume](ctx, m.metaStore, &PhysicalVolume{
		PhysicalVolume: &pb.PhysicalVolume{
			StorageNodeId: id,
		},
	})
	if err != nil {
		log.Error("failed to get physical volumes", zap.Error(err))
		return nil, errors.Join(baseErr, err)
	}

	return lo.Filter(maps.Values(vols), func(item *PhysicalVolume, index int) bool {
		return item.StorageNodeId == node.Id
	}), nil
}

// GetVirtualVolume retrieve VirtualVolume by id.
func (m *Manager) GetVirtualVolume(ctx context.Context, id uint64) (*VirtualVolume, error) {
	return store.DoGetValueByKey[VirtualVolume](ctx, m.metaStore, &VirtualVolume{
		VirtualVolume: &pb.VirtualVolume{
			Id: id,
		},
	})
}

// GetPhysicalVolume retrieve PhysicalVolume by id.
func (m *Manager) GetPhysicalVolume(ctx context.Context, id uint64) (*PhysicalVolume, error) {
	ctx, span := tracing.Tracer.Start(ctx, "get physical volume")
	defer span.End()
	return store.DoGetValueByKey[PhysicalVolume](ctx, m.metaStore, &PhysicalVolume{
		PhysicalVolume: &pb.PhysicalVolume{
			VirtualVolumeId: id,
		},
	})
}

// CreateVirtualVolume just create a virtual volume record in the metadata server.
func (m *Manager) CreateVirtualVolume(ctx context.Context, capInByte uint64) (*VirtualVolume, error) {
	vol := &VirtualVolume{
		VirtualVolume: &pb.VirtualVolume{
			Id:        uint64(NextSnowID()),
			CapByte:   capInByte,
			CreatedAt: uint64(time.Now().Unix()),
		},
	}
	if err := store.DoSaveInKVPair[VirtualVolume](ctx, m.metaStore, vol); err != nil {
		log.Error("failed to create virtual volume", zap.Error(err))
		return nil, err
	}
	return vol, nil
}

// CreatePhysicalVolume just creates a physical volume record in the metadata server.
func (m *Manager) CreatePhysicalVolume(ctx context.Context, req *pb.CreatePhysicalVolume_Request) (*PhysicalVolume, error) {
	ctx, span := tracing.Tracer.Start(ctx, "create physical volume")
	defer span.End()
	nodeID := req.StorageNodeId
	baseErr := fmt.Errorf("failed to create physical volume on node %d", nodeID)
	node, err := m.clusterManager.GetStorageNode(ctx, &pb.GetStorageNode_Request{Arg: &pb.GetStorageNode_Request_Id{Id: nodeID}})
	if err != nil {
		log.Error("failed to get storage node", zap.Error(err))
		return nil, errors.Join(baseErr, err)
	}

	vol := &PhysicalVolume{
		PhysicalVolume: &pb.PhysicalVolume{
			Id:            uint64(NextSnowID()),
			CreatedAt:     uint64(time.Now().Unix()),
			StorageNodeId: nodeID,
			Prefix:        req.Prefix,
			Type:          req.Type,
			CapByte:       lo.FromPtrOr(req.Quota, 40*1024*1024*1024),
			UsageByte:     0,
			Threhold:      lo.FromPtrOr(req.Threhold, 0.75),
		},
	}
	node.PhysicalVolumeIds = append(node.PhysicalVolumeIds, vol.Id)
	node.Cap += vol.CapByte
	if err := store.DoInTxn(ctx, m.metaStore, func(ctx context.Context, _ store.Transaction) error {
		return store.DoBatchSaveAnyStrictly(ctx, m.metaStore, []store.LossySerializer{vol, &cluster.StorageNode{StorageNode: node}})
	}); err != nil {
		log.Error("failed to create physical volume", zap.Error(err))
		return nil, errors.Join(baseErr, err)
	}
	return vol, nil
}

func (m *Manager) PatchPhysicalVolume(ctx context.Context, req *pb.PatchPhysicalVolume_Request) (*pb.PatchPhysicalVolume_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "patch physical volume")
	defer span.End()
	node, err := m.clusterManager.GetStorageNode(ctx, &pb.GetStorageNode_Request{Arg: &pb.GetStorageNode_Request_Id{Id: req.StorageNodeId}})
	if err != nil {
		log.Error("failed to get storage node", zap.Error(err))
		return nil, err
	}

	vols, _ := m.ListPhysicalVolumesInNode(ctx, req.StorageNodeId)
	for _, vol := range vols {
		fmt.Println("before patch", vol)
	}

	vol, _ := m.GetPhysicalVolume(ctx, req.Id)
	if vol != nil {
		return nil, nil
	}
	vol = &PhysicalVolume{
		PhysicalVolume: &pb.PhysicalVolume{
			Id:            req.Id,
			CreatedAt:     uint64(time.Now().Unix()),
			StorageNodeId: req.StorageNodeId,
			Prefix:        req.Prefix,
			Type:          req.Type,
			CapByte:       *req.Quota,
			UsageByte:     0,
			Threhold:      *req.Threhold,
		},
	}
	node.Cap += vol.CapByte
	node.PhysicalVolumeIds = append(node.PhysicalVolumeIds, vol.Id)
	if err := store.DoInTxn(ctx, m.metaStore, func(ctx context.Context, _ store.Transaction) error {
		return store.DoBatchSaveAnyStrictly(ctx, m.metaStore, []store.LossySerializer{vol, &cluster.StorageNode{StorageNode: node}})
	}); err != nil {
		log.Error("failed to create physical volume", zap.Error(err))
		return nil, err
	}
	vols, _ = m.ListPhysicalVolumesInNode(ctx, req.StorageNodeId)
	for _, vol := range vols {
		fmt.Println("after patch", vol)
	}
	return &pb.PatchPhysicalVolume_Response{PhysicalVolume: vol.PhysicalVolume}, nil
}

// BindPhysicalVolume binds a physical volume to a virtual volume.
func (m *Manager) BindPhysicalVolume(ctx context.Context, v VirtualVolumeID, p PhysicalVolumeID) error {
	baseErr := fmt.Errorf("failed to bind physical volume %d to virtual volume %d", p, v)
	vVol, err := m.GetVirtualVolume(ctx, v)
	if err != nil {
		return errors.Join(baseErr, err)
	}
	if lo.Contains(vVol.PhysicalVolumeIds, p) {
		return errors.Join(baseErr, fmt.Errorf("physical volume %d is already bound to virtual volume %d", p, v))
	}
	pVol, err := m.GetPhysicalVolume(ctx, p)
	if err != nil {
		return errors.Join(baseErr, err)
	}
	if pVol.VirtualVolumeId != 0 {
		return errors.Join(baseErr, fmt.Errorf("physical volume %d is already bound to virtual volume %d", p, pVol.VirtualVolumeId))
	}
	vVol.PhysicalVolumeIds = append(vVol.PhysicalVolumeIds, p)
	pVol.VirtualVolumeId = v
	if err := store.DoBatchSaveAnyStrictly(ctx, m.metaStore, []store.LossySerializer{vVol, pVol}); err != nil {
		return errors.Join(baseErr, err)
	}
	return nil
}

// UnbindPhysicalVolume unbinds a physical volume from a virtual volume.
func (m *Manager) UnbindPhysicalVolume(ctx context.Context, v VirtualVolumeID, p PhysicalVolumeID) error {
	baseErr := fmt.Errorf("failed to unbind physical volume %d from virtual volume %d", p, v)
	vVol, err := m.GetVirtualVolume(ctx, v)
	if err != nil {
		return errors.Join(baseErr, err)
	}
	if !lo.Contains(vVol.PhysicalVolumeIds, p) {
		return errors.Join(baseErr, fmt.Errorf("physical volume %d is not bound to virtual volume %d", p, v))
	}
	pVol, err := m.GetPhysicalVolume(ctx, p)
	if err != nil {
		return errors.Join(baseErr, err)
	}
	if pVol.VirtualVolumeId != v {
		return errors.Join(baseErr, fmt.Errorf("physical volume %d is not bound to virtual volume %d", p, v))
	}
	lo.Filter(vVol.PhysicalVolumeIds, func(id uint64, _ int) bool {
		return id == v
	})
	pVol.VirtualVolumeId = 0
	if err := store.DoBatchSaveAnyStrictly(ctx, m.metaStore, []store.LossySerializer{vVol, pVol}); err != nil {
		return errors.Join(baseErr, err)
	}
	return nil
}

var (
	_ store.Serializer[VirtualVolume]  = (*VirtualVolume)(nil)
	_ store.Serializer[PhysicalVolume] = (*PhysicalVolume)(nil)
)

// VirtualVolume is a virtual volume that is a collection of physical volumes.
type VirtualVolume struct {
	*pb.VirtualVolume
	store.UnimplementedSerializer
}

func (v *VirtualVolume) FormatKey() string {
	return fmt.Sprintf("%s%d", v.FormatPrefix(), v.Id)
}
func (v *VirtualVolume) FormatPrefix() string {
	return "virtual_volume/"
}
func (v *VirtualVolume) Serialize() ([]byte, error) {
	return proto.Marshal(v)
}
func (v *VirtualVolume) Deserialize(bytes []byte) (*VirtualVolume, error) {
	var out VirtualVolume
	if err := proto.Unmarshal(bytes, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (v *VirtualVolume) Self() *VirtualVolume {
	return v
}

// PhysicalVolume is a physical volume that is bound to a virtual volume.
type PhysicalVolume struct {
	*pb.PhysicalVolume
	store.UnimplementedSerializer
}

func (p *PhysicalVolume) FormatKey() string {
	return fmt.Sprintf("%s%d", p.FormatPrefix(), p.Id)
}
func (p *PhysicalVolume) FormatPrefix() string {
	return "physical_volume/"
}
func (p *PhysicalVolume) Serialize() ([]byte, error) {
	return proto.Marshal(p)
}
func (p *PhysicalVolume) Deserialize(bytes []byte) (*PhysicalVolume, error) {
	var out = PhysicalVolume{PhysicalVolume: &pb.PhysicalVolume{}}
	if err := proto.Unmarshal(bytes, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (p *PhysicalVolume) Self() *PhysicalVolume {
	return p
}
