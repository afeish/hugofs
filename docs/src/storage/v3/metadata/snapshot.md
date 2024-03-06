# Snapshot

Metadata Server should provide some interface about maintaining snapshots.

A snapshot can be implemented by creating a reference to one specified version of the metadata.

But no matter make snapshot at which level, we will get a read-only VirtualVolume eventually.

## Snapshot at directory level

```go
package design

type BlockIndexInFile = uint64
type TxnID = uint64

// Snapshot describes the structure of a snapshot. 
type Snapshot struct {
	ID               uint64
	VirtualVolumeID  uint64
	ParentDirIno     uint64
	SnapshotAt       TxnID
	AssociatedInodes []uint64
}
```

When creating a snapshot, we just make a structure which holds the reference to some inodes.

When deleting a snapshot, we just delete the snapshot record. But the associated data will
not be deleted immediately. We will delete the data in GC.
