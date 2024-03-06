package gateway

import (
	"context"
	"sync"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/node"
	"github.com/pingcap/log"
	"go.uber.org/zap"
)

type NodeSelector interface {
	Select(context.Context, *StorageNode) (adaptor.StorageClient, error)
	Closable
}

var _ NodeSelector = (*MockNodeSelector)(nil)

type MockNodeSelector struct {
	sync.RWMutex
	cache map[string]adaptor.StorageClient
}

func NewMockNodeSelector() NodeSelector {
	return &MockNodeSelector{
		cache: make(map[string]adaptor.StorageClient),
	}
}

func (s *MockNodeSelector) Select(ctx context.Context, n *StorageNode) (adaptor.StorageClient, error) {
	s.Lock()
	defer s.Unlock()
	cfg := node.NewConfig()
	cfg.IP = n.Ip
	cfg.Port = uint16(n.Port)
	s.cache[cfg.GetAddr()] = NewStorageClientMock(ctx, cfg)
	return s.cache[cfg.GetAddr()], nil
}

func (s *MockNodeSelector) Close(cbs ...func() error) (err error) {
	s.Lock()
	defer s.Unlock()
	for _, client := range s.cache {
		if err = client.Close(cbs...); err != nil {
			return
		}
	}
	return nil
}

var _ NodeSelector = (*DefaultNodeSelector)(nil)

type DefaultNodeSelector struct {
	connector PeerConnector
	mu        sync.RWMutex
}

func NewDefaultNodeSelector() NodeSelector {
	return &DefaultNodeSelector{
		connector: NewStorageConnector(),
	}
}

func NewTcpNodeSelector() NodeSelector {
	return &DefaultNodeSelector{
		connector: NewStorageTcpConnector(),
	}
}

func (s *DefaultNodeSelector) Select(ctx context.Context, n *StorageNode) (adaptor.StorageClient, error) {
	var peerId = int(n.Id)
	s.mu.RLock()
	c, err := s.connector.Get(ctx, peerId)
	if c != nil && err == nil {
		s.mu.RUnlock()
		return c.(adaptor.StorageClient), nil
	}
	s.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.connector.ConnectToPeer(ctx, peerId, StorageAddr(n), StorageTcpAddr(n)); err != nil {
		return nil, err
	}
	c, err = s.connector.Get(ctx, peerId)
	if err != nil {
		return nil, err
	}
	r := c.(adaptor.StorageClient)
	return r, nil
}

func (s *DefaultNodeSelector) Close(...func() error) error {
	return nil
}

type DefaultMetaServiceClient struct {
}

type MetaSelector interface {
	Select(context.Context, *adaptor.Config) (adaptor.MetaServiceClient, error)
	adaptor.Healthy
	Closable
}

type MockMetaSelector struct {
	sync.RWMutex
	cache map[string]adaptor.StorageClient
}

var _ MetaSelector = (*MockMetaSelector)(nil)

func NewMockMetaSelector() MetaSelector {
	return &MockMetaSelector{cache: map[string]adaptor.StorageClient{}}
}

func (s *MockMetaSelector) Select(ctx context.Context, cfg *adaptor.Config) (adaptor.MetaServiceClient, error) {
	return node.NewMetaClientMock(ctx, cfg), nil
}

func (s *MockMetaSelector) IsHealthy(ctx context.Context) bool {

	return true
}

func (s *MockMetaSelector) Close(cbs ...func() error) (err error) {
	s.Lock()
	defer s.Unlock()
	for _, client := range s.cache {
		if err = client.Close(cbs...); err != nil {
			return
		}
	}
	return nil
}

type DefaultMetaSelector struct {
	mu     sync.RWMutex
	client *adaptor.MetaServiceClientImpl
}

var _ MetaSelector = (*DefaultMetaSelector)(nil)

func NewDefaultMetaSelector() MetaSelector {
	return &DefaultMetaSelector{}
}

func (s *DefaultMetaSelector) Select(ctx context.Context, cfg *adaptor.Config) (adaptor.MetaServiceClient, error) {
	s.mu.RLock()
	if s.client != nil {
		if s.client.IsHealthy(ctx) {
			s.mu.RUnlock()
			return s.client, nil
		}
		if err := s.client.Close(); err != nil {
			log.Error("shutdown meta client", zap.Error(err))
		}
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	client, err := adaptor.NewMetaServiceClientImpl(ctx, cfg.MetaAddrs)
	if err != nil {
		return nil, err
	}
	s.client = client
	return client, nil
}

func (s *DefaultMetaSelector) IsHealthy(ctx context.Context) bool {
	return true
}

func (s *DefaultMetaSelector) Close(...func() error) error {
	return nil
}
