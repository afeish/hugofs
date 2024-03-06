package node

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/afeish/hugo/global"
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/client"
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/util/size"

	vol "github.com/afeish/hugo/pkg/storage/volume"
	"github.com/afeish/hugo/pkg/store"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/pingcap/log"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/samber/mo"
)

var (
	_ storage.StorageServiceServer = (*Node)(nil)
)

type NodeImpl interface {
}

func NewNodeImpl(node *Node) NodeImpl {
	return node
}

// Node represents a node in the storage cluster. Its duty is to manage a collection of physical volumes.
type Node struct {
	ID StorageNodeID

	clusterClient     pb.ClusterServiceClient
	storageMetaClient pb.StorageMetaClient
	beater            pb.StorageMeta_HeartBeatClient

	mu sync.RWMutex

	cfg       *Config
	selector  *Selector
	messenger *vol.Messenger
	lg        *zap.Logger

	store store.Store

	objectSyncer *ObjectSyncer

	ctx     context.Context
	cancel  context.CancelFunc
	closech ClosableCh
}

func NewNode(ctx context.Context, cfg *Config) (*Node, error) {
	if cfg == nil {
		return nil, errors.New("nil cfg")
	}
	if cfg != nil {
		if err := cfg.Must(); err != nil {
			return nil, err
		}
	}
	lg := cfg.Lg
	if lg == nil {
		lg = global.GetLogger().Named("node")
	} else {
		lg = cfg.Lg.Named("node")
	}

	lg.Debug("open db", zap.String("name", cfg.DB.Name), zap.String("arg", cfg.DB.Arg))
	s, err := store.GetStore(cfg.DB.Name, mo.Some(cfg.DB.Arg))
	if err != nil {
		return nil, err
	}
	v := &Node{
		ID:        cfg.ID,
		cfg:       cfg,
		messenger: vol.NewMessenger(),
		store:     s,
		closech:   NewClosableCh(),
		lg:        lg,
	}

	v.store = s

	return v, nil
}

func (v *Node) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	v.ctx = ctx
	v.cancel = cancel

	cfg := v.cfg
	if err := v.RegisterMyself(ctx); err != nil {
		return err
	}

	v.lg = v.lg.With(zap.Uint64(NODE_ID, v.ID))
	v.selector = NewSelector(ctx, v.store, v.messenger, v.lg) // selector must be created before start any volumes
	v.objectSyncer = NewObjectSyncer(ctx, v.selector, v.cfg.Object, v.lg)

	v.LoadRegisteredVolumes(ctx)

	if cfg != nil && cfg.Section != nil {
		for _, sec := range cfg.Section {
			c := &vol.Config{
				Prefix:    sec.Prefix,
				Engine:    sec.Engine,
				BlockSize: cfg.BlockSize,
				Quota:     sec.Quota,
				Threhold:  sec.Threhold,

				DB:     cfg.DB,
				NodeId: v.ID,
			}

			if err := v.ObtionVolumeId(ctx, c); err != nil {
				v.lg.Error("err obtion volume id, skip", zap.Error(err))
				continue
			}

			if err := v.StartVolume(ctx, c); err != nil {
				v.lg.Error("err register volume, skip", zap.Error(err))
				continue
			}
		}
	}

	var beater pb.StorageMeta_HeartBeatClient

	if cfg.Test.Enabled {
		beater = NewMetaClientMock(ctx, cfg.ToAdaptorConfig())
	} else {
		beater, _ = v.storageMetaClient.HeartBeat(ctx)
	}

	v.beater = beater

	go v.sendReport(ctx)
	go v.recvInstructions(ctx)
	return nil
}

func (v *Node) RegisterMyself(ctx context.Context) error {
	var (
		conn *grpc.ClientConn
		err  error
		addr string
	)
	addr = v.cfg.MetaAddrs
	if !v.cfg.Test.Enabled {
		v.lg.Debug("try connect to meta-server", zap.String("meta-server", addr))
		conn, err = client.NewGrpcConn(ctx, &client.Option{Addr: addr, TraceEnabled: true, Kind: pb.ClusterService_ServiceDesc.ServiceName})
		if err != nil {
			return errors.WithStack(err)
		}

		v.lg.Debug("establish the connection", zap.String("meta-server", addr))
		v.clusterClient = pb.NewClusterServiceClient(conn)
		v.storageMetaClient = pb.NewStorageMetaClient(conn)
	} else {
		metaClient := NewMetaClientMock(ctx, v.cfg.ToAdaptorConfig())
		v.clusterClient = metaClient
		v.storageMetaClient = metaClient
	}

	addr = v.cfg.GetAddr()
	if n, _ := v.clusterClient.GetStorageNode(ctx, &pb.GetStorageNode_Request{Arg: &pb.GetStorageNode_Request_Address{Address: addr}}); n != nil {
		v.SetNodeId(n.StorageNode.Id)
		log.Info("node already registered", zap.Uint64("node", v.ID))
		return nil
	}

	res, err := v.clusterClient.AddStorageNode(ctx, &pb.AddStorageNode_Request{Ip: v.cfg.IP, Port: uint64(v.cfg.Port), TcpPort: uint64(v.cfg.TcpPort)})
	if err != nil {
		return err
	}
	v.SetNodeId(res.StorageNode.Id)
	return nil
}

func (v *Node) LoadRegisteredVolumes(ctx context.Context) error {
	res, err := v.storageMetaClient.ListPhysicalVolumes(ctx, &pb.ListPhysicalVolumes_Request{StorageNodeId: v.ID})
	if err != nil {
		v.lg.Error("err list physical volumes")
		return nil
	}
	for _, volume := range res.PhysicalVolumes {
		if v.selector.Exists(volume.Id) {
			continue
		}
		eng, err := engine.ParseEngineType(volume.Type.String())
		if err != nil {
			v.lg.Error("illegal engine", zap.Uint64(VOL_ID, volume.Id))
			continue
		}
		cfg := &vol.Config{
			Id:        volume.Id,
			NodeId:    v.ID,
			Engine:    eng,
			Prefix:    volume.Prefix,
			Quota:     lo.ToPtr(size.SizeSuffix(volume.CapByte)),
			Threhold:  &volume.Threhold,
			DB:        v.cfg.DB,
			CreatedAt: volume.CreatedAt,
			BlockSize: v.cfg.BlockSize,
		}

		if err := v.StartVolume(ctx, cfg); err != nil {
			v.lg.Error("start volume failed", zap.Uint64(VOL_ID, volume.Id))
			continue
		}
	}

	if err = v.recoverFromLocal(ctx); err != nil {
		v.lg.Error("recover from local config", zap.Error(err))
	}

	return nil
}

func (v *Node) StartVolume(ctx context.Context, cfg *vol.Config) error {
	volume, err := vol.NewLocalPhysicalVolume(ctx, cfg, v.messenger, v.lg)
	if err != nil {
		v.lg.Error("err new physical volume, skip", zap.Error(err))
		return err
	}

	err = store.DoInTxn(ctx, v.store, func(context.Context, store.Transaction) error {
		if err = store.DoSaveInKVPair[vol.Config](ctx, v.store, cfg); err != nil {
			return err
		}
		if err = store.DoSaveInKVPair[vol.ConfigQuery](ctx, v.store, &vol.ConfigQuery{Engine: cfg.Engine, Prefix: cfg.Prefix, ID: cfg.Id}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		v.lg.Error("err save volume config", zap.Error(err))
		return err
	}

	v.selector.Sync(cfg.Id, volume)
	return nil
}
func (v *Node) recoverFromLocal(ctx context.Context) error {
	m, err := store.DoGetValuesByPrefix[vol.Config](ctx, v.store, &vol.Config{})
	if err != nil {
		return err
	}
	for _, cfg := range lo.Values(m) {
		if !v.selector.Exists(cfg.Id) {
			v.lg.Warn("not found volume, patching ...", zap.Uint64(VOL_ID, cfg.Id))
			if _, err = v.storageMetaClient.PatchPhysicalVolume(ctx, &pb.PatchPhysicalVolume_Request{
				StorageNodeId: cfg.NodeId,
				Id:            cfg.Id,
				Prefix:        cfg.Prefix,
				Quota:         lo.ToPtr(uint64(*cfg.Quota)),
				Threhold:      cfg.Threhold,
				// Type:          meta.EngineType(meta.EngineType_value[string(cfg.Engine)]),
			}); err == nil {
				if err = v.StartVolume(ctx, cfg); err != nil {
					return err
				}
			} else {
				fmt.Println(err)
			}
		}
	}
	return nil
}

func (v *Node) SetNodeId(id StorageNodeID) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.ID = id
}

func (v *Node) GetAddr() string {
	return v.cfg.GetAddr()
}

func (v *Node) IsHealthy() bool {
	return true
}

func (v *Node) GetStorageMetaClient() pb.StorageMetaClient {
	return v.storageMetaClient
}

func (v *Node) sendReport(ctx context.Context) {
	// send a collections of physical volumes stats to meta
	interval := time.Second * 30
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := v.sendPhysicalVolumesStats(ctx); err != nil {
				v.lg.Error("failed to send physical volumes stats", zap.Error(err))
			}
			timer.Reset(interval)
		case <-v.closech.ClosedCh():
			return
		}
	}
}

func (v *Node) sendPhysicalVolumesStats(ctx context.Context) error {
	volStats := v.selector.GetVolStats()
	if len(volStats) == 0 {
		v.lg.Debug("no physical volume mounted")
		return nil
	}
	inusedMap := lo.SliceToMap(volStats, func(item vol.VolumeStatistics) (uint64, uint64) {
		return item.VolID, item.InuseBytes
	})
	v.lg.Debug("sending physical volume usage", zap.Any("usage", inusedMap))
	return v.beater.Send(&pb.HeartBeat_Request{
		NodeId: v.ID,
		Reports: &pb.HeartBeat_Request_UsageReport{
			PhysicalVolumeUsage: inusedMap,
		},
	})
}

func (v *Node) recvInstructions(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-v.ctx.Done():
			return v.ctx.Err()
		default:
			r, err := v.beater.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if r.GetGc() != nil {
				v.lg.Debug("recv gc", zap.String("gc", r.GetGc().String()))
			}
			if r.GetNotifications() != nil {
				v.lg.Debug("recv noti", zap.String("noti", r.GetNotifications().Type.String()))
			}
		}
	}

}

// CancelBlock make the block invalid
func (v *Node) CancelBlock(ctx context.Context, locations []*lo.Tuple2[PhysicalVolumeID, BlockID]) error {
	// it's ok if we failed here.
	// the invalid block cannot find on the Metadata Server.
	// we can let gc do it.
	panic("implement me")
}

func (v *Node) Stop(ctx context.Context) (err error) {
	v.closech.Close(func() {
		v.cancel()

		err = v.selector.Shutdown(ctx)
		if err != nil {
			return
		}

		err = v.store.Close()
	})
	return
}
