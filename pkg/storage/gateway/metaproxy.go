package gateway

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore

	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/pingcap/log"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var (
	ErrNoSuchVolume = fmt.Errorf("no such volume")
	ErrNoSuchNode   = fmt.Errorf("no such node")
)

type MetaProxy struct {
	MetaSelector    MetaSelector
	metaHealthChgCh chan struct{}

	adaptor.MetaServiceClient

	cfg *adaptor.Config

	mu               sync.RWMutex
	volumeNodeCache  map[uint64]uint64
	nodeAddrCache    map[uint64]string
	nodeTcpAddrCache map[uint64]string
	nodes            map[uint64]*StorageNode
	ctx              context.Context

	closech ClosableCh
}

func NewMetaProxy(ctx context.Context, cfg *adaptor.Config) *MetaProxy {
	mp := &MetaProxy{
		metaHealthChgCh:  make(chan struct{}),
		ctx:              ctx,
		cfg:              cfg,
		volumeNodeCache:  make(map[uint64]uint64),
		nodeAddrCache:    make(map[uint64]string),
		nodeTcpAddrCache: make(map[uint64]string),
		nodes:            make(map[uint64]*pb.StorageNode),
		closech:          NewClosableCh(),
	}
	if cfg.Test.Enabled {
		mp.MetaSelector = NewMockMetaSelector()
	} else {
		mp.MetaSelector = NewDefaultMetaSelector()
	}
	if err := mp.doSelect(ctx); err != nil {
		log.Error("select meta", zap.Error(err))
	}
	if err := mp.updateCacheInternal(ctx); err != nil {
		log.Error("update meta cache", zap.Error(err))
	}

	go mp.updateCache(ctx)
	go mp.keepMetaAlive(ctx)
	return mp
}

func (m *MetaProxy) Ready() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.volumeNodeCache) > 2
}

func (m *MetaProxy) GetNode(volId uint64) (*StorageNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodeId, ok := m.volumeNodeCache[volId]
	if !ok {
		return nil, ErrNoSuchVolume
	}
	addr, ok := m.nodeAddrCache[nodeId]
	if !ok {
		return nil, ErrNoSuchNode
	}
	host, _port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := net.LookupPort("", _port)
	if err != nil {
		return nil, err
	}
	tcpaddr, ok := m.nodeTcpAddrCache[nodeId]
	if !ok {
		return nil, ErrNoSuchNode
	}
	_, _tcpport, err := net.SplitHostPort(tcpaddr)
	if err != nil {
		return nil, err
	}
	tcpport, err := net.LookupPort("", _tcpport)
	if err != nil {
		return nil, err
	}
	node := &StorageNode{
		Ip:      host,
		Port:    uint64(port),
		Id:      nodeId,
		TcpPort: uint64(tcpport),
	}

	return node, nil
}

func (m *MetaProxy) trackUsage(ctx context.Context, nodeId uint64, usage uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	node, loaded := m.nodes[nodeId]
	if !loaded {
		return
	}
	node.Usage += usage
}

// GetTopNPlus returns the top N+1 storage nodes based on their usage and capacity.
//
// ctx is the context for the function call.
// n is the number of nodes to return.
// Returns a slice of pointers to StorageNode structs and an error if no nodes are found.

func (m *MetaProxy) GetTopNPlus(ctx context.Context, replica int) ([]*StorageNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := replica + 1
	if len(m.nodes) < n {
		return nil, ErrNoSuchNode
	}
	nodes := lo.Values(m.nodes)
	sort.SliceStable(nodes, func(i, j int) bool {
		return int(nodes[j].Cap-nodes[j].Usage)-int(nodes[i].Cap-nodes[i].Usage) < 0
	})

	return nodes[:n], nil
}

func (m *MetaProxy) updateCache(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.updateCacheInternal(ctx); err != nil {
				log.Error("err refresh node-volume cache", zap.Error(err))
			}
		case <-m.closech.ClosedCh():
			return
		}
	}
}

func (m *MetaProxy) updateCacheInternal(ctx context.Context) error {
	r1, err := m.LoadConfiguration(ctx, &pb.LoadConfiguration_Request{})
	if err != nil {
		return err
	}
	volumeNodeCache := make(map[uint64]uint64)
	for _, v := range r1.PhysicalVolumes {
		volumeNodeCache[v.Id] = v.StorageNodeId
	}

	m.mu.Lock()
	m.volumeNodeCache = volumeNodeCache
	m.mu.Unlock()

	r2, err := m.ListStorageNodes(ctx, &pb.ListStorageNodes_Request{})
	if err != nil {
		return err
	}

	m.mu.Lock()
	for _, s := range r2.StorageNodes {
		m.nodeAddrCache[s.Id] = fmt.Sprintf("%s:%d", s.Ip, s.Port)
		m.nodeTcpAddrCache[s.Id] = fmt.Sprintf("%s:%d", s.Ip, s.TcpPort)
		n, found := m.nodes[s.Id]
		if !found || s.Usage > n.Usage { // prefer the local usage
			m.nodes[s.Id] = s
		}
	}
	m.mu.Unlock()
	return nil
}

func (m *MetaProxy) doSelect(ctx context.Context) error {
	metaClient, err := m.MetaSelector.Select(ctx, m.cfg)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.MetaServiceClient = metaClient
	m.mu.Unlock()
	return nil
}

func (m *MetaProxy) keepMetaAlive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
		case <-m.metaHealthChgCh:
			if err := m.doSelect(ctx); err != nil {
				log.Error("select meta", zap.Error(err))
			}
		case <-m.closech.ClosedCh():
			return
		}

	}
}

func (m *MetaProxy) Shutdown() {
	m.closech.Close(func() {
		close(m.metaHealthChgCh)

		m.MetaSelector.Close()
	})
}
