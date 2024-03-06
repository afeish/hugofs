# GC of data part

The StorageNode server has no idea about if the data are still referenced by the metadata server.

The main idea is asking metadata server to check if the block is referenced.

The GC logic should be implemented on the VirtualVolume side.

So it can transfer the clean notification to the PhysicalVolume side.

It is a background task, but it should also cloud be triggered by the user.

The major work of GC is freeing some slots which are not being referenced in the PhysicalVolume.

## GC steps

1. Collect the using slots from the metadata server.
2. Collect the using slots from the PhysicalVolume.
3. Compare the two lists and get the unused slots.
4. Free the unused slots.
