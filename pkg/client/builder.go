package client

import (
	"context"
	"strings"
	"time"

	_ "github.com/Jille/grpc-multi-resolver"
	"github.com/afeish/hugo/global"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pingcap/log"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	"go.uber.org/zap"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Option struct {
	Addr         string
	Creds        credentials.TransportCredentials
	Conn         *grpc.ClientConn
	Kind         string
	TraceEnabled bool
}
type Builder[O any, T GrpcClient[O]] struct {
	o *Option
}

func NewBuilder[O any, T GrpcClient[O]]() *Builder[O, T] {
	return &Builder[O, T]{
		o: &Option{},
	}
}
func (b *Builder[O, T]) SetAddr(addr string) *Builder[O, T] {
	b.o.Addr = addr
	return b
}
func (b *Builder[O, T]) SetCreds(creds credentials.TransportCredentials) *Builder[O, T] {
	b.o.Creds = creds
	return b
}
func (b *Builder[O, T]) SetConn(conn *grpc.ClientConn) *Builder[O, T] {
	b.o.Conn = conn
	return b
}
func (b *Builder[O, T]) Build(ctx context.Context) (r O, err error) {
	if b.o.Conn == nil {
		var conn *grpc.ClientConn
		conn, err = NewGrpcConn(ctx, b.o)
		if err != nil {
			return
		}
		b.o.Conn = conn
	}

	var tmp T
	return tmp.newClient(b.o.Conn), nil
}

type GrpcClient[O any] interface {
	newClient(conn *grpc.ClientConn) O
}

func NewGrpcConn(ctx context.Context, o *Option) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// serviceConfig := fmt.Sprintf(`{"healthCheckConfig": {"serviceName": "%s"}, "loadBalancingConfig": [ { "round_robin": {} } ]}`, o.Kind)
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100 * time.Millisecond)),
		grpc_retry.WithMax(5),
	}

	dailOptions := []grpc.DialOption{
		func() grpc.DialOption {
			if o.Creds != nil {
				return grpc.WithTransportCredentials(o.Creds)
			} else {
				return grpc.WithInsecure() //lint:ignore SA1019 ignore
			}
		}(),
		// grpc.WithDefaultServiceConfig(serviceConfig),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(global.GrpcMaxRMsgSize),
			grpc.MaxCallSendMsgSize(global.GrpcMaxSMsgSize)),
	}
	if o.TraceEnabled {
		dailOptions = append(dailOptions, []grpc.DialOption{
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpts...)),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
		}...)
	}

	if !strings.HasSuffix(o.Addr, "multi:///") {
		o.Addr = "multi:///" + o.Addr
	}
	conn, err := grpc.DialContext(ctx, o.Addr, dailOptions...)

	if err != nil {
		log.Error("err connect", zap.Any("addr", o.Addr), zap.Error(err))
		return nil, err
	}
	return conn, nil
}
