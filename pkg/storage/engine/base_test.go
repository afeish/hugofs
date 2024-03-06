package engine

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash/crc32"
	"os"
	"testing"

	"github.com/afeish/hugo/pkg/storage/block"
	"github.com/stretchr/testify/require"
)

func TestNewFsEngine(t *testing.T) {
	prefix, err := os.MkdirTemp("", "engine")
	require.Nil(t, err)
	t.Cleanup(func() {
		os.RemoveAll(prefix)
	})

	engine, err := NewProvider(context.Background(), ProviderInfo{Type: EngineTypeFS, Config: &Config{Prefix: prefix}})
	require.Nil(t, err)

	tests := []struct {
		index int
		data  []byte
	}{
		{
			index: 0,
			data:  []byte{1, 2, 3, 4},
		},
		{
			index: 1,
			data:  []byte("hello world"),
		},
		{
			index: 2,
			data:  []byte("hugo rocks"),
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("block-%d", tt.index), func(t *testing.T) {
			orig := tt.data
			idx := uint64(tt.index)
			h := sha256.New()
			h.Write(orig)
			sum := h.Sum(nil)
			h1 := &block.Header{
				Kind:     1,
				DataLen:  uint64(len(orig)),
				Index:    idx,
				Crc32:    crc32.ChecksumIEEE(orig),
				Checksum: sum,
			}

			loc := &Location{1, idx}
			err := engine.WriteBlock(context.TODO(), loc, &DataContainer{Index: uint64(tt.index), Data: orig})
			require.NoError(t, err)

			h2, err := engine.GetBlockHeader(context.TODO(), loc)
			require.NoError(t, err)

			require.EqualValues(t, h1, h2)

			new, err := engine.GetBlockData(context.Background(), loc)
			require.NoError(t, err)

			require.Equal(t, orig, new)
		})
	}

}

func TestNewMemEngine(t *testing.T) {
	engine, err := NewProvider(context.Background(), ProviderInfo{Type: EngineTypeINMEM})
	require.Nil(t, err)
	tests := []struct {
		index int
		data  []byte
	}{
		{
			index: 0,
			data:  []byte{1, 2, 3, 4},
		},
		{
			index: 1,
			data:  []byte("hello world"),
		},
		{
			index: 2,
			data:  []byte("hugo rocks"),
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("block-%d", tt.index), func(t *testing.T) {
			orig := tt.data
			idx := uint64(tt.index)
			h := sha256.New()
			h.Write(orig)
			sum := h.Sum(nil)

			h1 := &block.Header{
				Kind:     1,
				DataLen:  uint64(len(orig)),
				Index:    idx,
				Crc32:    crc32.ChecksumIEEE(orig),
				Checksum: sum,
			}

			loc := &Location{1, idx}
			err := engine.WriteBlock(context.TODO(), loc, &DataContainer{Index: uint64(tt.index), Data: orig})
			require.NoError(t, err)

			h2, err := engine.GetBlockHeader(context.TODO(), loc)
			require.NoError(t, err)

			require.EqualValues(t, h1, h2)

			new, err := engine.GetBlockData(context.Background(), loc)
			require.NoError(t, err)

			require.Equal(t, orig, new)
		})
	}

}
