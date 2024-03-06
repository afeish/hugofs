

# Test the storage commponent

## Bootstrap the test environment
Boot
```shell
make dev 
```

## Mount volume against specified storage node
```shell
docker exec -it g_s1 bash
root@g_s1:/# hugo s v l 
[2023/02/27 08:58:59.098 +00:00] [INFO] [main.go:50] ["debug mode"]
root@g_s1:/# hugo s v a -p /mnt/1-1 
[2023/02/27 08:59:09.668 +00:00] [INFO] [main.go:50] ["debug mode"]
engine: FS
success register to node  1630129891359916032
root@g_s1:/# hugo s v a -p /mnt/1-2
[2023/02/27 08:59:11.603 +00:00] [INFO] [main.go:50] ["debug mode"]
engine: FS
success register to node  1630129891359916032
root@g_s1:/# hugo s v l
[2023/02/27 08:59:14.907 +00:00] [INFO] [main.go:50] ["debug mode"]
id:1630130413550764032 prefix:"/mnt/1-2" quota:42949672960 threhold:0.75 node_id:1630129891359916032 created_at:1677488351
id:1630130405426397184 prefix:"/mnt/1-1" quota:42949672960 threhold:0.75 node_id:1630129891359916032 created_at:1677488349

```
## Write some block against specified storage node

```shell
root@g_s1:/# dd if=/dev/urandom of=1m bs=1M count=1 
1+0 records in
1+0 records out
1048576 bytes (1.0 MB, 1.0 MiB) copied, 0.00337204 s, 311 MB/s
root@g_s1:/# dd if=/dev/urandom of=2m bs=1M count=2
2+0 records in
2+0 records out
4194304 bytes (4.2 MB, 4.0 MiB) copied, 0.0128621 s, 326 MB/s
root@g_s1:/# hugo s io w -f 1m
[2023/02/27 09:02:08.126 +00:00] [INFO] [main.go:50] ["debug mode"]
ino:1 block_idx:1 block_id:"ee4ffc659208888ef4c572488e35cd765b2028f9baacda147d994b2978c2a451" vol_id:1630130405426397184 node_id:1630129891359916032 crc32:3636460043 len:4194304
root@g_s1:/# hugo s io w -f 2m
[2023/02/27 09:03:18.098 +00:00] [INFO] [main.go:50] ["debug mode"]
ino:1 block_idx:1 block_id:"5cc6285c02a0968ef9b93965f418d68f22a7858adb6b5a66b7e38c8bf0922310" vol_id:1630130413550764032 node_id:1630129891359916032 crc32:3356461909 len:2097152
root@g_s1:/# hugo s io l      
[2023/02/27 09:03:24.490 +00:00] [INFO] [main.go:50] ["debug mode"]
block_id:"ee4ffc659208888ef4c572488e35cd765b2028f9baacda147d994b2978c2a451" node_id:1630129891359916032 vol_id:1630130405426397184
block_id:"5cc6285c02a0968ef9b93965f418d68f22a7858adb6b5a66b7e38c8bf0922310" node_id:1630129891359916032 vol_id:1630130413550764032
```
## Read block against specified storage node

We are sure that the restore data block's md5sum remains same
```shell
root@g_s1:/tmp/tmp# hugo s io r -i ee4ffc659208888ef4c572488e35cd765b2028f9baacda147d994b2978c2a451
[2023/02/27 09:05:25.425 +00:00] [INFO] [main.go:50] ["debug mode"]
index=0, crc32=3636460043, len=4194304, id=ee4ffc659208888ef4c572488e35cd765b2028f9baacda147d994b2978c2a451
root@g_s1:/tmp/tmp# md5sum ee4ffc659208888ef4c572488e35cd765b2028f9baacda147d994b2978c2a451
ab12e04aa792cfbfe5a6feac8ca32d4b  ee4ffc659208888ef4c572488e35cd765b2028f9baacda147d994b2978c2a451
root@g_s1:/tmp/tmp# md5sum /1m
ab12e04aa792cfbfe5a6feac8ca32d4b  /1m

```

## Destroy the test environment

```shell
make dev-clean
```
