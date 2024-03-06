package volume

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path"
	"testing"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/pingcap/log"
	"github.com/stretchr/testify/require"
)

func TestLocalPhysicalVolume(t *testing.T) {
	prefix, err := os.MkdirTemp("", "phy_local")
	require.Nil(t, err)
	t.Cleanup(func() {
		os.RemoveAll(prefix)
	})

	dbPath := path.Join(prefix, "badger")
	err = os.Mkdir(dbPath, os.ModePerm)
	require.Nil(t, err)

	fmt.Println("create local volume at ", prefix)
	fmt.Println("create local volume storage  at ", dbPath)

	cfg := &Config{
		Engine: engine.EngineTypeFS,
		Prefix: prefix,
		DB: adaptor.DbOption{
			Name: store.BadgerName,
			Arg:  dbPath,
		},
	}
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	messenger := NewMessenger()
	go func() {
		for {
			select {
			case <-messenger.volStatCh:
			case <-messenger.blockMetaCh:
			case <-ctx.Done():
				return

			}
		}
	}()

	lg := zap.NewNop()

	localVol, err := NewLocalPhysicalVolume(ctx, cfg, messenger, lg)
	require.Nil(t, err)
	require.NotNil(t, localVol)

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
			h := sha256.New()
			h.Write(tt.data)
			hash := h.Sum(nil)
			blockID, _, err := localVol.WriteBlock(ctx, &engine.DataContainer{Data: tt.data, Hash: hash})
			require.Nil(t, err)

			data, header, err := localVol.ReadBlock(ctx, blockID)
			require.Nil(t, err)
			require.EqualValues(t, len(tt.data), header.DataLen)
			require.EqualValues(t, tt.data, data)
		})
	}

	localVol.Shutdown(ctx)

	localVol2, err := NewLocalPhysicalVolume(ctx, cfg, messenger, lg)
	require.Nil(t, err)
	require.NotNil(t, localVol2)

	tests = []struct {
		index int
		data  []byte
	}{
		{
			index: 3,
			data:  []byte{5, 4, 3, 2},
		},
		{
			index: 4,
			data:  []byte("hello golang"),
		},
		{
			index: 5,
			data:  []byte("hugo rocks ???"),
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("block-%d", tt.index), func(t *testing.T) {
			h := sha256.New()
			h.Write(tt.data)
			hash := h.Sum(nil)
			blockID, _, err := localVol2.WriteBlock(ctx, &engine.DataContainer{Data: tt.data, Hash: hash})
			require.Nil(t, err)

			data, header, err := localVol2.ReadBlock(ctx, blockID)
			require.Nil(t, err)
			require.EqualValues(t, len(tt.data), header.DataLen)
			require.EqualValues(t, tt.data, data)
		})
	}
	localVol2.Shutdown(ctx)

}

func TestLocalVolumeCacheEvict(t *testing.T) {
	prefix, err := os.MkdirTemp("", "phy_local")
	require.Nil(t, err)
	t.Cleanup(func() {
		os.RemoveAll(prefix)
	})

	dbPath := path.Join(prefix, "badger")
	err = os.Mkdir(dbPath, os.ModePerm)
	require.Nil(t, err)

	fmt.Println("create local volume at ", prefix)
	fmt.Println("create local volume storage  at ", dbPath)

	cfg := &Config{
		Engine:    engine.EngineTypeFS,
		Prefix:    prefix,
		Quota:     Ptr(size.SizeSuffix(20)),
		Threhold:  Ptr(0.75),
		BlockSize: Ptr(size.SizeSuffix(10)),
		DB: adaptor.DbOption{
			Name: store.BadgerName,
			Arg:  dbPath,
		},
	}

	log.SetLevel(zapcore.DebugLevel)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	messenger := NewMessenger()
	go func() {
		for {
			select {
			case <-messenger.volStatCh:
			case <-messenger.blockMetaCh:
			case <-ctx.Done():
				return

			}
		}
	}()

	lg := zap.NewNop()
	localVol, err := NewLocalPhysicalVolume(ctx, cfg, messenger, lg)
	require.Nil(t, err)
	require.NotNil(t, localVol)

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
			h := sha256.New()
			h.Write(tt.data)
			hash := h.Sum(nil)
			blockID, _, err := localVol.WriteBlock(ctx, &engine.DataContainer{Data: tt.data, Hash: hash})
			require.Nil(t, err)

			data, header, err := localVol.ReadBlock(ctx, blockID)
			require.Nil(t, err)
			require.EqualValues(t, len(tt.data), header.DataLen)
			require.EqualValues(t, tt.data, data)
		})
	}

	localVol.Shutdown(ctx)
}
