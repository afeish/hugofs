package buffer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// func TestMain(m *testing.M) {
// 	goleak.VerifyTestMain(m)
// }

func TestLimit(t *testing.T) {
	t.Skip()
	l := NewThrottler(context.Background(), 2, zap.NewNop())
	var counter atomic.Int32
	go func() {
		l._acquire()
		counter.Add(1)
		<-time.After(time.Second)
		l._release()
		counter.Add(-1)
	}()

	go func() {
		l._acquire()
		counter.Add(1)
		<-time.After(time.Second)
		l._release()
		counter.Add(-1)
	}()

	go func() {
		l._acquire()
		counter.Add(1)
		<-time.After(time.Second)
		l._release()
		counter.Add(-1)
	}()

	go func() {
		l._acquire()
		counter.Add(1)
		<-time.After(time.Second)
		l._release()
		counter.Add(-1)
	}()

	ticker := time.NewTicker(time.Second * 1)
	endC := time.After(time.Second * 3)
	for {
		select {
		case <-ticker.C:
			require.LessOrEqual(t, counter.Load(), int32(2))
		case <-endC:
			return
		}
	}
}

func TestThrottler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := NewThrottler(ctx, 2, zap.NewNop())
	defer l.Shutdown()

	storage := adaptor.NewMemStorageBackend(testBlockSize)
	storage.Write(ctx, meta.NewBlockKey(1, 0), []byte("hello world"))
	storage.Write(ctx, meta.NewBlockKey(2, 0), []byte("world hello"))
	storage.Write(ctx, meta.NewBlockKey(3, 0), []byte("hugo rocks"))

	lg := zap.NewNop()
	file1 := NewDefaultBlockBuffer(l, 1, testBlockSize, storage, lg)
	file2 := NewDefaultBlockBuffer(l, 2, testBlockSize, storage, lg)
	file3 := NewDefaultBlockBuffer(l, 3, testBlockSize, storage, lg)

	buf := make([]byte, 16)
	n, err := file3.Read(buf, 0)
	require.Nil(t, err)
	require.GreaterOrEqual(t, n, 0)

	n, err = file1.Read(buf, 0)
	require.Nil(t, err)
	require.GreaterOrEqual(t, n, 0)

	n, err = file3.Read(buf, 0)
	require.Nil(t, err)
	require.GreaterOrEqual(t, n, 0)

	n, err = file2.Read(buf, 0)
	require.Nil(t, err)
	require.GreaterOrEqual(t, n, 0)

	n, err = file1.Read(buf, 0)
	require.Nil(t, err)
	require.GreaterOrEqual(t, n, 0)
}
