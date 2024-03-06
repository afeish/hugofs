<!-- https://github.com/etcd-io/etcd/tree/main/contrib/raftexample  -->
# Use goreman to run `go install github.com/mattn/goreman@latest`

<!-- meta server clusters -->
m1: ./bin/hugo meta start --dbname badger --dbarg /tmp/db_hugo_meta --port 10001

<!-- data server clusters -->
d1: ./bin/hugo storage start -m  127.0.0.1:10001 --port 20001 --mock
d2: ./bin/hugo storage start -m  127.0.0.1:10001 --port 20002 --mock
d3: ./bin/hugo storage start -m  127.0.0.1:10001 --port 20003 --mock

<!-- v1: ./bin/data-server volume add -s 127.0.0.1:20001 -p /tmp/hello -q 10 -->
