# VirtualVolume

VirtualVolume is a logic concept, which is responsible for the management of the PhysicalVolumes.
PhysicalVolume is a physical concept, do the dirty work, VirtualVolume is a logical concept,
just tell the PhysicalVolume what to do and do the metadata management.

It is also a glue between the multiple physical devices and metadata.
In original, VirtualVolume is defined to organize different PhysicalVolumes to achieve the high availability of data:
`Data Migration`

But by introducing the concept, we can have more usages of VirtualVolume:

1. multiple replica policies
2. multiple data migration policies

In other words, we shield the replica work from the upper layer and also provides flexibility for the upper layer.

## Interface

A VirtualVolume needs to the following work:

```rust
trait VirtualVolume {
    fn write_block(&self, block: Block) -> Result<(BlockAddr1, BlockAddr2)>; // do replica work here
    fn cancel_block(&self, addrs: (BlockAddr1, BlockAddr2)) -> Result<()>;
    fn balance(&self) -> Result<()>; // make each PhysicalVolume have a samilar number of blocks
    fn migrate_to_cloud(&self) -> Result<()>; // TODO: migrate data to cloud
}
```

