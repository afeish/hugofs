package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"testing"

	"github.com/afeish/hugo/pb/storage"
	"github.com/pingcap/log"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestOneNode(t *testing.T) {
	str := `
id: 0
ip: localhost
port: 30001
block_size: 512
section:
- id: 1
  engine: INMEM
  prefix: /tmp/phy0
  quota: 10240
  threhold: 0.75
db:
  db_name: memory
  db_arg: s0
test:
  enabled: true
  db_name: memory
  db_arg: meta0
object:
  bucket_name: test
  platform: memory
`
	log.SetLevel(zapcore.DebugLevel)

	cfg, err := NewConfigFromText(str)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	t.Log(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n1, err := NewNode(ctx, cfg)
	require.NoError(t, err)
	err = n1.Start(ctx)
	require.NoError(t, err)

	r1, err := n1.ListVolumes(ctx, &storage.ListVolumes_Request{})
	require.NoError(t, err)
	require.EqualValues(t, 1, len(r1.Volumes))
	log.Info("vol stat", zap.Uint64(VOL_ID, r1.Volumes[0].Id))

	tests := []struct {
		index uint64
		data  string
	}{
		{index: 0, data: "hello world"},
		{index: 1, data: "hugo rocks"},
		{index: 2, data: "dlrow olleh"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.index), func(t *testing.T) {
			crc := crc32.ChecksumIEEE([]byte(tt.data))
			h := sha256.New()
			h.Write([]byte(tt.data))
			hash := h.Sum(nil)
			blockId := hex.EncodeToString(hash)
			r1, err := n1.Write(ctx, &storage.WriteBlock_Request{BlockData: []byte(tt.data), Crc32: crc, BlockIdx: lo.ToPtr(tt.index)})
			require.NoError(t, err)
			require.NotNil(t, r1)
			r2, err := n1.Read(ctx, &storage.Read_Request{BlockIds: []string{blockId}})
			require.NoError(t, err)
			require.Len(t, r2.Blocks, 1)
			require.EqualValues(t, tt.data, r2.Blocks[0].Data)
		})
	}
}

func TestThreeNodes(t *testing.T) {
	node1Str := `
id: 1
ip: localhost
port: 20001
block_size: 512
section:
- id: 1
  engine: INMEM
  prefix: /tmp/phy1
  quota: 10240
  threhold: 0.75
db:
  db_name: memory
  db_arg: s1
test:
  enabled: true
  db_name: memory
  db_arg: meta
object:
  bucket_name: test
  platform: memory
`
	node2Str := `
id: 1
ip: localhost
port: 20002
block_size: 512
section:
- id: 1
  engine: INMEM
  prefix: /tmp/phy2
  quota: 10240
  threhold: 0.75
db:
  db_name: memory
  db_arg: s2
test:
  enabled: true
  db_name: memory
  db_arg: meta
object:
  bucket_name: test
  platform: memory
`
	node3Str := `
id: 1
ip: localhost
port: 20003
block_size: 512
section:
- id: 1
  engine: INMEM
  prefix: /tmp/phy3
  quota: 10240
  threhold: 0.75
db:
  db_name: memory
  db_arg: s3
test:
  enabled: true
  db_name: memory
  db_arg: meta
object:
  bucket_name: test
  platform: memory
`
	log.SetLevel(zapcore.DebugLevel)

	cfg1, err := NewConfigFromText(node1Str)
	require.NoError(t, err)
	require.NotNil(t, cfg1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n1, err := NewNode(ctx, cfg1)
	require.NoError(t, err)
	err = n1.Start(ctx)
	require.NoError(t, err)
	r1, err := n1.ListVolumes(ctx, &storage.ListVolumes_Request{})
	require.NoError(t, err)
	require.EqualValues(t, 1, len(r1.Volumes))
	require.EqualValues(t, "/tmp/phy1", r1.Volumes[0].Prefix)

	cfg2, err := NewConfigFromText(node2Str)
	require.NoError(t, err)
	require.NotNil(t, cfg2)
	n2, err := NewNode(ctx, cfg2)
	require.NoError(t, err)
	err = n2.Start(ctx)
	require.NoError(t, err)
	r2, err := n2.ListVolumes(ctx, &storage.ListVolumes_Request{})
	require.NoError(t, err)
	require.EqualValues(t, 1, len(r2.Volumes))
	require.EqualValues(t, "/tmp/phy2", r2.Volumes[0].Prefix)

	cfg3, err := NewConfigFromText(node3Str)
	require.NoError(t, err)
	require.NotNil(t, cfg3)
	n3, err := NewNode(ctx, cfg3)
	require.NoError(t, err)
	err = n3.Start(ctx)
	require.NoError(t, err)
	r3, err := n3.ListVolumes(ctx, &storage.ListVolumes_Request{})
	require.NoError(t, err)
	require.EqualValues(t, 1, len(r3.Volumes))
	require.EqualValues(t, "/tmp/phy3", r3.Volumes[0].Prefix)
}
