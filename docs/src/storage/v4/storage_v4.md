# Version4 of the hugo Storage Design

In this version, a simple POC of previous storage design was implemented. Compared to the previous version, some changes were introduced to
better solve the problems we faced

[toc]

## Block Changes

We add the `Checksum` field to the block's `Header` struct, now the `Header` struct looks like this:
```go
Header struct {
    Kind     uint8  // maybe an index block or a data block
    DataLen  uint64 // length of the data
    Index    uint64 // index of the block
    Crc32    uint32 // crc32 of the data
    Checksum []byte // sha256 of the data
    Canceled uint8  // canceled or not, if canceled == 1, means this block is invalid
}
```
The `Checksum` field uniquely identifies the block, and it's useful for further de-staging the block from local volumes to the remote.


## Block Engine implementation

A block engine can be initialized using the following methods:
```go
eng, err := engine.NewProvider(ctx, engine.ProviderInfo{
		Type:   cfg.Engine,
		Config: &engine.Config{Prefix: cfg.Prefix, VolID: cfg.Id},
})
```
In the above example, `cfg.Prefix` is the local file path, it may be a local filesystem directory or ssd mount point. All the block persisted will be stored in the `cfg.Prefix` directory.

Currently, the engine supports `EngineTypeFS` and `EngineTypeINMEM`.

## Physical volume implementation

`PhysicalVolume` is our manage unit of the block ops.  An implementation named `LocalPhysicalVolume` is introduced. It looks like this:
```go
type LocalPhysicalVolume struct {
	sync.RWMutex
	id                PhysicalVolumeID
	stat              *VolumeStatistics
	location          string
	slot              *Slot
	engine            engine.Engine
	storageMetaClient pb.StorageMetaClient
	store             store.Store

	blockLru    *BlockLRU
	evitManager *EvitManager

	messenger *Messenger
}
```
The `LocalPhysicalVolume` can be initialized by `NewLocalPhysicalVolume(ctx context.Context, cfg *Config, messenger *Messenger)`, where the `Config` looks like this:
```go
type Config struct {
	store.UnimplementedSerializer
	Engine engine.EngineType
	Prefix string
	Id     PhysicalVolumeID

	Quota     *uint64
	Threhold  *float64
	BlockSize *uint64

	DBName string
	DBArg  string
	CreatedAt uint64
}
```
If `Quota`, `Threhold` and  `BlockSize` are optional, if not provided, the default will be `40G`, `0.75`, `512K`

### VolumeStatistics

The `stat` field in `LocalPhysicalVolume` maintains the current state of the volume. It looks like this:
```go
type VolumeStatistics struct {
	store store.Store `json:"-"`

	VolID PhysicalVolumeID `json:"-"`

	Quota      uint64
	Threhold   float64
	InuseBytes uint64

	notifyCh chan VolumeStatistics
}
```
Every time afther the `stat` is updated, current snapshot of the stat will be sent back to the `notifyCh` channel.

### Slot

The `slot` field in `LocalPhysicalVolume` manage the occupation of the block . It looks like this:
```go
type Slot struct {
	bits *roaring64.Bitmap
	cap  uint64

	volID PhysicalVolumeID // immutable

	mu        sync.RWMutex
	store     store.Store // take a reference
	hashPairs map[uint64][]byte
}
```
We divide the whole physical volume into a set of slots based on the volume's capacity and block's max size()
The slot's cap can be computed by `slotCap := int(threhold * float64(quota) / float64(blockSize))`. 

Every time a block is written to the physical volume, we should first call `slot.nextFreeSlot()` to get a empty slot (named **slotID**) for the block.
If we successfully get a **slotID**, we use it as the **filename** of the block.

After the block has been written to the physical volume successfully, the slot will be occupied by calling `slot.Occupy(slotID uint64, hash []byte)`. 
In the meantime, the hash or (**BlockID**) will be saved to help the lookup of the **blockID** to the **slotID**

### BlockLRU

The `blockLru` field in `LocalPhysicalVolume` is used to record the access activity of a given blocks. It's used to seperate the `cold` and `hot` blocks,
and give the upper components the advice that which blocks should be de-staged to remote storage. The `BlockLRU` looks like this:
```go
type BlockLRU struct {
	backed *lru.Cache[int, string] // K is slotID, V is blockID the block (used as key on s3)
	cap int
}
```

It internally uses `lru.Cache` to track and evict the **slotID-blockID** relationships. Every time a block is read or written, such relation will be refreshed to
escape the eviction. A `onEvit` hook can be provided to help upper components gather the `cold` blocks.

### EvitManager

The `evitManager` field in `LocalPhysicalVolume` manage the de-staging of the block. Right now, it's just a skeleton

### Messenger

The `messenger` field in `LocalPhysicalVolume` manage communication of the `LocalPhysicalVolume` to upper components. It's used to send various messages to upper components. The structure  looks like this:
```go
type Messenger struct {
	volStatCh   chan VolumeStatistics
	blockMetaCh chan BlockMeta
}
```
As the field in the struct suggests, Currently it support two type of messages(i.e. `VolumeStatistics` and `BlockMeta`).

For `VolumeStatistics`, everytime the volume's `InuseBytes` changed(possibly the add of new block or deletion of block), a message will be sent out.

For `BlockMeta`, everytime a new block is added, a message will be sent out to upper components to tell the relation of `BlockID` and `PhysicalVolumeID`


## Node implementation

Here `Node` stands for a actual storage instance. It will serve the grpc protocol or user-defined protocol to the client part. It will manage the individual physical
volumes and dispatch the `Read` or `Write` requests to the specified volume. The `Node` structure looks like this:
```go
type Node struct {
	ID StorageNodeID

	clusterClient     pb.ClusterServiceClient
	storageMetaClient pb.StorageMetaClient
	beater            pb.StorageMeta_HeartBeatClient

	mu sync.RWMutex

	cfg       *Config
	selector  *Selector
	messenger *vol.Messenger

	store store.Store
}
```
The `Node` instance can be constructed using ` NewNode(ctx context.Context, cfg *Config)`. The `Config` object looks like this:
```go
type Config struct {
	ID        uint64
	IP        string
	Port      uint16
	MetaAddrs []string `yaml:"meta_addrs"`
	BlockSize *uint64  `yaml:"block_size"`
	Section   []struct {
		ID       PhysicalVolumeID
		Engine   engine.EngineType
		Prefix   string
		Quota    *uint64
		Threhold *float64
	}
	DBName string `yaml:"db_name"`
	DBArg  string `yaml:"db_arg"`
	Test   bool
}
```

### Selector

A `Selector` is to forward the `Read` and `Write` request to dest volumes. It maintain the state of the volumes and relation between  `BlockID`-`VolID`. The structure looks like this:
```go
type Selector struct {
	mu         sync.RWMutex
	candidates map[PhysicalVolumeID]vol.PhysicalVolume
	volStats   map[PhysicalVolumeID]vol.VolumeStatistics
	blockMetas map[string]*vol.BlockMeta
	messenger  *vol.Messenger
	ctx        context.Context
	store      store.Store
}

```

### Startup behavior

When the `Node` is constructed, a given set of operations will be executed, the pseudo code looks like this:
```go  
func (v *Node) init(ctx context.Context) error {
	cfg := v.cfg

	v.RegisterMyself(ctx) { // connect to meta, the register this node
	v.LoadRegisteredVolumes(ctx) // connect to meta the get the registered volumes and start them up

	if cfg != nil && cfg.Section != nil {
		for _, sec := range cfg.Section {
			c := &vol.Config{}
			v.ObtionVolumeId(ctx, c)// if found any config volumes, register them 
			v.StartVolume(ctx, c){ // start volume them up
		}
	}
	v.beater = v.storageMetaClient.HeartBeat(ctx)
	go v.sendReport(ctx) // send the volume usage data to the meta side
	go v.recvInstructions(ctx) // accept the meta instructions
	return nil
}
```

### Write behavior

When a block write request is received, the node will first verify the request and use `Selector` to choose the appropriate volume to write,
and then forward the request to the volume to finish the write operation

### Read behavior

When a block read request is received, the node will use `Selector` to lookup the `BlockID`-`VolID` relation to find the dest volume. If such releaton
is not found, an error will be thrown to the client. If found, the read will be forwarded to the destination volume


## VolumeCoordinator

The `VolumeCoordinator` is used by the client side fuse application to communicate with the meta server to determine which `Node` to choose to `Read` or `Write`.
Its structure looks like the following:

```go
type VolumeCoordinator struct {
	inodeMetaClient pb.RawNodeServiceClient
	clusterClient   pb.ClusterServiceClient

	activeMu         sync.RWMutex
	activeMetaClient pb.StorageMetaClient

	StorageConnector     PeerConnector
	StorageMetaConnector PeerConnector
```


### Write behavior

When a block write occurs, The `VolumeCoordinator` will first communicate with the meta server to get a candidate nodes' connection informations. 

And then write the raw data to the destination node concurrently. If both succed, A `UpdateBlockLocation` will be executed to update the file's blockMap


