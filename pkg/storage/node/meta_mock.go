package node

import (
	"context"
	"sync"

	meta "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/meta/cluster"
	"github.com/afeish/hugo/pkg/meta/event"
	"github.com/afeish/hugo/pkg/meta/inode"
	"github.com/afeish/hugo/pkg/meta/storage_meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/store"

	"github.com/samber/mo"
	"google.golang.org/grpc"
)

var _ adaptor.MetaServiceClient = (*MetaClientMock)(nil)

var (
	_metaMockFactory = newMetaMockFactory()
)

func NewMetaClientMock(ctx context.Context, cfg *adaptor.Config) adaptor.MetaServiceClient {
	return _metaMockFactory.newMetaClientMock(ctx, cfg)
}

func ResetMetaClientMock() {
	_metaMockFactory.reset()
}

type metaMockFactory struct {
	mu    sync.RWMutex
	cache map[string]adaptor.MetaServiceClient
}

func newMetaMockFactory() metaMockFactory {
	return metaMockFactory{cache: make(map[string]adaptor.MetaServiceClient)}
}

func (f *metaMockFactory) newMetaClientMock(ctx context.Context, cfg *adaptor.Config) adaptor.MetaServiceClient {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cfg.GetTestAddr()
	if f.cache[key] != nil {
		return f.cache[key]
	}
	s, err := store.GetStore(cfg.Test.DBName, mo.Some(cfg.Test.DBArg))
	if err != nil {
		panic(err)
	}
	storage, err := storage_meta.NewService(ctx, 1, s)
	if err != nil {
		panic(err)
	}
	cluster, _ := cluster.NewService(s)
	if err != nil {
		panic(err)
	}
	node, err := inode.NewService(ctx, s)
	if err != nil {
		panic(err)
	}
	event, err := event.NewService(s)
	if err != nil {
		panic(err)
	}
	val := &MetaClientMock{
		key:     key,
		store:   s,
		storage: storage,
		cluster: cluster,
		node:    node,
		event:   event,
	}
	f.cache[key] = val
	return val
}

func (f *metaMockFactory) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache = make(map[string]adaptor.MetaServiceClient)
}

type MetaClientMock struct {
	grpc.ClientStream

	store   store.Store
	key     string
	event   *event.Service
	node    *inode.Service
	storage *storage_meta.Service
	cluster *cluster.Service
}

func (c *MetaClientMock) Close(...func() error) error {
	return c.store.Close()
}

// load storage configuration
func (c *MetaClientMock) LoadConfiguration(ctx context.Context, in *meta.LoadConfiguration_Request, opts ...grpc.CallOption) (*meta.LoadConfiguration_Response, error) {
	return c.storage.LoadConfiguration(ctx, in)
}

// Statistics returns usage.
func (c *MetaClientMock) Statistics(ctx context.Context, in *meta.Statistics_Request, opts ...grpc.CallOption) (*meta.Statistics_Response, error) {
	return c.storage.Statistics(ctx, in)
}

// ////////////////////////////////////////////////
// BeginTransaction will result the transaction_id increase.
// Make each virtual volume holds a separate transaction to avoid overflow.
func (c *MetaClientMock) BeginTransaction(ctx context.Context, in *meta.BeginTransaction_Request, opts ...grpc.CallOption) (*meta.BeginTransaction_Response, error) {
	return c.storage.BeginTransaction(ctx, in)
}

// CommitTransaction will commit the transaction to make the change visible.
// By removing the given transaction id from the active transaction list.
func (c *MetaClientMock) CommitTransaction(ctx context.Context, in *meta.CommitTransaction_Request, opts ...grpc.CallOption) (*meta.CommitTransaction_Response, error) {
	return c.storage.CommitTransaction(ctx, in)
}

// RollbackTransaction will rollback the transaction, by deleting the changes with the given transaction id.
func (c *MetaClientMock) RollbackTransaction(ctx context.Context, in *meta.RollbackTransaction_Request, opts ...grpc.CallOption) (*meta.RollbackTransaction_Response, error) {
	return c.storage.RollbackTransaction(ctx, in)
}

// ////////////////////////////////////////////////
// GetPhysicalVolume gets the physical volume by id. At present cannot know the history of the physical volume.
func (c *MetaClientMock) GetPhysicalVolume(ctx context.Context, in *meta.GetPhysicalVolume_Request, opts ...grpc.CallOption) (*meta.GetPhysicalVolume_Response, error) {
	return c.storage.GetPhysicalVolume(ctx, in)
}

// Get the virtual volume by id. At present cannot know the history of the virtual volume.
func (c *MetaClientMock) GetVirtualVolume(ctx context.Context, in *meta.GetVirtualVolume_Request, opts ...grpc.CallOption) (*meta.GetVirtualVolume_Response, error) {
	return c.storage.GetVirtualVolume(ctx, in)
}

// ListPhysicalVolumes lists all physical volumes.
func (c *MetaClientMock) ListPhysicalVolumes(ctx context.Context, in *meta.ListPhysicalVolumes_Request, opts ...grpc.CallOption) (*meta.ListPhysicalVolumes_Response, error) {
	return c.storage.ListPhysicalVolumes(ctx, in)
}

// ListVirtualVolume lists all virtual volumes.
func (c *MetaClientMock) ListVirtualVolume(ctx context.Context, in *meta.ListVirtualVolume_Request, opts ...grpc.CallOption) (*meta.ListVirtualVolume_Response, error) {
	return c.storage.ListVirtualVolume(ctx, in)
}

// ////////////////////////////////////////////////
// GetLatestInoBlockMap gets the block map of a file with a transaction id.
// If no transaction id is given, it will return the latest block map.
func (c *MetaClientMock) GetInoBlockMap(ctx context.Context, in *meta.GetInoBlockMap_Request, opts ...grpc.CallOption) (*meta.GetInoBlockMap_Response, error) {
	return c.storage.GetInoBlockMap(ctx, in)
}

func (c *MetaClientMock) GetHistoryBlockMap(ctx context.Context, in *meta.GetInoBlockMap_Request, opts ...grpc.CallOption) (*meta.GetInoBlockMap_Response, error) {
	return c.storage.GetHistoryBlockMap(ctx, in)
}

// GetInoBlockMeta gets the block metadata, with the transaction_id, we can have a history view of the file.
func (c *MetaClientMock) GetInoBlockMeta(ctx context.Context, in *meta.GetInoBlockMeta_Request, opts ...grpc.CallOption) (*meta.GetInoBlockMeta_Response, error) {
	return c.storage.GetInoBlockMeta(ctx, in)
}

// ////////////////////////////////////////////////
// UpdateBlockLocation updates the block location of a file.
func (c *MetaClientMock) UpdateBlockLocation(ctx context.Context, in *meta.UpdateBlockLocation_Request, opts ...grpc.CallOption) (*meta.UpdateBlockLocation_Response, error) {
	return c.storage.UpdateBlockLocation(ctx, in)
}

// BatchUpdateBlockLocation updates the block location of a file.
func (c *MetaClientMock) BatchUpdateBlockLocation(ctx context.Context, in *meta.BatchUpdateBlockLocation_Request, opts ...grpc.CallOption) (*meta.BatchUpdateBlockLocation_Response, error) {
	return c.storage.BatchUpdateBlockLocation(ctx, in)
}

// ////////////////////////////////////////////////
// CreatePhysicalVolume creates a physical volume.
func (c *MetaClientMock) CreatePhysicalVolume(ctx context.Context, in *meta.CreatePhysicalVolume_Request, opts ...grpc.CallOption) (*meta.CreatePhysicalVolume_Response, error) {
	return c.storage.CreatePhysicalVolume(ctx, in)
}

func (c *MetaClientMock) PatchPhysicalVolume(ctx context.Context, req *meta.PatchPhysicalVolume_Request, opts ...grpc.CallOption) (*meta.PatchPhysicalVolume_Response, error) {
	return c.storage.PatchPhysicalVolume(ctx, req)
}

// CreateVirtualVolume creates a virtual volume.
func (c *MetaClientMock) CreateVirtualVolume(ctx context.Context, in *meta.CreateVirtualVolume_Request, opts ...grpc.CallOption) (*meta.CreateVirtualVolume_Response, error) {
	return c.storage.CreateVirtualVolume(ctx, in)
}

// CreateReadOnlyVirtualVolume creates a read only virtual volume.
func (c *MetaClientMock) CreateReadOnlyVirtualVolume(ctx context.Context, in *meta.CreateReadOnlyVirtualVolume_Request, opts ...grpc.CallOption) (*meta.CreateReadOnlyVirtualVolume_Response, error) {
	return c.storage.CreateReadOnlyVirtualVolume(ctx, in)
}

// ////////////////////////////////////////////////
// BindPhysicalVolToVirtualVol binds physical volumes to a virtual volume.
func (c *MetaClientMock) BindPhysicalVolToVirtualVol(ctx context.Context, in *meta.BindPhysicalVolToVirtualVol_Request, opts ...grpc.CallOption) (*meta.BindPhysicalVolToVirtualVol_Response, error) {
	return c.storage.BindPhysicalVolToVirtualVol(ctx, in)
}

// UnBindPhysicalVolToVirtualVol unbinds physical volumes from a virtual volume.
func (c *MetaClientMock) UnBindPhysicalVolToVirtualVol(ctx context.Context, in *meta.UnBindPhysicalVolToVirtualVol_Request, opts ...grpc.CallOption) (*meta.UnBindPhysicalVolToVirtualVol_Response, error) {
	return c.storage.UnBindPhysicalVolToVirtualVol(ctx, in)
}

// ////////////////////////////////////////////////
// HeartBeat is a heartbeat of the storage node.
// It can be used to check the health of the storage node.
// It is also used to distribute event to the storage node.
func (c *MetaClientMock) HeartBeat(ctx context.Context, opts ...grpc.CallOption) (meta.StorageMeta_HeartBeatClient, error) {
	return nil, nil
}

func (c *MetaClientMock) Send(in *meta.HeartBeat_Request) error {
	_, err := c.storage.PatchStorageNode(context.Background(), &meta.PatchStorageNode_Request{Id: in.NodeId, PhysicalVolumeUsage: in.Reports.PhysicalVolumeUsage})
	return err
}
func (c *MetaClientMock) Recv() (*meta.HeartBeat_Response, error) {
	return nil, nil
}

func (c *MetaClientMock) AddStorageNode(ctx context.Context, in *meta.AddStorageNode_Request, opts ...grpc.CallOption) (*meta.AddStorageNode_Response, error) {
	return c.cluster.AddStorageNode(ctx, in)
}
func (c *MetaClientMock) GetStorageNode(ctx context.Context, in *meta.GetStorageNode_Request, opts ...grpc.CallOption) (*meta.GetStorageNode_Response, error) {
	node, err := c.cluster.GetStorageNode(ctx, in)
	if err != nil {
		return nil, err
	}
	return &meta.GetStorageNode_Response{StorageNode: node}, nil
}
func (c *MetaClientMock) ListStorageNodes(ctx context.Context, in *meta.ListStorageNodes_Request, opts ...grpc.CallOption) (*meta.ListStorageNodes_Response, error) {
	nodes, err := c.cluster.ListStorageNodes(ctx)
	if err != nil {
		return nil, err
	}
	return &meta.ListStorageNodes_Response{StorageNodes: nodes}, nil
}
func (c *MetaClientMock) GetCandidateStorage(ctx context.Context, in *meta.GetCandidateStorage_Request, opts ...grpc.CallOption) (*meta.GetCandidateStorage_Response, error) {
	return c.cluster.GetCandidateStorage(ctx, in)
}
func (c *MetaClientMock) PatchStorageNode(ctx context.Context, in *meta.PatchStorageNode_Request, opts ...grpc.CallOption) (*meta.PatchStorageNode_Response, error) {
	return c.cluster.PatchStorageNode(ctx, in)
}

func (c *MetaClientMock) IsHealthy(ctx context.Context) bool {
	return true
}

// IfExistsInode check if the inode exists.
func (c *MetaClientMock) IfExistsInode(ctx context.Context, in *meta.IfExistsInode_Request, opts ...grpc.CallOption) (*meta.IfExistsInode_Response, error) {
	return c.node.IfExistsInode(ctx, in)
}

// GetInodeAttr returns the attributes of the specified inode.
func (c *MetaClientMock) GetInodeAttr(ctx context.Context, in *meta.GetInodeAttr_Request, opts ...grpc.CallOption) (*meta.GetInodeAttr_Response, error) {
	return c.node.GetInodeAttr(ctx, in)
}

// Lookup returns the attributes of specified inode which under the given inode.
func (c *MetaClientMock) Lookup(ctx context.Context, in *meta.Lookup_Request, opts ...grpc.CallOption) (*meta.Lookup_Response, error) {
	return c.node.Lookup(ctx, in)
}

// ListItemUnderInode returns the items under the specified inode.
func (c *MetaClientMock) ListItemUnderInode(ctx context.Context, in *meta.ListItemUnderInode_Request, opts ...grpc.CallOption) (*meta.ListItemUnderInode_Response, error) {
	return c.node.ListItemUnderInode(ctx, in)
}

// UpdateInodeAttr update the attr of the specified inode, return the right attr.
func (c *MetaClientMock) UpdateInodeAttr(ctx context.Context, in *meta.UpdateInodeAttr_Request, opts ...grpc.CallOption) (*meta.UpdateInodeAttr_Response, error) {
	return c.node.UpdateInodeAttr(ctx, in)
}

// DeleteInode delete the specified inode, if the inode is a dir, it will return an error.
func (c *MetaClientMock) DeleteInode(ctx context.Context, in *meta.DeleteInode_Request, opts ...grpc.CallOption) (*meta.DeleteInode_Response, error) {
	return c.node.DeleteInode(ctx, in)
}

// DeleteDirInode deletes a directory inode.
// If the directory is not empty and the request is not set as recursively, it will return an error.
func (c *MetaClientMock) DeleteDirInode(ctx context.Context, in *meta.DeleteDirInode_Request, opts ...grpc.CallOption) (*meta.DeleteDirInode_Response, error) {
	return c.node.DeleteDirInode(ctx, in)
}

// CreateInode will create a brand new inode with the given attributes.
func (c *MetaClientMock) CreateInode(ctx context.Context, in *meta.CreateInode_Request, opts ...grpc.CallOption) (*meta.CreateInode_Response, error) {
	return c.node.CreateInode(ctx, in)
}

// Link will make a hard link to the old inode with the given attributes
func (c *MetaClientMock) Link(ctx context.Context, in *meta.Link_Request, opts ...grpc.CallOption) (*meta.Link_Response, error) {
	return c.node.Link(ctx, in)
}

func (c *MetaClientMock) Rename(ctx context.Context, in *meta.Rename_Request, opts ...grpc.CallOption) (*meta.Rename_Response, error) {
	return c.node.Rename(ctx, in)
}

func (c *MetaClientMock) PullLatest(ctx context.Context, in *meta.PullLatest_Request, opts ...grpc.CallOption) (*meta.PullLatest_Response, error) {
	return c.event.PullLatest(ctx, in)
}

func (c *MetaClientMock) AcquireFileLock(ctx context.Context, in *meta.AcquireFileLock_Request, opts ...grpc.CallOption) (*meta.AcquireFileLock_Request, error) {
	return c.node.AcquireFileLock(ctx, in)
}
func (c *MetaClientMock) GetFileLock(ctx context.Context, in *meta.GetFileLock_Request, opts ...grpc.CallOption) (*meta.GetFileLock_Response, error) {
	return c.node.GetFileLock(ctx, in)
}
func (c *MetaClientMock) DelFileLock(ctx context.Context, in *meta.DelFileLock_Request, opts ...grpc.CallOption) (*meta.DelFileLock_Response, error) {
	return c.node.DelFileLock(ctx, in)
}
func (c *MetaClientMock) GetFileLocks(ctx context.Context, in *meta.GetFileLocks_Request, opts ...grpc.CallOption) (*meta.GetFileLocks_Response, error) {
	return c.node.GetFileLocks(ctx, in)
}
