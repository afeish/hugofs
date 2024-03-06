# InoBlockMap

InoBlockMap describes the mapping between inode and block.

A block is a unit of storage, and an inode is a unit of file.
Since we need to support multiple versions, we will implement it by MVCC.

First of all, the InoBlockMap looks like this:

```go
package design

type InoBlockMap struct {
	Ino             uint64 // which file?
	Maps            map[BlockFileIndexVersion]*BlockLocation
	DeleteAtVersion *uint64
}

type BlockFileIndexVersion struct {
	BlockIndexInFile uint64
	Version          uint64
}

type BlockLocation struct {
	VirtualVolumeID            uint64
	PhysicalVolumeID           uint64
	BlockIndexInPhysicalVolume uint64
	CloudStorageKey            string
}

```

Once we modify the file content, we will append a new record to the InoBlockMap, rather than modify the old one.

Once we read a file, we will open a transaction, and read the latest committed version of the file.

## About Delete

Delete operation is a little tricky, we cannot delete the block immediately since we need to support multiple versions
and snapshot.

So if we need to delete a file, we just mark the `DeleteAtVersion` field.
The InoBlockMap record will be deleted if no snapshot is pointing to it.

## Next

In the next section we will talk about the implementation of MVCC.
