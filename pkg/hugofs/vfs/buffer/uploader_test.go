package buffer

import (
	"os"
	"testing"

	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/util/mem"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	testBlockSize int64 = 16
	testPageSize  int64 = 4
)

func TestUploadThroughFile(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// logger, _ := zap.NewDevelopment()
	logger := zap.NewNop()
	up := NewUploadPipeline(1, adaptor.NewMemStorageBackend(testBlockSize), &mockRandWriteMonitor{}, logger,
		meta.WithBlockSize(testBlockSize),
		meta.WithPageSize(testPageSize),
		meta.WithLimit(1),
		meta.WithCacheSize(testBlockSize),
		meta.WithCacheDir(dir),
	)

	require.EqualValues(t, testBlockSize, up.opts.BlockSize)

	var (
		n   int
		err error
	)
	n, err = up.WriteAt(mem.Align([]byte("hello world"), testBlockSize), testBlockSize*0)
	require.EqualValues(t, testBlockSize, n)
	require.Nil(t, err)
	n, err = up.WriteAt(mem.Align([]byte("hello world"), testBlockSize), testBlockSize*1)
	require.EqualValues(t, testBlockSize, n)
	require.Nil(t, err)
	n, err = up.WriteAt(mem.Align([]byte("hello world"), testBlockSize), testBlockSize*3)
	require.EqualValues(t, testBlockSize, n)
	require.Nil(t, err)
	n, err = up.WriteAt(mem.Align([]byte("hello world"), testBlockSize), testBlockSize*4)
	require.EqualValues(t, testBlockSize, n)
	require.Nil(t, err)

}

func TestUploadHalfBlock(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// logger, _ := zap.NewDevelopment()
	logger := zap.NewNop()
	up := NewUploadPipeline(1, adaptor.NewMemStorageBackend(testBlockSize), &mockRandWriteMonitor{}, logger,
		meta.WithBlockSize(testBlockSize),
		meta.WithPageSize(testPageSize),
		meta.WithLimit(1),
		meta.WithCacheSize(testBlockSize),
		meta.WithCacheDir(dir),
	)

	require.EqualValues(t, testBlockSize, up.opts.BlockSize)

	var testdata = []struct {
		data []byte
		off  int64
	}{
		{
			data: []byte("hello world"),
			off:  0,
		},
		{
			data: []byte("hello hugo"),
			off:  testBlockSize * 1,
		},
		{
			data: []byte("dlrow olleh"),
			off:  testBlockSize * 2,
		},
	}

	for _, d := range testdata {
		t.Run("", func(t *testing.T) {
			n, err := up.WriteAt(d.data, d.off)
			require.EqualValues(t, len(d.data), n)
			require.Nil(t, err)
			logger.Sugar().Debugf("usage of file: %v", up.swapFile.OccupiedUsage())
		})
	}

}

func TestUploadMem(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// logger, _ := zap.NewDevelopment()
	logger := zap.NewNop()
	up := NewUploadPipeline(1, adaptor.NewMemStorageBackend(testBlockSize), &mockSeqWriteMonitor{}, logger,
		meta.WithBlockSize(testBlockSize),
		meta.WithLimit(2),
		meta.WithCacheSize(testBlockSize*2),
		meta.WithCacheDir(dir),
	)

	require.EqualValues(t, testBlockSize, up.opts.BlockSize)

	var (
		n   int
		err error
	)
	n, err = up.WriteAt(mem.Align([]byte("hello world"), testBlockSize), testBlockSize*0)
	require.EqualValues(t, testBlockSize, n)
	require.Nil(t, err)
	n, err = up.WriteAt(mem.Align([]byte("hello world"), testBlockSize), testBlockSize*1)
	require.EqualValues(t, testBlockSize, n)
	require.Nil(t, err)
	n, err = up.WriteAt(mem.Align([]byte("hello world"), testBlockSize), testBlockSize*3)
	require.EqualValues(t, testBlockSize, n)
	require.Nil(t, err)
	n, err = up.WriteAt(mem.Align([]byte("hello world"), testBlockSize), testBlockSize*4)
	require.EqualValues(t, testBlockSize, n)
	require.Nil(t, err)

}

type mockRandWriteMonitor struct {
}

func (m *mockRandWriteMonitor) Monitor(offset int64, size int) {
}
func (m *mockRandWriteMonitor) IsSequentialMode() bool {
	return false
}

type mockSeqWriteMonitor struct {
}

func (m *mockSeqWriteMonitor) Monitor(offset int64, size int) {
}
func (m *mockSeqWriteMonitor) IsSequentialMode() bool {
	return true
}
