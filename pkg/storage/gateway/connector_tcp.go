package gateway

import (
	"context"
	"sync"

	"github.com/afeish/hugo/pkg/storage/adaptor"
)

var _ PeerConnector = (*StorageTcpConnector)(nil)

type StorageTcpConnector struct {
	mu             sync.RWMutex
	storageClients map[int]*adaptor.StorageClientTcpImpl
}

func NewStorageTcpConnector() PeerConnector {
	r := &StorageTcpConnector{
		storageClients: make(map[int]*adaptor.StorageClientTcpImpl),
	}
	return r
}

func (s *StorageTcpConnector) ConnectToPeer(ctx context.Context, peerId int, addr, tcpAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.storageClients[peerId]; !ok {
		client, err := adaptor.NewStorageClientTcpImpl(ctx, addr, tcpAddr)
		if err != nil {
			return err
		}
		s.storageClients[peerId] = client
	}
	return nil
}

// DisconnectPeer disconnects this server from the peer identified by peerId.
func (s *StorageTcpConnector) DisconnectPeer(ctx context.Context, peerId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.storageClients[peerId]
	if !ok {
		return nil
	}
	delete(s.storageClients, peerId)
	if err := c.Grpc.Close(); err != nil {
		return err
	}
	c.Tcp.Close()
	return nil
}

func (s *StorageTcpConnector) Get(ctx context.Context, peerId int) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.storageClients[peerId]
	if !ok {
		return nil, ErrClientNotFound
	}
	return c, nil
}
