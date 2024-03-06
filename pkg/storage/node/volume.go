package node

import (
	"context"
	"time"

	meta "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/storage/volume"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var _ storage.VolumeServiceServer = (*Node)(nil)

func (v *Node) MountVolume(ctx context.Context, req *storage.MountVolume_Request) (*storage.MountVolume_Response, error) {
	eng, err := engine.ParseEngineType(req.Type.String())
	if err != nil {
		return nil, err
	}
	if _, err := store.DoGetValueByKey[volume.ConfigQuery](ctx, v.store, &volume.ConfigQuery{
		Engine: eng,
		Prefix: req.Prefix,
	}); err == nil {
		v.lg.Warn("volume already registered", zap.String(VOL_PREFIX, req.Prefix))
		return &storage.MountVolume_Response{NodeId: v.ID}, nil
	}

	cfg := &volume.Config{
		Engine:    eng,
		Prefix:    req.Prefix,
		Id:        req.Id,
		Quota:     lo.ToPtr(size.SizeSuffix(*req.Quota)),
		Threhold:  req.Threhold,
		BlockSize: v.cfg.BlockSize,
		DB:        v.cfg.DB,
		CreatedAt: uint64(time.Now().Unix()),
	}

	if err = v.ObtionVolumeId(ctx, cfg); err != nil {
		return nil, err
	}
	if err = v.StartVolume(ctx, cfg); err != nil {
		v.lg.Error("failed to start volume", zap.String(VOL_PREFIX, req.Prefix))
		return nil, err
	}

	return &storage.MountVolume_Response{NodeId: v.ID}, nil
}

func (v *Node) ObtionVolumeId(ctx context.Context, cfg *volume.Config) error {
	// try obtion the physical volume id first
	res, err := v.storageMetaClient.CreatePhysicalVolume(ctx, &meta.CreatePhysicalVolume_Request{
		StorageNodeId: v.ID,
		Prefix:        cfg.Prefix,
		Quota:         lo.ToPtr(uint64(*cfg.Quota)),
		Threhold:      cfg.Threhold,
		Type:          meta.EngineType(meta.EngineType_value[string(cfg.Engine)]),
	})
	if err != nil {
		v.lg.Error("failed to register physical volume", zap.Error(err))
		return err
	}
	cfg.Id = res.PhysicalVolume.Id
	return nil
}

func (v *Node) GetVolume(ctx context.Context, req *storage.GetVolume_Request) (*storage.GetVolume_Response, error) {
	var (
		id uint64
	)
	if req.GetQuery() != nil {
		eng, err := engine.ParseEngineType(req.GetQuery().GetType().String())
		if err != nil {
			return nil, err
		}
		cfgQuery, err := store.DoGetValueByKey[volume.ConfigQuery](ctx, v.store, &volume.ConfigQuery{
			Engine: eng,
			Prefix: req.GetQuery().GetPrefix(),
		})
		if err != nil {
			return nil, err
		}
		id = cfgQuery.ID
	} else {
		id = req.GetId()
	}

	cfg, err := store.DoGetValueByKey[volume.Config](ctx, v.store, &volume.Config{
		Id: id,
	})
	if err != nil {
		return nil, err
	}

	volStat := cfg.ToPhysicalVolume()
	if volStat != nil {
		volStat.NodeId = v.ID
	}
	volStat.InuseBytes = lo.ToPtr(v.selector.GetVolStat(cfg.Id)).GetInUseBytes()
	return &storage.GetVolume_Response{
		Volume: volStat,
	}, nil
}
func (v *Node) ListVolumes(ctx context.Context, req *storage.ListVolumes_Request) (*storage.ListVolumes_Response, error) {
	cfgMap, err := store.DoGetValuesByPrefix[volume.Config](ctx, v.store, &volume.Config{})
	if err != nil {
		return nil, err
	}

	return &storage.ListVolumes_Response{Volumes: lo.Map(lo.Values(cfgMap), func(cfg *volume.Config, _ int) *storage.PhysicalVolume {
		volStat := cfg.ToPhysicalVolume()
		if volStat != nil {
			volStat.NodeId = v.ID
			volStat.InuseBytes = lo.ToPtr(v.selector.GetVolStat(cfg.Id)).GetInUseBytes()
		}
		return volStat
	})}, nil
}
