package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/tracing"

	"github.com/samber/lo"
	"golang.org/x/exp/maps"
)

var (
	ErrNotEnoughNodes = errors.New("not enough nodes")

	ErrNotEnoughVolumes = errors.New("not enough volumes")
)

type Service struct {
	metaStore store.Store
}

func NewService(metaStore store.Store) (*Service, error) {
	return &Service{
		metaStore: metaStore,
	}, nil
}

func (m *Service) ListStorageNodes(ctx context.Context) ([]*pb.StorageNode, error) {
	ctx, span := tracing.Tracer.Start(ctx, "list storage nodes")
	defer span.End()
	nodes, err := store.DoGetValuesByPrefix[StorageNode](ctx, m.metaStore, &StorageNode{})
	if err != nil {
		return nil, err
	}
	return lo.Map(maps.Values(nodes), func(item *StorageNode, index int) *pb.StorageNode {
		return item.StorageNode
	}), nil
}
func (m *Service) GetStorageNode(ctx context.Context, req *pb.GetStorageNode_Request) (*pb.StorageNode, error) {
	var (
		id      uint64
		address string
		node    *StorageNode
		err     error
	)

	ctx, span := tracing.Tracer.Start(ctx, "get storage node")
	defer span.End()

	if req.GetId() != 0 {
		id = req.GetId()
	}
	if req.GetAddress() != "" {
		address = req.GetAddress()
		nodeAddr, err := store.DoGetValueByKey[NodeAddr](ctx, m.metaStore, &NodeAddr{
			Addr: address,
		})
		if err != nil {
			return nil, err
		}
		id = nodeAddr.Id
	}

	node, err = store.DoGetValueByKey[StorageNode](ctx, m.metaStore, &StorageNode{
		StorageNode: &pb.StorageNode{
			Id: id,
		},
	})
	if err != nil {
		return nil, err
	}
	return node.StorageNode, nil
}
func (m *Service) AddStorageNode(ctx context.Context, req *pb.AddStorageNode_Request) (*pb.AddStorageNode_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "add storage node")
	defer span.End()

	baseErr := fmt.Errorf("failed to add storage node %s:%d", req.Ip, req.Port)
	nodes, err := m.ListStorageNodes(ctx)
	if err != nil {
		return nil, errors.Join(baseErr, err)
	}

	id := uint64(NextSnowID())
	node := &StorageNode{
		StorageNode: &pb.StorageNode{
			Id:        id,
			Ip:        req.Ip,
			Port:      req.Port,
			TcpPort:   req.TcpPort,
			CreatedAt: uint64(time.Now().Unix()),
			UpdatedAt: uint64(time.Now().Unix()),
			Cap:       req.Cap,
			Peers: func() []StorageNodeID {
				if len(nodes) == 0 {
					return []StorageNodeID{id}
				}
				return append(nodes[0].Peers, id)
			}(),
		},
	}

	if err := store.DoInTxn(ctx, m.metaStore, func(ctx context.Context, _ store.Transaction) error {
		// add node
		if err := store.DoSaveInKVPair[StorageNode](ctx, m.metaStore, node); err != nil {
			return err
		}
		if err := store.DoSaveInKVPair[NodeAddr](ctx, m.metaStore, &NodeAddr{Addr: node.Addr(), Id: node.Id}); err != nil {
			return err
		}
		lo.ForEach(nodes, func(item *pb.StorageNode, index int) {
			item.Peers = append(item.Peers, node.Id)
		})
		// update peers
		return store.DoBatchSaveStrictly(ctx, m.metaStore, lo.Map(nodes, func(item *pb.StorageNode, index int) store.Serializer[StorageNode] {
			return &StorageNode{StorageNode: item}
		}))
	}); err != nil {
		return nil, errors.Join(baseErr, err)
	}

	return &pb.AddStorageNode_Response{StorageNode: node.StorageNode}, nil
}

func (m *Service) GetCandidateStorage(ctx context.Context, req *pb.GetCandidateStorage_Request) (*pb.GetCandidateStorage_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "get candidate storage nodes")
	defer span.End()
	nodes, err := m.ListStorageNodes(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) < 2 {
		return nil, ErrNotEnoughVolumes
	}
	nodes = SortR(nodes, func(n1 *pb.StorageNode, n2 *pb.StorageNode) int {
		return int(n2.Cap-n2.Usage) - int(n1.Cap-n1.Usage)
	})
	return &pb.GetCandidateStorage_Response{Primary: nodes[0], Secondary: nodes[1]}, nil
}

func (m *Service) PatchStorageNode(ctx context.Context, req *pb.PatchStorageNode_Request) (*pb.PatchStorageNode_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "patch storage node")
	defer span.End()
	node, err := store.DoGetValueByKey[StorageNode](ctx, m.metaStore, &StorageNode{
		StorageNode: &pb.StorageNode{
			Id: req.Id,
		},
	})
	if err != nil {
		return nil, err
	}
	node.Usage = lo.Sum(lo.Values(req.PhysicalVolumeUsage))
	// log.Debug("patch storage node to", zap.Uint64("node", node.Id), zap.String("cap", humanize.IBytes(node.Cap)), zap.String("usage", humanize.IBytes(node.Usage)))
	if err := store.DoSaveInKVPair[StorageNode](ctx, m.metaStore, node); err != nil {
		return nil, err
	}
	return &pb.PatchStorageNode_Response{StorageNode: node.StorageNode}, nil
}
