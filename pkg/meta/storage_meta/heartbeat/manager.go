package heartbeat

import (
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/meta/cluster"
	"github.com/afeish/hugo/pkg/store"

	"github.com/buraksezer/connpool"
)

// Manager is responsible for managing heartbeat connections and heartbeat events.
type Manager struct {
	metaStore  store.Store
	pool       map[StorageNodeID]*connpool.Pool
	clusterSvc *cluster.Service
}

func NewManager(clusterSvc *cluster.Service, metaStore store.Store) (*Manager, error) {
	clusterSvc, err := cluster.NewService(metaStore)
	if err != nil {
		return nil, err
	}
	return &Manager{
		metaStore:  metaStore,
		pool:       make(map[StorageNodeID]*connpool.Pool),
		clusterSvc: clusterSvc,
	}, nil
}
