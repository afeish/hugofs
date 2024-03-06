# Distributed Storage Server V2

This article describes the design of a distributed storage server,
which aims to handle Block-level IO requests from clients, and manage
the file content in an efficient way.

## 1. Goals

1. Provides an interface that is simple to clients, and shields the underlying storage details.
2. Make node server high availability, data are replicated on multi machines, but
   the replicate process can also be done in the background, transparently.
3. One single file can be stored on different storage nodes.
4. Support different storage schemas, and multi-tenant. (use resources efficiently)

## 2. Implementation

We can divide the storage server into two parts, the storage part and distributed part.
The Storage part is responsible for handling Block-level IO requests which are based on block,
and the other part is responsible for managing the storage server nodes and storage metadata,
making clients can access the expected data from any storage node.

## 2.1 How files are organized?

We split files into `Blocks`, and each `Block` is 512kb.
The `block` is the smallest unit of storage, and the `block` is the smallest unit of IO.
And the interface is also based on `block`, rather than byte range.
Client should map offset to `block` by self.

In this way, the implementation of storage block becomes easy, but it will cause a problem that
we need to store missive metadata on the metadata server to tell fs
how a file is organized when occur a big file.

So we need to reduce the metadata size, and we can do this by using a `IndiractBlock` to store a range
of blocks. Which refers to a block which contains a list of block ids.

## 2.2 Interface

Since we have split files into `Blocks`, so the interface exposed to clients will be like this:

```rust
trait StorageService {
    fn read_block(&self, block_id: u64) -> Result<&[u8, 512 * 1024]>;
    fn write_block(&self, block_id: u64, buf: &[u8; 512 * 1024]) -> Result<()>;
}
```

## 2.3 Manage Blocks

Block is the smallest unit of storage, and the `block` is the smallest unit of IO.
We save block data in a `Store`, and the `Store` is a key-value store.

```rust
trait BlockStore {
    fn get(&self, block_id: u64) -> Result<Block>;
    fn put(&self, block: Block) -> Result<()>;
}

struct Block {}
```

Although we treat it as a k/v database, but in actually, we save file content in the file system
directly. The reason why we wrap a k/v is to make it easy to change the storage backend.
We can use a memory or a real database to store the block.

### 2.3.1 Volumes

To make manage blocks easier, we introduce a concept named `VirtualVolume`,
which is a logical storage unit, consisting of multiple `PhysicalVolumes`.
`VirtualVolume` can be accessed from all nodes, 'cause it is a logical concept,
but the `PhysicalVolume` is the real storage unit, it can only be accessed from
the belonging node.
And one `PhysicalVolume` can only belong to one node and one `VirtualVolume`.

The `PhysicalVolume` handle the real IO operation, and the `VirtualVolume` is responsible
for manages metadata of `Block`.
Since `PhysicalVolume` is the real storage unit, and `Block` has a fixed length.
We can make `PhysicalVolme` is also fixed length. Then each `Block` can be represented by
index. Here we assume `PhysicalVolume` is 40GB, and each `Block` is (512Kb = 64KB), so a
`PhysicalVolume` can store ((40 * 1024 * 1024 * 1024 * 8 )/ (512 * 1024) = 640 * 1024) blocks.


### 2.1.1 Volume

When the IO request from clients to one storage node, the storage node will first
query metadata server to find the `VirtualVolume` to which the `Ino` belongs to,
then the storage node lets `VirtualVolume` do the rest work.
`VirtualVolume` route request to suitable `PhysicalVolume` which belongs to the
`VirtualVolume`, and then `PhysicalVolume` do the real IO operation.
After the operation is finished, the storage node needs to update the associated metadata on the
metadata server.
Following this design, we can make `VirtualVolume` to do data migration and load balance.

We only need to persist little basic metadata information in the metadata server,
and clients only need to know which virtual volume, and any storage node, but nothing
more, simple!

### 2.1.2 How to store file data?

Here comes the real challenge, how to translate the offset of `Ino` to the
real offset in the `PhysicalVolume`?

#### 1. locate ino

We can let each `PhysicalVolume` maintain a local table that records currently
used `Ino`.

So far, we first use `Ino` to find it in which `VirtualVolume`, then use `Ino` to
find it in which `PhysicalVolume`.
And then the question is the translation of the file offset to `PhysicalVolume`.
To answer the question, we need to design how we store the data.

#### 2. store the data

From bottom to top, we can learn from the design of K/V storage, use
LSM-tree to achieve the high performance of write, and memory index to
achieve the high-performance of read.

But traditional LSM tree is not a good idea for file-system, 'cause one
file may modify its content a lot, which will cause amplification of writing.
So we need to design a new LSM tree, which can be implemented on top of
the original LSM tree and can be used to store file content.

Thankfully, some people have already done this.
We can learn from the paper: [wisckey](https://www.usenix.org/system/files/conference/fast16/fast16-papers-lu.pdf),
which separates data metadata and data itself to make a better lsm.

We do not have to remake wheels.
What we do is wrap a normal LSM-based store into `Wisckey` store.
And it exposes the same interface as a regular kv store.

We can learn from Ext4Fs, which uses a block size of 4K to save the file content.
With the fixed-size block, we can easily translate the offset into the expected
block id and offset in the block.
To make reading/writing easier and reduce amplification, we can introduce an abstraction
over block:

```rust
// ObjectBlock can be extended to reduce the number of blocks.
enum ObjectBlock {
    UnitBlock,
    // organize blocks into a linked list, it exposes as a normal block.
    BlockLinkedList(),
}
```

One block can just be a unit of a linked list, with this design, we can reduce the
block metadata usages.

Now we can save block data in this way:

1. split ino into multi-blocks

   ```
   key: {ino}/block/{index}
   value: {block_id:u64, block_size: u64, is_unit:bool}
   ```

2. save block data
   ```
   key: unit_block/{block_id}
   value: {data: &[u8]}
   ```

3. maintain the block list
   ```
   key: block_linked_list/{block_id}
   value: {next_block_id: u64, cur_block_id: u64}
   ```

Now, we make the io operation on file become read/write a block, and that's it.

## 2.2 Future

### 2.1.1 GC

We can make the GC at the wiskkey store level, unused block will be deleted if no valid
key point to it.

### 2.2.2 Multi Version

We add a field named `Epoch` on each `Ino` attribute, it is a monopolistic counter,
once the `Ino` is modified, the `Epoch` will be increased.
When the client wants to read a file, it can provide the `Epoch` it expects,
we can return the expected data, or return an error if the `Epoch` is not expected.

### 2.2.3 Snapshot

To make a snapshot, we just need to make a read-only `VirtualVolume` with a new `Epoch`,
and we cannot modify the `VirtualVolume` anymore, so easy!

## 2.3 Reference

- [petal inspires virtual volume idea](https://www.cs.purdue.edu/homes/csjgwang/CloudNativeDB/PetalASPLOS96.pdf)
- [beaver: facebook blob storage ](https://www.usenix.org/legacy/events/osdi10/tech/full_papers/Beaver.pdf)

Block LinkedList is learned from Ext4FS. 
