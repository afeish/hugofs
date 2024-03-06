
# Version5 of the hugo Storage Design

In this version, a bridge that connect the client to meta and storage server was implemented. Compared to the previous version, some changes were introduced to
better solve the problems we faced


## VolumeCoordinator

`VolumeCoordinator` is a helper structure that help the client to communicate with the meta and storage server. It provides two simple methods `Read` and `Write`
to interact with the meta and storage server. The `VolumeCoordinator` looks like this:

```go

type VolumeCoordinator struct {
	NodeSelector    NodeSelector
	nodeHealthChgCh chan struct{}
	cfg             *adaptor.Config

	metaProxy *MetaProxy
}
```

As the field name `metaProxy` suggests, the `MetaProxy` help maintain the connection to the meta server. It's self maintainable and responsive to the health of the
underlying grpc connections. If the underlying connection went to dead, It will auto choose a new connection from the pool of meta addresses.

`NodeSelector` is another helper to manage the connection to the relative storage server. Internally, it maintains a map of connection cache.

## Test the write and read from client side


### Prepare the environment
```shell
# start the test containers
make dev

# register a volume on each storage node 1
docker exec -it g_s1 bash
root@g_s1:/# hugo s v a -p /mnt/1-1 
[2023/02/27 08:59:09.668 +00:00] [INFO] [main.go:50] ["debug mode"]
engine: FS
success register to node  1630129891359916032

# register a volume on each storage node 2
docker exec -it g_s2 bash
root@g_s1:/# hugo s v a -p /mnt/2-1 
[2023/02/27 08:59:09.668 +00:00] [INFO] [main.go:50] ["debug mode"]
engine: FS
success register to node  1630129891359916034

# register a volume on each storage node 3
docker exec -it g_s3 bash
root@g_s1:/# hugo s v a -p /mnt/3-1 
[2023/02/27 08:59:09.668 +00:00] [INFO] [main.go:50] ["debug mode"]
engine: FS
success register to node  1630129891359916033
```

### Write some data to the cluster

```shell
docker exec -it g_c1 bash

root@g_c1:~# dd if=/dev/urandom of=2m bs=1M count=2
2+0 records in
2+0 records out
2097152 bytes (2.1 MB, 2.0 MiB) copied, 0.00588917 s, 356 MB/s
root@g_c1:~# md5sum 2m
06bbbef15f2a5daf215ebf0fdaaa53ee  2m
root@g_c1:~# hugo c io w -f 2m
[2023/03/03 08:41:24.571 +00:00] [INFO] [main.go:51] ["debug mode"]
[2023/03/03 08:41:24.574 +00:00] [INFO] [cmd_io.go:102] ["success create ino"] [ino=8]
[2023/03/03 08:41:24.574 +00:00] [DEBUG] [impl.go:69] ["try connect to meta-server"] [meta-server=meta:26666]
[2023/03/03 08:41:24.575 +00:00] [DEBUG] [impl.go:79] ["establish the connection"] [meta-server=meta:26666]
[2023/03/03 08:41:24.578 +00:00] [INFO] [cmd_io.go:118] ["success read file to buffer"] [read=2097152] [size=2097152]
0
1
2
3
```
From the above step, we successfully upload a file named `2m` and its md5sum is `06bbbef15f2a5daf215ebf0fdaaa53ee`

### Read data from the cluster

To read the data we previously uploaded, we should get the `ino` first

```shell
root@g_c1:~/restore# hugo c io l 
[2023/03/03 08:43:22.074 +00:00] [INFO] [main.go:51] ["debug mode"]
name: 1m, ino: 7
name: 2m, ino: 8
name: hello.txt, ino: 6
name: .Trash, ino: 2
name: .Trash-1000, ino: 3
name: .accesslog, ino: 5
name: .stats, ino: 4
```
As we can tell, the file named `2m`' ino is `8`. Now let's read the data from the cluster and check its integrity
```shell
root@g_c1:~# mkdir restore
root@g_c1:~# cd restore
root@g_c1:~/restore# hugo c io r -i 8
[2023/03/03 08:45:39.530 +00:00] [INFO] [main.go:51] ["debug mode"]
[2023/03/03 08:45:39.532 +00:00] [DEBUG] [impl.go:69] ["try connect to meta-server"] [meta-server=meta:26666]
[2023/03/03 08:45:39.533 +00:00] [DEBUG] [impl.go:79] ["establish the connection"] [meta-server=meta:26666]
[2023/03/03 08:45:39.535 +00:00] [DEBUG] [cmd_io.go:231] ["find block meta"] [length=4]
dst file is  /root/restore/2m
[2023/03/03 08:45:39.549 +00:00] [INFO] [cmd_io.go:260] ["success write to file"] [dst=/root/restore] [written=2097152] [ino=8]
root@g_c1:~/restore# md5sum 2m
06bbbef15f2a5daf215ebf0fdaaa53ee  2m
```
The restore file `/root/restore/2m`'s md5sum is `06bbbef15f2a5daf215ebf0fdaaa53ee` which is identical to the previous write file


### Write some data to the cluster in batch

Use `hugo c io bw` to write some random data in batch to the cluster. As follows:

```shell
docker exec -it g_c1 bash
root@g_c1:~# hugo c io bw
[2023/03/30 07:56:21.746 +00:00] [INFO] [main.go:51] ["debug mode"]
[2023/03/30 07:56:21.755 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock191131893] [md5=1999785ee6c2f994186730b274429de5]
[2023/03/30 07:56:21.756 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock191131893]
[2023/03/30 07:56:21.764 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock551987432] [md5=103c8437b24a62e49339794897981a2e]
[2023/03/30 07:56:21.764 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock551987432]
[2023/03/30 07:56:21.772 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock279551581] [md5=100878821b926778c72ae74c9ca401b3]
[2023/03/30 07:56:21.773 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock279551581]
[2023/03/30 07:56:21.781 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock2715545839] [md5=18bbc73a7f5acc870f9f2f84862c8762]
[2023/03/30 07:56:21.781 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock2715545839]
[2023/03/30 07:56:21.789 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock3312011975] [md5=0834f5d106b6b6927b0e4d84bfdce5c6]
[2023/03/30 07:56:21.789 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock3312011975]
[2023/03/30 07:56:21.798 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock1034607808] [md5=0ce008d6ce324058fd9ee3481399f673]
[2023/03/30 07:56:21.798 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock1034607808]
[2023/03/30 07:56:21.806 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock3675762399] [md5=9055e4a2457a23bb5aef634e19f6b298]
[2023/03/30 07:56:21.806 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock3675762399]
[2023/03/30 07:56:21.814 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock4162414138] [md5=27e721df67e20e6633e0631004dbc408]
[2023/03/30 07:56:21.814 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock4162414138]
[2023/03/30 07:56:21.822 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock1751962146] [md5=9a559ee2e09f3a023a88a8b51ba17c2a]
[2023/03/30 07:56:21.822 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock1751962146]
[2023/03/30 07:56:21.831 +00:00] [DEBUG] [io.go:33] [md5] [name=/tmp/mock3879648475] [md5=8a7112c4c21e959b5586f9ceda73becc]
[2023/03/30 07:56:21.831 +00:00] [DEBUG] [cmd_io.go:180] ["using generated mock file"] [name=/tmp/mock3879648475]
[2023/03/30 07:56:21.833 +00:00] [DEBUG] [cmd_io.go:209] ["writing file to hugo"] [name=/tmp/mock191131893]
[2023/03/30 07:56:21.837 +00:00] [INFO] [cmd_io.go:237] ["success create ino"] [ino=27]
[2023/03/30 07:56:21.837 +00:00] [DEBUG] [impl.go:72] ["try connect to meta-server"] [meta-server=meta:26666]
[2023/03/30 07:56:21.838 +00:00] [DEBUG] [impl.go:82] ["establish the connection"] [meta-server=meta:26666]
[2023/03/30 07:56:21.845 +00:00] [INFO] [cmd_io.go:253] ["success read file to buffer"] [read=1048576] [size=1048576]
[2023/03/30 07:56:21.846 +00:00] [DEBUG] [impl.go:128] ["start connect to storage"] [addr=s2:27778]
[2023/03/30 07:56:21.848 +00:00] [DEBUG] [impl.go:128] ["start connect to storage"] [addr=s3:27779]
[2023/03/30 07:56:21.849 +00:00] [DEBUG] [coordinator.go:111] ["select storage"] [primary=StorageClientImpl(s2:27778)] [secondary=StorageClientImpl(s3:27779)]
[2023/03/30 07:56:21.849 +00:00] [DEBUG] [coordinator.go:111] ["select storage"] [primary=StorageClientImpl(s2:27778)] [secondary=StorageClientImpl(s3:27779)]
[2023/03/30 07:56:21.864 +00:00] [DEBUG] [coordinator.go:145] ["write block succed"] [ino=27] [block-id=68d615506025ae74e98e70ec8f34b89148f87891893c2971ea1f88b696b7f2fd] [block-index=1] [primary-vol=1641346311947206656] [secondary-vol=1641346354431311872]
[2023/03/30 07:56:21.865 +00:00] [DEBUG] [coordinator.go:145] ["write block succed"] [ino=27] [block-id=fbe3b0100447a0958429e633be733afcf9a83bbf365830f9b349728bd8517ac6] [block-index=0] [primary-vol=1641346311947206656] [secondary-vol=1641346354431311872]
[2023/03/30 07:56:21.866 +00:00] [DEBUG] [cmd_io.go:267] ["write success"] [index=1] [data-len=524288]
[2023/03/30 07:56:21.866 +00:00] [DEBUG] [cmd_io.go:267] ["write success"] [index=0] [data-len=524288]
[2023/03/30 07:56:21.867 +00:00] [DEBUG] [cmd_io.go:209] ["writing file to hugo"] [name=/tmp/mock551987432]
[2023/03/30 07:56:21.868 +00:00] [INFO] [cmd_io.go:237] ["success create ino"] [ino=28]
[2023/03/30 07:56:21.868 +00:00] [DEBUG] [impl.go:72] ["try connect to meta-server"] [meta-server=meta:26666]
[2023/03/30 07:56:21.870 +00:00] [DEBUG] [impl.go:82] ["establish the connection"] [meta-server=meta:26666]
[2023/03/30 07:56:21.874 +00:00] [INFO] [cmd_io.go:253] ["success read file to buffer"] [read=1048576] [size=1048576]
[2023/03/30 07:56:21.875 +00:00] [DEBUG] [impl.go:128] ["start connect to storage"] [addr=s2:27778]
[2023/03/30 07:56:21.877 +00:00] [DEBUG] [impl.go:128] ["start connect to storage"] [addr=s3:27779]
[2023/03/30 07:56:21.878 +00:00] [DEBUG] [coordinator.go:111] ["select storage"] [primary=StorageClientImpl(s2:27778)] [secondary=StorageClientImpl(s3:27779)]
[2023/03/30 07:56:21.878 +00:00] [DEBUG] [coordinator.go:111] ["select storage"] [primary=StorageClientImpl(s2:27778)] [secondary=StorageClientImpl(s3:27779)]
[2023/03/30 07:56:21.890 +00:00] [DEBUG] [coordinator.go:145] ["write block succed"] [ino=28] [block-id=439a3f185afa8c8d6d208c0f95b827aa9eeee07a455100947cf3eac5f7df230c] [block-index=0] [primary-vol=1641346311947206656] [secondary-vol=1641346354431311872]
[2023/03/30 07:56:21.891 +00:00] [DEBUG] [cmd_io.go:267] ["write success"] [index=0] [data-len=524288]
[2023/03/30 07:56:21.894 +00:00] [DEBUG] [coordinator.go:145] ["write block succed"] [ino=28] [block-id=7c0dd7b94f03d540cbdf68352b3eadfe3ab772bfad4d45e7c5a54fefb7879b07] [block-index=1] [primary-vol=1641346311947206656] [secondary-vol=1641346354431311872]
[2023/03/30 07:56:21.895 +00:00] [DEBUG] [cmd_io.go:267] ["write success"] [index=1] [data-len=524288]

```
