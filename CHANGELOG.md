<a name="unreleased"></a>
## [Unreleased]

### Bug Fixes
- **fuse-test:** chown,chmod,mkfifo,mkdir,mknod,symlink
- **iobuffer:** fix random buffer
- **posix-test:** rename,open
- **posix-test:** symlink unlink utimensat link

### Code Refactoring
- **store:** store ops and grpc ifaces

### Features
- **buffer:** refine buffer impl
- **infra:** add deploy infra
- **mount:** refactor mount impl
- **posix-test:** add posix test ci
- **raft:** add raft to support the control of scheduler
- **rw:** split read and write
- **s3:** add storage s3 skeleton
- **test:** add race-test
- **trace:** add trace support


<a name="v0.0.0"></a>
## v0.0.0 - 2023-03-23
### Bug Fixes
- filehandle count self
- fix orginize problem in meta package
- update heartbeat interface and layout
- update some interface comments
- rename
-     1. invalid go module name     2. introduce event mechanism
- update client layout
- store layout
- update readme
- book
- **io:** https://github.com/afeish00/hugo/-/issues/12 and https://github.com/afeish00/hugo/-/issues/7
- **link:** soft link and hard link

### Features
- add git-chglog mechanism
- add virtual file system interface
- add cloc rule
- update readme
- **add godoc:** check project layout with godoc
- **block:** add block engine impl
- **ci:** add docker dev test setup
- **client:** make client io works
- **docker:** refactor command outline
- **fuse:** fuse rw works
- **fuse-test:** rename worked
- **fuse_test:** make fuse_test pass
- **fuse_test:** mkdir and rmdir worked
- **fuse_test:** add dir_seek support
- **storage:** add storage rpc mock
- **store:** add txn support
- **store:** add tikv support
- **vol:** add physical local vol impl
- **volume:** add physical vol impl and releated logic

### Init
- some idea
- update test
- add test suite example

### Update
- book
- add snapshot and gc
- inode
- meta key organize
- 1. change name of package store to storage, means packages of storage node.
- add deatils on inode associated stuff
- add some makefile rules
- update interface
- changelog
- add make rule about chglog

### Pull Requests
- Merge branch 'topic/neo/io-v2' into 'main'
- Merge branch 'topic/neo/resolv-test' into 'main'
- Merge branch 'topic/neo/fix-dirseek' into 'main'
- Merge branch 'topic/dh/leak' into 'main'
- Merge branch 'topic/neo/fix-mkdir' into 'main'
- Merge branch 'topic/dh/memStore' into 'main'
- Merge branch 'topic/neo/fuse-rw-work' into 'main'
- Merge branch 'topic/neo/fuse-rw-work' into 'main'
- Merge branch 'topic/neo/add-node-test' into 'main'
- Merge branch 'topic/neo/add-node-test' into 'main'
- Merge branch 'topic/dh/fuse' into 'main'
- Merge branch 'topic/dh/memFS' into 'main'
- Merge branch 'topic/neo/refactor-cmd' into 'main'
- Merge branch 'topic/neo/add-cache' into 'main'
- Merge branch 'topic/neo/impl-blockengine' into 'main'
- Merge branch 'topic/neo/impl-blockengine' into 'main'
- Merge branch 'topic/dh/sm' into 'main'
- Merge branch 'topic/dh/do3' into 'main'
- Merge branch 'topic/dh/do2' into 'main'
- Merge branch 'topic/dh/do' into 'main'
- Merge branch 'topic/dh/interface' into 'main'
- Merge branch 'topic/dh/docGC' into 'main'
- Merge branch 'topic/dh/nodeIdea' into 'main'
- Merge branch 'topic/neo/add-txn-support' into 'main'
- Merge branch 'topic/neo/add-container-dev' into 'main'
- Merge branch 'topic/dh/testNode' into 'main'
- Merge branch 'topic/dh/testNode' into 'main'
- Merge branch 'topic/dh/node' into 'main'
- Merge branch 'topic/neo/fix-link' into 'main'
- Merge branch 'topic/dh/fuseRW' into 'main'
- Merge branch 'topic/neo/add-tikv' into 'main'
- Merge branch 'topic/dh/fuseRW' into 'main'
- Merge branch 'topic/dh/optimizeStore' into 'main'
- Merge branch 'topic/dh/refact' into 'main'
- Merge branch 'topic/neo/impl-inode' into 'main'
- Merge branch 'topic/dh/meta' into 'main'
- Merge branch 'topic/dh/mock_meta' into 'main'
- Merge branch 'topic/dh/meta' into 'main'
- Merge branch 'topic/dh/meta' into 'main'
- Merge branch 'topic/dh/io' into 'main'
- Merge branch 'topic/dh/io' into 'main'
- Merge branch 'topic/dh/cache2' into 'main'
- Merge branch 'topic/neo/fix-chmod' into 'main'
- Merge branch 'topic/dh/gile2' into 'main'
- Merge branch 'topic/neo/fix-readdir' into 'main'
- Merge branch 'topic/dh/gile' into 'main'
- Merge branch 'topic/dh/gile2' into 'main'
- Merge branch 'topic/neo/fix-readdir' into 'main'
- Merge branch 'topic/dh/def' into 'main'
- Merge branch 'topic/dh/mount' into 'main'


[Unreleased]: https://github.com:2222/ellis.chen/hugo/compare/v0.0.0...HEAD
