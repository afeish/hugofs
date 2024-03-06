# Perf test

## Perf on aws
```shell
# seq write
fio --name=hugo-test --directory=/home/ubuntu/hugo --ioengine=libaio --rw=write --bs=1m --size=1g --numjobs=4 --direct=1 --group_reporting
# seq read
fio --name=hugo-test --directory=/home/ubuntu/hugo --ioengine=libaio --rw=read --bs=1m --size=1g --numjobs=4 --direct=1 --group_reporting

# random write
fio --name=hugo-test --directory=/home/ubuntu/hugo --ioengine=libaio --rw=randwrite --bs=4k --size=1g --numjobs=4 --direct=1 --group_reporting
# random read
fio --name=hugo-test --directory=/home/ubuntu/hugo --ioengine=libaio --rw=randread --bs=4k --size=1g --numjobs=4 --direct=1 --group_reporting

fio --name=hugo-test --directory=/home/ubuntu/hugo --ioengine=libaio --rw=rw --bs=4k --size=1g --numjobs=4 --direct=1 --group_reporting

```

## Perf on docker
```shell

fio --name=hugo-test --directory=/hugo --ioengine=libaio --rw=write --bs=1m --size=1g --numjobs=4 --direct=1 --group_reporting
fio --name=hugo-test --directory=/hugo --ioengine=libaio --rw=read --bs=1m --size=1g --numjobs=4 --direct=1 --group_reporting

fio --name=hugo-test --directory=/hugo --ioengine=libaio --rw=randwrite --bs=4k --size=1g --numjobs=4 --direct=1 --group_reporting
fio --name=hugo-test --directory=/hugo --ioengine=libaio --rw=randread --bs=4k --size=1g --numjobs=4 --direct=1 --group_reporting
```
