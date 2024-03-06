# hugo Introduce

This project aims to make a general, POSIX-compliant and cloud-native distributed-file-system (DFS) written in Go.
The main desires of hugo are:

- Implementation of compliant POSIX semantics
- Employment of distributed architecture to achieve horizontal scalability
- Better IO performance by storing data to adjacent cloud instances rather than S3
- Massive capacity with acceptable price (put cold data to S3; cloud instances can be all destroyed)

## Brief architecture

Roughly speaking, this project design is following the central architecture,
like [GFS](https://static.googleusercontent.com/media/research.google.com/en//archive/gfs-sosp2003.pdf).
The big difference between GFS is that we don't let one single node to manage the whole system,
instead, we employee a distributed consistent store service to store metadata of files.
And the file data is split into multiple fixed-size blocks, which are stored in the storage cluster (a bunch of
cloud instances that only store block).
To make the system affordable, we will free data node's space by uploading cold data to s3, and retrieve it when
necessary.
Thus, this system can be seen as a high-performance POSIX-compliant file system layer for S3,
with a fault-tolerant distributed consistent store service (the storage cluster can be seen as a reliable
high-performance cache).

*NOTE: We should be able to make one node act in multiple roles to reduce cost.*
*About the POSIX semantics, we achieve it by implementing the FUSE interface.*

Major components of hugo:

1. Metadata Cluster
2. Storage Cluster
3. Fuse Client
4. S3

![](./images/arch.drawio.svg)

## Roadmap:

- [x] Mount by FUSE
- [ ] Compliant POSIX semantics.
- [ ] Support multiple filesystems(tenants)
- [ ] Format filesystem
- [ ] Support S3
    - [ ] Backup data to s3
        - [ ] Backup algorithm
    - [ ] Retrieve data from s3
        - [ ] Prefetch algorithm
    - [ ] Backup whole system's metadata to s3
    - [ ] Rebuild system from s3
- [ ] Distributed
    - [ ] Implement consistent store service
        - [x] Tikv
        - [ ] Deprecate Tikv and implement by self
    - [ ] Data replication
        - [x] Replicate data to multiple nodes
    - [ ] Verify data/metadata consistency
        - [ ] Read-After-Close
    - [ ] Fault tolerance
        - [ ] Balance data when adding/removing nodes
- [ ] Optimize IO performance
    - [ ] random 4KB
    - [ ] sequential 1MB
- [ ] Optimize Metadata performance

## Design

### 1. Metadata Cluster

The metadata cluster acts as a consistent store service that provides file metadata management;
at present, we implement it by employing [tikv](https://github.com/tikv/tikv) directly,
which means we don't implement the raft algorithm by ourselves.
This way brings some cons to us, and we may implement the raft algorithm by ourselves in the future.
The metadata associated operations are defined under the `proto/meta` directory.

Some important metadata:

1. InodeAttr
    - which stores the inode's attributes, like the file size, the number of hard links, etc.
2. Dentry
    - which stores the directory entry, like the file name, the inode number, etc.
3. StorageNode
    - which stores the storage node's information, like the node's id, the node's configuration, etc.
4. InoBlockMap
    - which stores the inode's block mapping information, like the inode number, the block number, etc.

### 2. Storage Cluster




