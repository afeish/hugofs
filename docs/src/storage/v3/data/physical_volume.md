# PhysicalVolume

PhysicalVolume is responsible for managing the `Blocks` on the disk, each PhysicalVolume manages a fixed count
of `Blocks`. It is designed to make the `Block` management easier.

It is an interface. The implementation of `PhysicalVolume` is `DiskVolume` or `RemoteVolume`.
A `DiskVolume` is a `PhysicalVolume` which is stored on the local disk, and a `RemoteVolume` is existed on a
remote machine.

## 1. Problem

Although we shield the persistent logic by the `BlockEngine`, but we still need to manage the `Blocks` on the disk,
like GC, and we also need to know the `Block` is free or not.
We cannot put this logic on the `BlockEngine` cause the `BlockEngine` is only responsible for the persisting.

## 2. Definition

The `PhysicalVolume` have the following attributes:

1. Each `PhysicalVolume` has a fixed size, and the size of the `PhysicalVolume` is the multiple of the `Block` size.
2. Each `PhysicalVolume` has a unique `PhysicalVolumeID`.
3. Each `PhysicalVolume` has a `BlockEngine` to handle the persisting logic.
4. Each `PhysicalVolume` has a `BlockAllocator` to allocate `BlockID` for the `Block` to be written.
5. Each `PhysicalVolume` can only exist on one `StorageNode`, but one `StorageNode` can have multiple `PhysicalVolume`s.
6. We can read/write `PhysicalVolume` by block.

So far, we can find a Block with its `BlockID` and `PhysicalVolumeID` on the `StorageNode`:

```

<BlockID>:<PhysicalVolumeID>:<StorageNode>

```

About `BlockID`, since the `Block` is fixed size, `PhysicalVolume` is also fixed size, so we can use the index to
represent it.

Then the address of block can be defined like this:

```

<BlockIndexInPhysicalVolume>:<PhysicalVolumeID>:<StorageNode>

```

If we use Index to represent the `BlockID`, we can make the logic of finding free slot easier and no need for other
logic.

## 3. Interface

`PhysicalVolume` must provide the following interfaces:

1. `ReadBlock(BlockIndex)`: read the `Block` from the `PhysicalVolume`.
2. `WriteBlock(Block)`: write the `Block` to the `PhysicalVolume` and return the `BlockIndex`

It can just call the `BlockEngine` to do the real IO operation.

## 4. CancelBlock

In some cases, we need to cancel the `Block` which has been written to the `Disk` already.
For example, when we write data to the `Disk` successfully, but we failed to update the
associated metadata to metadata server, then we need to cancel the `Block` to free the slot.

This can be implemented by handling the header part of the `Block`.

## 5. TODO: GC of PhysicalVolume 
