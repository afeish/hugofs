# API

The api should shield the details of the storage, and the client only needs to know which
`StorageNode` to access and read or write data by block.

The API component is the most important part of the StorageNode, for organizing the internal
components and provides a simple interface to the client.

A simple interface can be defined as this:

```protobuf
syntax = "proto3";

package hugo.v1.storage;
option go_package = "github.com/afeish/hugo/rpc/pb/storage";

// StorageService is a service for read/write file in block-level.
service StorageService {
  rpc Read (Read.Request) returns (Read.Response);
  rpc StreamRead(Read.Request) returns (stream StreamReadResponse);
  rpc Write (WriteBlock.Request) returns (WriteBlock.Response);
  rpc StreamWrite(stream StreamWrite.Request) returns (StreamWrite.Response);
}

//////////////////////////////////////////////////

message Block {
  bytes data = 1; // block content
  uint32 len = 2; // length of block
  uint64 index = 3; // index of current block
  uint32 crc32 = 5; // crc32 of block
}


//////////////////////////////////////////////////

message Read {
  message Request {
    uint64 ino = 1; // read from which file?
    optional Range start_block_idx = 2; // read start from where? if not given, read from head.
    optional Range end_block_idx = 3; // read end at where? if not given, read to tail.
    optional uint64 history_version = 4; // read history?
  }
  message Response {
    repeated Block blocks = 1;
  }
}

message Range {
  enum Type {
    INVALID = 0;
    INCLUDE = 1;
    EXCLUDE = 2;
  }
  uint64 val = 1;
}

//////////////////////////////////////////////////

message StreamReadResponse {
  Block block = 1;
}

//////////////////////////////////////////////////

// WriteOneBlock just write one block at the given index or append to the tail.
message WriteBlock {
  message Request {
    uint64 ino = 1; // write to which file?
    optional uint64 block_idx = 2; // write which block? if not given, just append
    bytes block_data = 3; // write a block; create if not exist; overwrite if exist;
    uint32 crc32 = 5; // crc32 of block
  }
  message Response {
    uint64 block_idx = 2; // index of written block
  }
}

//////////////////////////////////////////////////

message StreamWrite {
  message Header {
    uint64 ino = 1; // write to which file?
  }
  message Request {
    oneof mode {
      Header header = 1;
      bytes block = 2;
    }
  }
  message Response {
    repeated uint64 block_idx = 2; // index of written block
  }
}
```

