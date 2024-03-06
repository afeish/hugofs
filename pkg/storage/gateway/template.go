package gateway

var storageTemplate = `
id: 0
ip: localhost
port: 20001
block_size: {{ .block_size }}
section:
- id: 1
  engine: INMEM
  prefix: /tmp/phy1
  quota: {{ .vol_size }}
  threhold: 1.00
db:
  db_name: memory
  db_arg: s1
test:
  enabled: true
  db_name:  {{ .test_db_name }}
  db_arg: {{ .test_db_arg}}
object:
  platform: memory
  bucket_name: test
---
id: 1
ip: localhost
port: 20002
block_size: {{ .block_size }}
section:
- id: 1
  engine: INMEM
  prefix: /tmp/phy2
  quota: {{ .vol_size }}
  threhold: 1.00
db:
  db_name: memory
  db_arg: s2
test:
  enabled: true
  db_name:  {{ .test_db_name }}
  db_arg: {{ .test_db_arg}}
object:
  platform: memory
  bucket_name: test
---

id: 2
ip: localhost
port: 20003
block_size: {{ .block_size }}
section:
- id: 1
  engine: INMEM
  prefix: /tmp/phy3
  quota: {{ .vol_size }}
  threhold: 1.00
db:
  db_name: memory
  db_arg: s3
test:
  enabled: true
  db_name:  {{ .test_db_name }}
  db_arg: {{ .test_db_arg}}
object:
  platform: memory
  bucket_name: test
`

var metaTemplate = `
db:
  db_name: memory
  db_arg: meta
test:
  enabled: true
  db_name:  {{ .test_db_name }}
  db_arg: {{ .test_db_arg}}
io:
  transport: MOCK
  block_size: {{ .block_size }}
  replica: 1
`

var inodeTemplate = `
name: _dir1
size: 4096
mode: 0644
is_directory: true
entries:
- name: _dir2
  size: 4096
  mode: 0644
  is_directory: true
- name: _dir3
  size: 4096
  mode: 0644
  is_directory: true
  entries:
  - name: bar.txt
    size: 16
    mode: 0644
    is_directory: false
- name: foo.txt
  size: 20
  mode: 0644
  is_directory: false
---

name: _hello.txt
size: 16
mode: 0644
is_directory: false

---
name: _dir4
size: 4096
mode: 0644
is_directory: true
`
