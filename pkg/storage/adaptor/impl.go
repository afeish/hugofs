package adaptor

import (
	"context"
	"fmt"
	"sync"

	"github.com/afeish/hugo/global"
	meta "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/client"
	"github.com/afeish/hugo/pkg/tcp"
	"github.com/pingcap/log"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var _ MetaServiceClient = (*MetaServiceClientImpl)(nil)

type MetaServiceClientImpl struct {
	meta.RawNodeServiceClient
	meta.EventServiceClient
	meta.StorageMetaClient
	meta.ClusterServiceClient
	meta.StorageMeta_HeartBeatClient
	Healthy
	addrs string

	mu              sync.RWMutex
	conn            *grpc.ClientConn
	activeAddr      string
	metaHealthChgCh chan struct{}

	lg *zap.Logger
}

func NewMetaServiceClientImpl(ctx context.Context, addrs string) (*MetaServiceClientImpl, error) {
	m := &MetaServiceClientImpl{
		addrs:           addrs,
		metaHealthChgCh: make(chan struct{}),
		lg:              global.GetLogger().Named("connect"),
	}

	if err := m.create(ctx); err != nil {
		return nil, err
	}

	go m.checkHealth(ctx)
	return m, nil
}

func (m *MetaServiceClientImpl) IsHealthy(ctx context.Context) bool {
	return true
}

func (m *MetaServiceClientImpl) checkHealth(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-m.metaHealthChgCh:
			if err := m.create(ctx); err != nil {
				log.Error("err reconstruct meta client", zap.Error(err))
			}
		}
	}
}

func (m *MetaServiceClientImpl) create(ctx context.Context) error {
	var (
		conn *grpc.ClientConn
		err  error
	)
	addrs := m.addrs
	m.lg.Debug("try connect to meta-server", zap.String("meta-server", addrs))
	conn, err = client.NewGrpcConn(ctx, &client.Option{Addr: addrs, TraceEnabled: true, Kind: "meta"})
	if err != nil {
		return nil
	}

	m.lg.Debug("establish the connection", zap.String("meta-server", addrs))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EventServiceClient = meta.NewEventServiceClient(conn)
	m.RawNodeServiceClient = meta.NewRawNodeServiceClient(conn)
	m.ClusterServiceClient = meta.NewClusterServiceClient(conn)
	m.StorageMetaClient = meta.NewStorageMetaClient(conn)
	m.StorageMeta_HeartBeatClient, _ = m.StorageMetaClient.HeartBeat(ctx)
	m.conn = conn

	return nil
}

func (m *MetaServiceClientImpl) GetActiveAddr() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeAddr
}

func (m *MetaServiceClientImpl) Close(...func() error) error {
	return m.conn.Close()
}

var _ StorageClient = (*StorageClientImpl)(nil)

type StorageClientImpl struct {
	storage.StorageServiceClient
	storage.VolumeServiceClient
	Healthy

	addr string
	conn *grpc.ClientConn

	lg *zap.Logger
}

func NewStorageClientImpl(ctx context.Context, addr string) (*StorageClientImpl, error) {
	c := &StorageClientImpl{
		addr: addr,
		lg:   global.GetLogger().Named("connect"),
	}

	if err := c.Start(ctx); err != nil {
		c.lg.Error("start storage client", zap.Error(err))
		return nil, err
	}
	return c, nil
}

func (c *StorageClientImpl) Start(ctx context.Context) error {
	c.lg.Debug("start connect to storage", zap.String("addr", c.addr))
	conn, err := client.NewGrpcConn(ctx, &client.Option{Addr: c.addr, TraceEnabled: true, Kind: "storage"})
	if err != nil {
		return nil
	}
	c.StorageServiceClient = storage.NewStorageServiceClient(conn)
	c.VolumeServiceClient = storage.NewVolumeServiceClient(conn)
	c.conn = conn
	return nil
}

func (c *StorageClientImpl) IsHealthy(ctx context.Context) bool {
	return true
}

func (c *StorageClientImpl) Close(...func() error) error {
	return c.conn.Close()
}

func (c *StorageClientImpl) String() string {
	return fmt.Sprintf("StorageClientImpl(%s)", c.addr)
}

var _ StorageClient = (*StorageClientTcpImpl)(nil)

type StorageClientTcpImpl struct {
	Grpc *StorageClientImpl
	Tcp  tcp.TcpClient
}

func NewStorageClientTcpImpl(ctx context.Context, addr, tcpAddr string) (*StorageClientTcpImpl, error) {
	c := &StorageClientTcpImpl{
		Tcp: tcp.NewClient(ctx, tcpAddr),
	}
	var err error
	if c.Grpc, err = NewStorageClientImpl(ctx, addr); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *StorageClientTcpImpl) Read(ctx context.Context, in *storage.Read_Request, opts ...grpc.CallOption) (*storage.Read_Response, error) {
	return c.Tcp.StorageRead(ctx, in)
}

func (c *StorageClientTcpImpl) StreamRead(ctx context.Context, in *storage.Read_Request, opts ...grpc.CallOption) (storage.StorageService_StreamReadClient, error) {
	return c.Grpc.StreamRead(ctx, in)
}

func (c *StorageClientTcpImpl) Write(ctx context.Context, in *storage.WriteBlock_Request, opts ...grpc.CallOption) (*storage.WriteBlock_Response, error) {
	return c.Tcp.StorageWrite(ctx, in)
}

func (c *StorageClientTcpImpl) StreamWrite(ctx context.Context, opts ...grpc.CallOption) (storage.StorageService_StreamWriteClient, error) {
	return c.Grpc.StreamWrite(ctx)
}

func (c *StorageClientTcpImpl) DeleteBlock(ctx context.Context, in *storage.DeleteBlock_Request, opts ...grpc.CallOption) (*storage.DeleteBlock_Response, error) {
	return c.Grpc.DeleteBlock(ctx, in)
}

func (c *StorageClientTcpImpl) ListBlocks(ctx context.Context, in *storage.ListBlocks_Request, opts ...grpc.CallOption) (*storage.ListBlocks_Response, error) {
	return c.Grpc.ListBlocks(ctx, in)
}

func (c *StorageClientTcpImpl) EvictBlock(ctx context.Context, in *storage.EvictBlock_Request, opts ...grpc.CallOption) (*storage.EvictBlock_Response, error) {
	return c.Grpc.EvictBlock(ctx, in)
}

func (c *StorageClientTcpImpl) RecycleSpace(ctx context.Context, in *storage.RecycleSpace_Request, opts ...grpc.CallOption) (*storage.RecycleSpace_Response, error) {
	return c.Grpc.RecycleSpace(ctx, in)
}

func (c *StorageClientTcpImpl) Warmup(ctx context.Context, in *storage.Warmup_Request, opts ...grpc.CallOption) (*storage.Warmup_Response, error) {
	return c.Grpc.Warmup(ctx, in)
}

func (c *StorageClientTcpImpl) MountVolume(ctx context.Context, in *storage.MountVolume_Request, opts ...grpc.CallOption) (*storage.MountVolume_Response, error) {
	return c.Grpc.MountVolume(ctx, in)
}

func (c *StorageClientTcpImpl) GetVolume(ctx context.Context, in *storage.GetVolume_Request, opts ...grpc.CallOption) (*storage.GetVolume_Response, error) {
	return c.Grpc.GetVolume(ctx, in)
}

func (c *StorageClientTcpImpl) ListVolumes(ctx context.Context, in *storage.ListVolumes_Request, opts ...grpc.CallOption) (*storage.ListVolumes_Response, error) {
	return c.Grpc.ListVolumes(ctx, in)
}

func (c *StorageClientTcpImpl) IsHealthy(ctx context.Context) bool {
	return c.Grpc.IsHealthy(ctx)
}

func (c *StorageClientTcpImpl) String() string {
	return c.Grpc.String()
}

func (c *StorageClientTcpImpl) Close(cbs ...func() error) error {
	return c.Grpc.Close(cbs...)
}
