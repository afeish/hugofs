package heartbeat

import (
	"context"

	meta "github.com/afeish/hugo/pb/meta"
)

func (m *Manager) ProcessBeatRequest(ctx context.Context, req *meta.HeartBeat_Request) error {
	_, err := m.clusterSvc.PatchStorageNode(ctx, &meta.PatchStorageNode_Request{Id: req.NodeId, PhysicalVolumeUsage: req.Reports.PhysicalVolumeUsage})
	return err
}
