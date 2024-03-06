package gateway

import (
	"context"
	"sync"

	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/node"
	"google.golang.org/grpc"
)

var _ adaptor.StorageClient = (*StorageServiceClientMock)(nil)

var (
	_storageMockFactory = newStorageMockFactory()
)

func NewStorageClientMock(ctx context.Context, cfg *node.Config) adaptor.StorageClient {
	return _storageMockFactory.newStorageClientMock(ctx, cfg)
}

func ResetStorageClientMock() {
	_storageMockFactory.reset()
}

type storageMockFactory struct {
	mu    sync.RWMutex
	cache map[string]adaptor.StorageClient
}

func newStorageMockFactory() storageMockFactory {
	return storageMockFactory{cache: make(map[string]adaptor.StorageClient)}
}

func (f *storageMockFactory) newStorageClientMock(ctx context.Context, cfg *node.Config) adaptor.StorageClient {
	f.mu.Lock()
	defer f.mu.Unlock()
	addr := cfg.GetAddr()
	val := f.cache[addr]
	if val != nil {
		return val
	}
	delegate, err := node.NewNode(ctx, cfg)
	if err != nil {
		panic(err)
	}
	err = delegate.Start(ctx)
	if err != nil {
		panic(err)
	}
	val = &StorageServiceClientMock{
		delegate: delegate,
	}
	f.cache[addr] = val
	return val
}

func (f *storageMockFactory) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache = make(map[string]adaptor.StorageClient)
}

type StorageServiceClientMock struct {
	delegate *node.Node
}

func (c *StorageServiceClientMock) Read(ctx context.Context, in *storage.Read_Request, opts ...grpc.CallOption) (*storage.Read_Response, error) {
	return c.delegate.Read(ctx, in)

}
func (c *StorageServiceClientMock) StreamRead(ctx context.Context, in *storage.Read_Request, opts ...grpc.CallOption) (storage.StorageService_StreamReadClient, error) {
	return nil, nil
}
func (c *StorageServiceClientMock) Write(ctx context.Context, in *storage.WriteBlock_Request, opts ...grpc.CallOption) (*storage.WriteBlock_Response, error) {
	return c.delegate.Write(ctx, in)
}
func (c *StorageServiceClientMock) StreamWrite(ctx context.Context, opts ...grpc.CallOption) (storage.StorageService_StreamWriteClient, error) {
	return nil, nil
}
func (c *StorageServiceClientMock) DeleteBlock(ctx context.Context, in *storage.DeleteBlock_Request, opts ...grpc.CallOption) (*storage.DeleteBlock_Response, error) {
	return c.delegate.DeleteBlock(ctx, in)
}
func (c *StorageServiceClientMock) ListBlocks(ctx context.Context, in *storage.ListBlocks_Request, opts ...grpc.CallOption) (*storage.ListBlocks_Response, error) {
	return c.delegate.ListBlocks(ctx, in)
}

func (c *StorageServiceClientMock) EvictBlock(ctx context.Context, in *storage.EvictBlock_Request, opts ...grpc.CallOption) (*storage.EvictBlock_Response, error) {
	return c.delegate.EvictBlock(ctx, in)
}

func (c *StorageServiceClientMock) RecycleSpace(ctx context.Context, in *storage.RecycleSpace_Request, opts ...grpc.CallOption) (*storage.RecycleSpace_Response, error) {
	return c.delegate.RecycleSpace(ctx, in)
}

func (c *StorageServiceClientMock) Warmup(ctx context.Context, req *storage.Warmup_Request, opts ...grpc.CallOption) (*storage.Warmup_Response, error) {
	return c.delegate.Warmup(ctx, req)
}

func (c *StorageServiceClientMock) MountVolume(ctx context.Context, in *storage.MountVolume_Request, opts ...grpc.CallOption) (*storage.MountVolume_Response, error) {
	return c.delegate.MountVolume(ctx, in)
}
func (c *StorageServiceClientMock) GetVolume(ctx context.Context, in *storage.GetVolume_Request, opts ...grpc.CallOption) (*storage.GetVolume_Response, error) {
	return c.delegate.GetVolume(ctx, in)
}
func (c *StorageServiceClientMock) ListVolumes(ctx context.Context, in *storage.ListVolumes_Request, opts ...grpc.CallOption) (*storage.ListVolumes_Response, error) {
	return c.delegate.ListVolumes(ctx, in)
}

func (c *StorageServiceClientMock) IsHealthy(ctx context.Context) bool {
	return true
}

func (c *StorageServiceClientMock) Close(...func() error) error {
	return c.delegate.Stop(context.Background())
}

func (c *StorageServiceClientMock) String() string {
	return "StorageServiceClientMock"
}
