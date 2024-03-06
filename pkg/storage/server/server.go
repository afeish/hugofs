package server

import (
	"context"

	"github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pb/storage"
	pb "github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/node"
	"github.com/afeish/hugo/pkg/util/grpc_util"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

var (
	_ pb.StorageServiceServer = (*Server)(nil)
	_ pb.VolumeServiceServer  = (*Server)(nil)
)

type Server struct {
	*grpc_util.BaseGrpcServer

	o       *StorageOption
	service *node.Node
}

func newServer(ctx context.Context, o *StorageOption) (*Server, error) {
	ser, err := node.NewNode(ctx, o.ToCfg())
	if err != nil {
		return nil, err
	}
	return &Server{
		o:       o,
		service: ser,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.service.Start(ctx); err != nil {
		return err
	}
	s.BaseGrpcServer = grpc_util.NewBaseGrpcServer(ctx)
	pb.RegisterStorageServiceServer(s.RpcServer, s)
	pb.RegisterVolumeServiceServer(s.RpcServer, s)
	reflection.Register(s.RpcServer)

	go s.Serve(s.o.Addr())
	if global.GetEnvCfg().Tracing.TraceEnabled {
		go grpc_util.StartPyroscopeServerAddr("", s.o.PyroscopeServerAddr)
	}
	go grpc_util.StartPprof(s.o.PprofAddr)
	// tcp
	s.registerTcp(s.o, s.service)
	return nil
}

func (s *Server) Read(ctx context.Context, req *pb.Read_Request) (*pb.Read_Response, error) {
	return s.service.Read(s.Ctx, req)
}
func (s *Server) StreamRead(*pb.Read_Request, pb.StorageService_StreamReadServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamRead not implemented")
}
func (s *Server) Write(ctx context.Context, req *pb.WriteBlock_Request) (*pb.WriteBlock_Response, error) {
	return s.service.Write(ctx, req)
}
func (s *Server) StreamWrite(pb.StorageService_StreamWriteServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamWrite not implemented")
}
func (s *Server) DeleteBlock(context.Context, *pb.DeleteBlock_Request) (*pb.DeleteBlock_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteBlock not implemented")
}
func (s *Server) ListBlocks(ctx context.Context, req *pb.ListBlocks_Request) (*pb.ListBlocks_Response, error) {
	return s.service.ListBlocks(ctx, req)
}

func (s *Server) EvictBlock(ctx context.Context, req *pb.EvictBlock_Request) (*pb.EvictBlock_Response, error) {
	return s.service.EvictBlock(ctx, req)
}

func (s *Server) RecycleSpace(ctx context.Context, req *pb.RecycleSpace_Request) (*pb.RecycleSpace_Response, error) {
	return s.service.RecycleSpace(ctx, req)
}

func (s *Server) Warmup(ctx context.Context, req *pb.Warmup_Request) (*pb.Warmup_Response, error) {
	return s.service.Warmup(ctx, req)
}

func (s *Server) ListVolumes(ctx context.Context, req *pb.ListVolumes_Request) (*pb.ListVolumes_Response, error) {
	return s.service.ListVolumes(ctx, req)
}
func (s *Server) MountVolume(ctx context.Context, req *storage.MountVolume_Request) (*storage.MountVolume_Response, error) {
	return s.service.MountVolume(ctx, req)
}

func (s *Server) GetVolume(ctx context.Context, req *storage.GetVolume_Request) (*storage.GetVolume_Response, error) {
	return s.service.GetVolume(ctx, req)
}

func (s *Server) Stop() error {
	s.service.Stop(s.Ctx)
	s.Cancel()
	return nil
}
