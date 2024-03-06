package block

import (
	"bytes"
	"os"
	"testing"

	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	testBlockSize int64 = 16
	testPageSize  int64 = 4
)

func TestSwapFile(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// lg, _ := zap.NewDevelopment()
	lg := zap.NewNop()
	storage := adaptor.NewMemStorageBackend(testBlockSize)

	// dir, int64(testBlockSize), int64(testPageSize),
	sf := NewSwapFile(1, storage, lg, meta.WithBlockSize(testBlockSize), meta.WithPageSize(testPageSize), meta.WithCacheDir(dir), meta.WithCacheSize(testBlockSize*4))

	b1, err := sf.NewFileBlock(meta.NewBlockKey(1, 0))
	require.Nil(t, err)

	n, err := b1.WriteAt(bytes.Repeat([]byte{'a'}, int(testBlockSize)), 0)
	require.Nil(t, err)
	require.EqualValues(t, testBlockSize, n)

	require.True(t, b1.IsFull())

	p := make([]byte, testBlockSize)
	n, err = b1.ReadAt(p, 0)
	require.Nil(t, err)
	require.EqualValues(t, testBlockSize, n)

	require.EqualValues(t, bytes.Repeat([]byte{'a'}, int(testBlockSize)), p)
}

func TestSwapFileEvict(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// lg, _ := zap.NewDevelopment()
	lg := zap.NewNop()
	storage := adaptor.NewMemStorageBackend(testBlockSize)

	sf := NewSwapFile(1, storage, lg,
		meta.WithBlockSize(testBlockSize), meta.WithPageSize(testPageSize),
		meta.WithCacheDir(dir), meta.WithCacheSize(testBlockSize),
		meta.WithPageCtx(meta.NewPageContext(testBlockSize, testPageSize)),
	)

	b1, err := sf.NewFileBlock(meta.NewBlockKey(1, 0))
	require.Nil(t, err)

	var offset int64

	n, err := b1.WriteAt(bytes.Repeat([]byte{'a'}, int(testBlockSize)), offset)
	require.Nil(t, err)
	require.EqualValues(t, testBlockSize, n)

	require.True(t, b1.IsFull())

	p := make([]byte, testBlockSize)

	n, err = b1.ReadAt(p, offset)
	require.Nil(t, err)
	require.EqualValues(t, testBlockSize, n)

	require.EqualValues(t, bytes.Repeat([]byte{'a'}, int(testBlockSize)), p)
	require.EqualValues(t, int64(testBlockSize/testPageSize), sf.opts.PageCtx.GetPages())

	b2, err := sf.NewFileBlock(meta.NewBlockKey(1, 1))
	require.Nil(t, err)

	offset = testBlockSize
	n, err = b2.WriteAt(bytes.Repeat([]byte{'b'}, int(testBlockSize)), offset)
	require.Nil(t, err)
	require.EqualValues(t, testBlockSize, n)

	require.True(t, b2.IsFull())

	p = make([]byte, testBlockSize)
	n, err = b2.ReadAt(p, offset)
	require.Nil(t, err)
	require.EqualValues(t, testBlockSize, n)
	require.EqualValues(t, bytes.Repeat([]byte{'b'}, int(testBlockSize)), p)
}
