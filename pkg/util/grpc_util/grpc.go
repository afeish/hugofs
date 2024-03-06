package grpc_util

import (
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"time"

	"github.com/afeish/hugo/global"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	"github.com/pingcap/log"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

type BaseGrpcServer struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	// grpc
	Listener   net.Listener
	RpcServer  *grpc.Server
	RpcErrChan chan error
}

var (
	recoverHandler = func(p interface{}) (err error) {
		println(fmt.Sprintf("%T: %+v", p, p))
		println(fmt.Sprintf("%s\n", debug.Stack()))
		return status.Errorf(codes.Internal, "%v", p)
	}
)

func NewBaseGrpcServer(ctx context.Context) *BaseGrpcServer {
	s := &BaseGrpcServer{}
	s.Ctx, s.Cancel = context.WithCancel(ctx)
	s.RpcErrChan = make(chan error, 1)

	s.RpcServer = grpc.NewServer(
		grpc.ChainUnaryInterceptor(recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(recoverHandler)), otelgrpc.UnaryServerInterceptor()),
		grpc.ChainStreamInterceptor(
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(recoverHandler)),
			// otelgrpc.StreamServerInterceptor(),
		),
		grpc.MaxRecvMsgSize(global.GrpcMaxRMsgSize),
		grpc.MaxSendMsgSize(global.GrpcMaxSMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              5 * time.Second,
			Timeout:           1 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	return s
}
func (s *BaseGrpcServer) Serve(addr string, hooks ...func()) {
	var err error
	s.Listener, err = net.Listen("tcp", addr)
	if err != nil {
		s.RpcErrChan <- err
		close(s.RpcErrChan)
		return
	}

	for _, fn := range hooks {
		fn()
	}
	log.Info("starting grpc server on", zap.String("rpcAddr", addr))
	if err := s.RpcServer.Serve(s.Listener); err != nil {
		log.Error("failed to serve", zap.Error(err))
		s.RpcErrChan <- err
		close(s.RpcErrChan)
	}
}
func (s *BaseGrpcServer) ServeListener(listener net.Listener) {
	s.Listener = listener
	if err := s.RpcServer.Serve(listener); err != nil {
		log.Error("failed to serve", zap.Error(err))
		s.RpcErrChan <- err
		close(s.RpcErrChan)
	}
}
func (s *BaseGrpcServer) Wait() error {
	select {
	case <-s.Ctx.Done():
		return s.Ctx.Err()
	case err := <-s.RpcErrChan:
		return err
	}
}

func (s *BaseGrpcServer) Stop() error {
	s.Cancel()
	return nil
}
