package server

import (
	"context"
	"io/fs"

	"github.com/afeish/hugo/global"
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/meta/cluster"
	"github.com/afeish/hugo/pkg/meta/consensus"
	"github.com/afeish/hugo/pkg/meta/event"
	"github.com/afeish/hugo/pkg/meta/inode"
	"github.com/afeish/hugo/pkg/meta/storage_meta"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util"
	_ "github.com/afeish/hugo/pkg/util/grace"
	"github.com/afeish/hugo/pkg/util/grpc_util"

	"github.com/pingcap/log"
	"github.com/samber/mo"
	"go.uber.org/zap"
	"google.golang.org/grpc/reflection"
)

var (
	_ pb.RawNodeServiceServer = (*Server)(nil)
	_ pb.EventServiceServer   = (*Server)(nil)
	_ pb.StorageMetaServer    = (*Server)(nil)
	_ pb.ClusterServiceServer = (*Server)(nil)
)

type Server struct {
	o *MetaOption
	// grpc
	*grpc_util.BaseGrpcServer
	nodeService        *inode.Service
	eventService       *event.Service
	storageMetaService *storage_meta.Service
	clusterService     *cluster.Service

	scheduler *consensus.Scheduler
}

func newServer(ctx context.Context, o *MetaOption) (*Server, error) {
	s, err := store.GetStore(o.DBName, IfOr(o.DBArg == "", mo.None[string](), mo.Some(o.DBArg)))
	if err != nil {
		return nil, err
	}
	var nodeID StorageNodeID // TODO: get nodeID from history or generating it
	metaService, err := storage_meta.NewService(ctx, nodeID, s)
	if err != nil {
		return nil, err
	}
	inodeService, err := inode.NewService(ctx, s)
	if err != nil {
		return nil, err
	}
	eventService, err := event.NewService(s)
	if err != nil {
		return nil, err
	}

	clusterService, err := cluster.NewService(s)
	if err != nil {
		return nil, err
	}
	log.Info("open db", zap.String("name", o.DBName), zap.String("arg", o.DBArg))

	return (&Server{
		o:                  o,
		storageMetaService: metaService,
		nodeService:        inodeService,
		eventService:       eventService,
		clusterService:     clusterService,
	}), nil
}

func (s *Server) init() *Server {
	defer func() {
		if err := recover(); err != nil {
			log.Error("server error", zap.Any("err", err))
		}
	}()
	for _, e := range []struct {
		name string
		mode fs.FileMode
		size uint64
	}{
		{
			name: ".Trash",
			mode: util.MakeMode(0777, fs.ModeDir),
			size: 4096,
		},
		{
			name: ".Trash-1000",
			mode: util.MakeMode(0644, fs.ModeDir),
			size: 4096,
		},
		{
			name: ".stats",
			mode: util.MakeMode(0444, fs.ModeDir),
			size: 4096,
		},
		{
			name: ".accesslog",
			mode: util.MakeMode(0400, fs.ModeDir),
			size: 4096,
		},
		{
			name: "hello.txt",
			mode: util.MakeMode(0755, 0),
			size: 0,
		},
	} {
		if _, err := s.nodeService.CreateInode(context.TODO(), &pb.CreateInode_Request{
			ParentIno: 1,
			ItemName:  e.name,
			Attr: &pb.ModifyAttr{
				Mode:  (*uint32)(&e.mode),
				Size:  Ptr(e.size),
				Nlink: Ptr(uint32(1)),
			},
		}); err != nil {
			log.Warn(err.Error())
		}
	}
	return s
}
func (s *Server) Start(ctx context.Context) error {
	s.BaseGrpcServer = grpc_util.NewBaseGrpcServer(ctx)

	// raft section
	cfg := GetEnvCfg()
	s.scheduler = consensus.Run(ctx, cfg.Meta.Raft.Dir, s.o.ID, s.o.Addr(), cfg.Meta.Raft.Peers, s.o.Bootstrap, s.RpcServer)

	s.init()

	pb.RegisterRawNodeServiceServer(s.RpcServer, s)
	pb.RegisterEventServiceServer(s.RpcServer, s)
	pb.RegisterClusterServiceServer(s.RpcServer, s)
	pb.RegisterStorageMetaServer(s.RpcServer, s)
	reflection.Register(s.RpcServer)

	if s.o.testMode {
		go s.BaseGrpcServer.ServeListener(s.o.listener)
		return nil
	}
	go s.BaseGrpcServer.Serve(s.o.Addr())
	if global.GetEnvCfg().Tracing.TraceEnabled {
		go grpc_util.StartPyroscopeServerAddr("", s.o.PyroscopeServerAddr)
	}
	go grpc_util.StartPprof(s.o.PprofAddr)
	return nil
}

func (s *Server) Wait() error {
	// s.scheduler.Stop()
	return s.BaseGrpcServer.Wait()
}
func (s *Server) IfExistsInode(ctx context.Context, req *pb.IfExistsInode_Request) (*pb.IfExistsInode_Response, error) {
	return s.nodeService.IfExistsInode(ctx, req)
}
func (s *Server) GetInodeAttr(ctx context.Context, req *pb.GetInodeAttr_Request) (*pb.GetInodeAttr_Response, error) {
	return s.nodeService.GetInodeAttr(ctx, req)
}
func (s *Server) Lookup(ctx context.Context, req *pb.Lookup_Request) (*pb.Lookup_Response, error) {
	return s.nodeService.Lookup(ctx, req)
}
func (s *Server) ListItemUnderInode(ctx context.Context, req *pb.ListItemUnderInode_Request) (*pb.ListItemUnderInode_Response, error) {
	return s.nodeService.ListItemUnderInode(ctx, req)
}
func (s *Server) UpdateInodeAttr(ctx context.Context, req *pb.UpdateInodeAttr_Request) (*pb.UpdateInodeAttr_Response, error) {
	return s.nodeService.UpdateInodeAttr(ctx, req)
}
func (s *Server) DeleteInode(ctx context.Context, req *pb.DeleteInode_Request) (*pb.DeleteInode_Response, error) {
	return s.nodeService.DeleteInode(ctx, req)
}
func (s *Server) DeleteDirInode(ctx context.Context, req *pb.DeleteDirInode_Request) (*pb.DeleteDirInode_Response, error) {
	return s.nodeService.DeleteDirInode(ctx, req)
}
func (s *Server) CreateInode(ctx context.Context, req *pb.CreateInode_Request) (*pb.CreateInode_Response, error) {
	return s.nodeService.CreateInode(ctx, req)
}
func (s *Server) Link(ctx context.Context, req *pb.Link_Request) (*pb.Link_Response, error) {
	return s.nodeService.Link(ctx, req)
}
func (s *Server) Rename(ctx context.Context, req *pb.Rename_Request) (*pb.Rename_Response, error) {
	return s.nodeService.Rename(ctx, req)
}
func (s *Server) PullLatest(ctx context.Context, request *pb.PullLatest_Request) (*pb.PullLatest_Response, error) {
	return s.eventService.PullLatest(ctx, request)
}

func (s *Server) LoadConfiguration(ctx context.Context, request *pb.LoadConfiguration_Request) (*pb.LoadConfiguration_Response, error) {
	return s.storageMetaService.LoadConfiguration(ctx, request)
}
func (s *Server) Statistics(ctx context.Context, request *pb.Statistics_Request) (*pb.Statistics_Response, error) {
	return s.storageMetaService.Statistics(ctx, request)
}
func (s *Server) BeginTransaction(ctx context.Context, request *pb.BeginTransaction_Request) (*pb.BeginTransaction_Response, error) {
	return s.storageMetaService.BeginTransaction(ctx, request)
}
func (s *Server) CommitTransaction(ctx context.Context, request *pb.CommitTransaction_Request) (*pb.CommitTransaction_Response, error) {
	return s.storageMetaService.CommitTransaction(ctx, request)
}
func (s *Server) RollbackTransaction(ctx context.Context, request *pb.RollbackTransaction_Request) (*pb.RollbackTransaction_Response, error) {
	return s.storageMetaService.RollbackTransaction(ctx, request)
}
func (s *Server) GetPhysicalVolume(ctx context.Context, request *pb.GetPhysicalVolume_Request) (*pb.GetPhysicalVolume_Response, error) {
	return s.storageMetaService.GetPhysicalVolume(ctx, request)
}
func (s *Server) GetVirtualVolume(ctx context.Context, request *pb.GetVirtualVolume_Request) (*pb.GetVirtualVolume_Response, error) {
	return s.storageMetaService.GetVirtualVolume(ctx, request)
}
func (s *Server) ListPhysicalVolumes(ctx context.Context, request *pb.ListPhysicalVolumes_Request) (*pb.ListPhysicalVolumes_Response, error) {
	return s.storageMetaService.ListPhysicalVolumes(ctx, request)
}
func (s *Server) ListVirtualVolume(ctx context.Context, request *pb.ListVirtualVolume_Request) (*pb.ListVirtualVolume_Response, error) {
	return s.storageMetaService.ListVirtualVolume(ctx, request)
}
func (s *Server) GetInoBlockMap(ctx context.Context, request *pb.GetInoBlockMap_Request) (*pb.GetInoBlockMap_Response, error) {
	return s.storageMetaService.GetInoBlockMap(ctx, request)
}
func (s *Server) GetHistoryBlockMap(ctx context.Context, request *pb.GetInoBlockMap_Request) (*pb.GetInoBlockMap_Response, error) {
	return s.storageMetaService.GetHistoryBlockMap(ctx, request)
}
func (s *Server) GetInoBlockMeta(ctx context.Context, request *pb.GetInoBlockMeta_Request) (*pb.GetInoBlockMeta_Response, error) {
	return s.storageMetaService.GetInoBlockMeta(ctx, request)
}
func (s *Server) UpdateBlockLocation(ctx context.Context, request *pb.UpdateBlockLocation_Request) (*pb.UpdateBlockLocation_Response, error) {
	return s.storageMetaService.UpdateBlockLocation(ctx, request)
}
func (s *Server) BatchUpdateBlockLocation(ctx context.Context, request *pb.BatchUpdateBlockLocation_Request) (*pb.BatchUpdateBlockLocation_Response, error) {
	return s.storageMetaService.BatchUpdateBlockLocation(ctx, request)
}
func (s *Server) CreatePhysicalVolume(ctx context.Context, request *pb.CreatePhysicalVolume_Request) (*pb.CreatePhysicalVolume_Response, error) {
	return s.storageMetaService.CreatePhysicalVolume(ctx, request)
}
func (s *Server) PatchPhysicalVolume(ctx context.Context, request *pb.PatchPhysicalVolume_Request) (*pb.PatchPhysicalVolume_Response, error) {
	return s.storageMetaService.PatchPhysicalVolume(ctx, request)
}
func (s *Server) CreateVirtualVolume(ctx context.Context, request *pb.CreateVirtualVolume_Request) (*pb.CreateVirtualVolume_Response, error) {
	return s.storageMetaService.CreateVirtualVolume(ctx, request)
}
func (s *Server) CreateReadOnlyVirtualVolume(ctx context.Context, request *pb.CreateReadOnlyVirtualVolume_Request) (*pb.CreateReadOnlyVirtualVolume_Response, error) {
	return s.storageMetaService.CreateReadOnlyVirtualVolume(ctx, request)
}
func (s *Server) BindPhysicalVolToVirtualVol(ctx context.Context, request *pb.BindPhysicalVolToVirtualVol_Request) (*pb.BindPhysicalVolToVirtualVol_Response, error) {
	return s.storageMetaService.BindPhysicalVolToVirtualVol(ctx, request)
}
func (s *Server) UnBindPhysicalVolToVirtualVol(ctx context.Context, request *pb.UnBindPhysicalVolToVirtualVol_Request) (*pb.UnBindPhysicalVolToVirtualVol_Response, error) {
	return s.storageMetaService.UnBindPhysicalVolToVirtualVol(ctx, request)
}
func (s *Server) HeartBeat(server pb.StorageMeta_HeartBeatServer) error {
	return s.storageMetaService.HeartBeat(server)
}
func (s *Server) GetCandidateStorage(ctx context.Context, request *pb.GetCandidateStorage_Request) (*pb.GetCandidateStorage_Response, error) {
	return s.clusterService.GetCandidateStorage(ctx, request)
}
func (s *Server) AddStorageNode(ctx context.Context, request *pb.AddStorageNode_Request) (*pb.AddStorageNode_Response, error) {
	return s.clusterService.AddStorageNode(ctx, request)
}
func (s *Server) GetStorageNode(ctx context.Context, request *pb.GetStorageNode_Request) (*pb.GetStorageNode_Response, error) {
	node, err := s.clusterService.GetStorageNode(ctx, request)
	if err != nil {
		return nil, err
	}
	return &pb.GetStorageNode_Response{
		StorageNode: node,
	}, nil
}

func (s *Server) ListStorageNodes(ctx context.Context, request *pb.ListStorageNodes_Request) (*pb.ListStorageNodes_Response, error) {
	nodes, err := s.clusterService.ListStorageNodes(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListStorageNodes_Response{
		StorageNodes: nodes,
	}, nil
}
func (s *Server) PatchStorageNode(ctx context.Context, req *pb.PatchStorageNode_Request) (*pb.PatchStorageNode_Response, error) {
	return s.clusterService.PatchStorageNode(ctx, req)
}

func (s *Server) AcquireFileLock(ctx context.Context, request *pb.AcquireFileLock_Request) (*pb.AcquireFileLock_Request, error) {
	return s.nodeService.AcquireFileLock(ctx, request)
}
func (s *Server) GetFileLock(ctx context.Context, request *pb.GetFileLock_Request) (*pb.GetFileLock_Response, error) {
	return s.nodeService.GetFileLock(ctx, request)
}
func (s *Server) DelFileLock(ctx context.Context, request *pb.DelFileLock_Request) (*pb.DelFileLock_Response, error) {
	return s.nodeService.DelFileLock(ctx, request)
}
func (s *Server) GetFileLocks(ctx context.Context, request *pb.GetFileLocks_Request) (*pb.GetFileLocks_Response, error) {
	return s.nodeService.GetFileLocks(ctx, request)
}
