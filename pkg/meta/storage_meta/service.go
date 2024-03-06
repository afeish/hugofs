package storage_meta

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/meta/cluster"
	"github.com/afeish/hugo/pkg/meta/storage_meta/block_map"
	"github.com/afeish/hugo/pkg/meta/storage_meta/heartbeat"
	"github.com/afeish/hugo/pkg/meta/storage_meta/volume"
	"github.com/afeish/hugo/pkg/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pingcap/log"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var _ pb.StorageMetaServer = (*Service)(nil)

type Service struct {
	selfNodeID      StorageNodeID
	metaStore       store.Store
	volumeManager   *volume.Manager
	blockMapManager *block_map.InoBlockMapManager
	beatManager     *heartbeat.Manager
}

func NewService(ctx context.Context, nodeID StorageNodeID, s store.Store) (*Service, error) {
	clusterSvc, err := cluster.NewService(s)
	if err != nil {
		return nil, err
	}
	vm, err := volume.NewManager(clusterSvc, s)
	if err != nil {
		return nil, err
	}
	bm, err := block_map.NewInoBlockMapManager(s)
	if err != nil {
		return nil, err
	}
	beater, err := heartbeat.NewManager(clusterSvc, s)
	if err != nil {
		return nil, err
	}

	return &Service{
		selfNodeID:      nodeID,
		metaStore:       s,
		volumeManager:   vm,
		blockMapManager: bm,
		beatManager:     beater,
	}, nil
}

func (s *Service) LoadConfiguration(ctx context.Context, request *pb.LoadConfiguration_Request) (*pb.LoadConfiguration_Response, error) {
	vvols, err := s.volumeManager.ListVirtualVolumes(ctx)
	if err != nil {
		log.Error("failed to get virtual volumes", zap.Error(err))
		return nil, err
	}
	pvols, err := s.volumeManager.ListPhysicalVolumes(ctx)
	if err != nil {
		log.Error("failed to get physical volumes", zap.Error(err))
		return nil, err
	}

	return &pb.LoadConfiguration_Response{
		VirtualVolumes: lo.Map(vvols, func(item *volume.VirtualVolume, index int) *pb.VirtualVolume {
			return item.VirtualVolume
		}),
		PhysicalVolumes: lo.Map(pvols, func(item *volume.PhysicalVolume, index int) *pb.PhysicalVolume {
			return item.PhysicalVolume
		}),
	}, nil
}
func (s *Service) Statistics(ctx context.Context, request *pb.Statistics_Request) (*pb.Statistics_Response, error) {
	var r pb.Statistics_Response
	pvols, err := s.volumeManager.ListPhysicalVolumes(ctx)
	if err != nil {
		log.Error("failed to get physical volumes", zap.Error(err))
		return nil, err
	}

	for _, p := range pvols {
		r.Cap += p.CapByte
		r.Usage += p.UsageByte
	}

	return &r, nil
}
func (s *Service) BeginTransaction(ctx context.Context, request *pb.BeginTransaction_Request) (*pb.BeginTransaction_Response, error) {
	return nil, nil
}
func (s *Service) CommitTransaction(ctx context.Context, request *pb.CommitTransaction_Request) (*pb.CommitTransaction_Response, error) {
	return &pb.CommitTransaction_Response{}, nil
}
func (s *Service) RollbackTransaction(ctx context.Context, request *pb.RollbackTransaction_Request) (*pb.RollbackTransaction_Response, error) {
	return &pb.RollbackTransaction_Response{}, nil
}
func (s *Service) GetPhysicalVolume(ctx context.Context, request *pb.GetPhysicalVolume_Request) (*pb.GetPhysicalVolume_Response, error) {
	physicalVolume, err := s.volumeManager.GetPhysicalVolume(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return &pb.GetPhysicalVolume_Response{
		PhysicalVolume: physicalVolume.PhysicalVolume,
	}, nil
}
func (s *Service) GetVirtualVolume(ctx context.Context, request *pb.GetVirtualVolume_Request) (*pb.GetVirtualVolume_Response, error) {
	virtualVolume, err := s.volumeManager.GetVirtualVolume(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return &pb.GetVirtualVolume_Response{
		VirtualVolume: virtualVolume.VirtualVolume,
	}, nil
}
func (s *Service) ListPhysicalVolumes(ctx context.Context, request *pb.ListPhysicalVolumes_Request) (*pb.ListPhysicalVolumes_Response, error) {
	physicalVolumes, err := s.volumeManager.ListPhysicalVolumesInNode(ctx, request.GetStorageNodeId())
	if err != nil {
		return nil, err
	}
	return &pb.ListPhysicalVolumes_Response{
		PhysicalVolumes: lo.Map(physicalVolumes, func(item *volume.PhysicalVolume, index int) *pb.PhysicalVolume {
			return item.PhysicalVolume
		}),
	}, nil
}
func (s *Service) ListVirtualVolume(ctx context.Context, request *pb.ListVirtualVolume_Request) (*pb.ListVirtualVolume_Response, error) {
	virtualVolumes, err := s.volumeManager.ListVirtualVolumes(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListVirtualVolume_Response{
		VirtualVolumes: lo.Map(virtualVolumes, func(item *volume.VirtualVolume, index int) *pb.VirtualVolume {
			return item.VirtualVolume
		}),
	}, nil
}
func (s *Service) GetInoBlockMap(ctx context.Context, request *pb.GetInoBlockMap_Request) (*pb.GetInoBlockMap_Response, error) {
	bm, err := s.blockMapManager.GetInoBlockMap(ctx, request.Ino)
	if err != nil {
		return nil, err
	}

	return &pb.GetInoBlockMap_Response{
		VirtualVolumeId: bm.VirtualVolumeID,
		InoBlockMap: &pb.InoBlockMap{
			Ino: request.Ino,
			Map: lo.MapValues(bm.Blocks, func(item *block_map.BlockMeta, _ uint64) *pb.BlockMeta {
				return item.BlockMeta
			}),
		},
	}, nil
}
func (s *Service) GetHistoryBlockMap(ctx context.Context, request *pb.GetInoBlockMap_Request) (*pb.GetInoBlockMap_Response, error) {
	bm, err := s.blockMapManager.GetHistoryBlockMap(ctx, request.Ino)
	if err != nil {
		return nil, err
	}

	return &pb.GetInoBlockMap_Response{
		VirtualVolumeId: bm.VirtualVolumeID,
		InoBlockMap: &pb.InoBlockMap{
			Ino: request.Ino,
			Map: lo.MapValues(bm.Blocks, func(item *block_map.BlockMeta, _ uint64) *pb.BlockMeta {
				return item.BlockMeta
			}),
		},
	}, nil
}
func (s *Service) GetInoBlockMeta(ctx context.Context, request *pb.GetInoBlockMeta_Request) (*pb.GetInoBlockMeta_Response, error) {
	bm, err := s.blockMapManager.GetInoBlockMeta(ctx, request.Ino, request.Index)
	if err != nil {
		return nil, err
	}
	return &pb.GetInoBlockMeta_Response{
		BlockMeta: bm.BlockMeta,
	}, nil
}
func (s *Service) UpdateBlockLocation(ctx context.Context, request *pb.UpdateBlockLocation_Request) (*pb.UpdateBlockLocation_Response, error) {
	out := block_map.BlockMetaFromPB(request.Meta)
	out.CreatedAt = uint64(time.Now().Unix())
	oldMeta, err := s.blockMapManager.GetInoBlockMeta(ctx, out.Ino, out.BlockIndexInFile)
	if err == nil && oldMeta != nil {
		if err = s.blockMapManager.ArchiveBlockMeta(ctx, oldMeta); err != nil {
			log.Info(fmt.Sprintf("3333 ArchiveBlockMeta ok error(%v)", err))
			return nil, err
		}
	}
	if err := s.blockMapManager.UpdateInoBlockMap(ctx, out); err != nil {
		log.Info(fmt.Sprintf("3333 UpdateInoBlockMap ok error(%v)", err))
		return nil, err
	}

	return &pb.UpdateBlockLocation_Response{}, nil
}
func (s *Service) BatchUpdateBlockLocation(ctx context.Context, request *pb.BatchUpdateBlockLocation_Request) (*pb.BatchUpdateBlockLocation_Response, error) {
	if err := s.blockMapManager.BatchUpdateInoBlockMap(ctx, lo.Map(request.Metas, func(item *pb.BlockMeta, index int) *block_map.BlockMeta {
		return block_map.BlockMetaFromPB(item)
	})); err != nil {
		return nil, err
	}

	return &pb.BatchUpdateBlockLocation_Response{}, nil
}
func (s *Service) CreatePhysicalVolume(ctx context.Context, request *pb.CreatePhysicalVolume_Request) (*pb.CreatePhysicalVolume_Response, error) {
	// TODO: fixme, make size of physical volume can be different
	physicalVolume, err := s.volumeManager.CreatePhysicalVolume(ctx, request)
	if err != nil {
		return nil, err
	}
	return &pb.CreatePhysicalVolume_Response{
		PhysicalVolume: physicalVolume.PhysicalVolume,
	}, nil
}
func (s *Service) PatchPhysicalVolume(ctx context.Context, req *pb.PatchPhysicalVolume_Request) (*pb.PatchPhysicalVolume_Response, error) {
	return s.volumeManager.PatchPhysicalVolume(ctx, req)
}
func (s *Service) CreateVirtualVolume(ctx context.Context, request *pb.CreateVirtualVolume_Request) (*pb.CreateVirtualVolume_Response, error) {
	virtualVolume, err := s.volumeManager.CreateVirtualVolume(ctx, request.Cap)
	if err != nil {
		return nil, err
	}
	return &pb.CreateVirtualVolume_Response{
		VirtualVolume: virtualVolume.VirtualVolume,
	}, nil
}
func (s *Service) CreateReadOnlyVirtualVolume(ctx context.Context, request *pb.CreateReadOnlyVirtualVolume_Request) (*pb.CreateReadOnlyVirtualVolume_Response, error) {
	//TODO implement me
	panic("implement me")
}
func (s *Service) BindPhysicalVolToVirtualVol(ctx context.Context, request *pb.BindPhysicalVolToVirtualVol_Request) (*pb.BindPhysicalVolToVirtualVol_Response, error) {
	if err := s.volumeManager.BindPhysicalVolume(ctx, request.VirtualVolumeId, request.PhysicalVolumeId); err != nil {
		return nil, err
	}

	return &pb.BindPhysicalVolToVirtualVol_Response{}, nil
}
func (s *Service) UnBindPhysicalVolToVirtualVol(ctx context.Context, request *pb.UnBindPhysicalVolToVirtualVol_Request) (*pb.UnBindPhysicalVolToVirtualVol_Response, error) {
	if err := s.volumeManager.UnbindPhysicalVolume(ctx, request.VirtualVolumeId, request.PhysicalVolumeId); err != nil {
		return nil, err
	}

	return &pb.UnBindPhysicalVolToVirtualVol_Response{}, nil
}
func (s *Service) HeartBeat(server pb.StorageMeta_HeartBeatServer) error {
	for {
		req, err := server.Recv()
		if err != nil {
			if strings.Contains(err.Error(), context.Canceled.Error()) {
				return nil
			}
			log.Error("recv heartbeat", zap.Error(err))
			return nil
		}
		if err = s.beatManager.ProcessBeatRequest(context.Background(), req); err != nil {
			log.Error("recv heartbeat", zap.Error(err))
		}
	}
}
func (s *Service) GetCandidateStorage(context.Context, *pb.GetCandidateStorage_Request) (*pb.GetCandidateStorage_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCandidateStorage not implemented")
}

func (s *Service) PatchStorageNode(ctx context.Context, req *pb.PatchStorageNode_Request) (*pb.PatchStorageNode_Response, error) {
	err := s.beatManager.ProcessBeatRequest(ctx, &pb.HeartBeat_Request{NodeId: req.Id, Reports: &pb.HeartBeat_Request_UsageReport{PhysicalVolumeUsage: req.PhysicalVolumeUsage}})
	return nil, err
}
