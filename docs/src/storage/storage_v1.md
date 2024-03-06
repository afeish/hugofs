```rust
use crate::Ino;
use crate::error::{*};

// ApiV1 is the assumption that the storage server will provide.
trait ApiV1 {
// client only have to send ino and offset, then we can fulfill the buffer.
fn read(ino: Ino, offset: u64, dest: &[u8]) -> Result<()>;
// client only needs to send the offset and write data, to make the write operation.
fn write(ino: Ino, offset: u64, src: &[u8], twin: std::net::IpAddr) -> Result<()>;
}

// To make this api work, we need to make the store of file as tight as possible,
// in other word that i hope one whole file be find on one server.
//
// Solution: use hashing-ring to decide which server to store the file, then file
// data will not be too loose.
//
// For write:
//
// 1. There will be a hashing-ring on the metadata server,
// client should first query the two appropriate storage server location by ino.
//
// 2. Then client chooses one of them to invoke the write api, includes the associated-storage server
// info in the request.
//
// 3. if success, then commit it.
//
// For read:
//
// 1. As same as the write part, client should first query the two appropriate storage server location by ino.
//
// 2. Then client chooses one of them to invoke the read api, but in read, we do not need to send
// the twin-server any more, cause one file can be found on one server.
//
// 3. once client receives the data, read is finished.

// Pros:
// 1. This api design can make client request more easier, just like normal io operation.
//
// 2. This api design can make the storage server more easier to implement, 'cause we just need to find
// the associated byte slice on the storage server, then read or write.
//
// 3. We make storage server to do the copy work, rather than client. Internal network will have a better performance.
//
// 4. With the ino information, we can implement the gc work at the storage server side, rather than metadata serve.
//
// 5. Last but not least, we can implement the storage engine in append-only mode, which has a good performance on the
// SSD situation.
```
