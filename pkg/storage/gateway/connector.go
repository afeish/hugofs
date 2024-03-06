package gateway

import (
	"context"
	"errors"
	"sync"

	"github.com/afeish/hugo/pkg/storage/adaptor"
)

var (
	ErrClientNotFound = errors.New("client not found")
)
var _ PeerConnector = (*StorageConnector)(nil)

type StorageConnector struct {
	mu             sync.RWMutex
	storageClients map[int]*adaptor.StorageClientImpl
}

func NewStorageConnector() PeerConnector {
	r := &StorageConnector{
		storageClients: make(map[int]*adaptor.StorageClientImpl),
	}
	return r
}

func (s *StorageConnector) ConnectToPeer(ctx context.Context, peerId int, addr, tcpAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.storageClients[peerId]; !ok {
		client, err := adaptor.NewStorageClientImpl(ctx, addr)
		if err != nil {
			return err
		}
		s.storageClients[peerId] = client
	}
	return nil
}

// DisconnectPeer disconnects this server from the peer identified by peerId.
func (s *StorageConnector) DisconnectPeer(ctx context.Context, peerId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.storageClients[peerId]
	if !ok {
		return nil
	}
	delete(s.storageClients, peerId)
	if err := c.Close(); err != nil {
		return err
	}
	return nil
}

func (s *StorageConnector) Get(ctx context.Context, peerId int) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.storageClients[peerId]
	if !ok {
		return nil, nil
	}
	return c, nil
}
