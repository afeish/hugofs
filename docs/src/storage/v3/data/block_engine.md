# BlockEngine

BlockEngine is an interface to read/write block by block id.

## How to persist Blocks?

At present design, each Block is mapping to a real file on the disk, and the file name is the `BlockID`.
But we still need to provide an abstraction of how to persist `Blocks` to fit the future requirements,
make the implementation more flexible while make writing test code easier.

So we can introduce a concept called `BlockEngine` to handle the details of how to persist `Blocks`.

```rust
trait BlockEngine {
    fn read(&self, block_id: u64) -> Result<&[u8]>;
    fn write(&self, block_id: u64, data: &[u8]) -> Result<()>;
}
```

## How to read block from remote `StorageNode`?

To make Blocks can be transferred between different `StorageNode`s, we need to define a rpc interface to
make BlockEngine can read and write data from remote `StorageNode`.
The rpc interface can be defined as a regular k/v database:

```protobuf
syntax = "proto3";

service BlockEngine {
  rpc Get(Get.Request) returns(Get.Response);
  rpc Put(Put.Request) returns(Put.Response);
}

message Get {
  message Request {
    string key = 1;
  }
  message Response {
    bytes value = 2;
  }
}

message Put {
  message Request {
    string key = 1;
    bytes value = 2;
  }
  message Response {}
}
```

Then we will have a `RemoteBlockEngine` to implement the `BlockEngine` trait by calling the RPC interface.

## TODO: Implement a better protocol to replace the gRPC.
