//go:build !excludeTest

package buffer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	. "github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/util/mem"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type BufferTestSuite struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc

	blockSize int64

	storage adaptor.StorageBackend

	lg *zap.Logger
}

func (c *BufferTestSuite) SetupSuite() {
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.blockSize = testBlockSize

	// c.lg, _ = zap.NewDevelopment()
	c.lg = zap.NewNop()
	// c.storage = adaptor.NewMemStorageBackend(testBlockSize)
}

func (c *BufferTestSuite) newStorage(metaDBArg string) adaptor.StorageBackend {
	harness := gateway.NewStorageHarness(
		c.ctx,
		gateway.WithTestBlockSize(int(c.blockSize)),
		gateway.WithTestMetaArg(metaDBArg),
		gateway.WithTestLog(c.lg),
		gateway.WithTestLockedBackend(),
	)
	return harness
}

func (c *BufferTestSuite) BeforeTest(suiteName, testName string) {
	c.storage = c.newStorage(testName)
}

func (c *BufferTestSuite) AfterTest(suiteName, testName string) {
	c.storage.Destory()
}

func (c *BufferTestSuite) TearDownSuite() {
	c.cancel()
}

func (c *BufferTestSuite) TestRWMultipleFile() {
	for i := 0; i < 1_000; i++ {
		c.testSimpleTask(i)
	}
}

func (c *BufferTestSuite) TestRWMultipleFileConcurrent() {
	var wg sync.WaitGroup
	for i := 0; i < 1_000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c.testSimpleTask(idx)
		}(i)
	}
	wg.Wait()
}

func BenchmarkWriteThroughBuffer(b *testing.B) {
	c := new(BufferTestSuite)
	c.SetT(&testing.T{})
	c.SetupSuite()
	b.ResetTimer()

	c.storage = c.newStorage("default")

	var wg sync.WaitGroup
	for n := 0; n < b.N; n++ {
		b.StartTimer()

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c.testSimpleTask(idx)
		}(n)

		b.StopTimer()
	}
	wg.Wait()
	c.TearDownSuite()
}

func (c *BufferTestSuite) testSimpleTask(idx int) {
	opts := []Option[*meta.Options]{
		meta.WithBlockSize(c.blockSize),
	}

	buffer := NewDirectRWBuffer(uint64(idx), c.storage, NewThrottler(c.ctx, 2, c.lg), c.lg, opts...)
	c.simpleTask(buffer, uint64(idx))
	buffer.Destroy()
}

func (c *BufferTestSuite) simpleTask(buffer meta.RWBuffer, ino uint64) {
	type testdata struct {
		data  []byte
		ino   uint64
		index uint64
	}

	tests := []testdata{
		{
			data:  mem.Align([]byte("hello world"), c.blockSize),
			ino:   ino,
			index: 0,
		},
		{
			data:  mem.Align([]byte("dlrow olleh"), c.blockSize),
			ino:   ino,
			index: 1,
		},
		{
			data:  mem.Align([]byte("world hello"), c.blockSize),
			ino:   ino,
			index: 2,
		},
		{
			data:  mem.Align([]byte("hugo rocks"), c.blockSize),
			ino:   ino,
			index: 3,
		},
		{
			data:  mem.Align([]byte("skcor aiag"), c.blockSize),
			ino:   ino,
			index: 4,
		},
		{
			data:  bytes.Repeat([]byte{'a'}, int(c.blockSize)),
			ino:   ino,
			index: 5,
		},
	}

	for _, test := range tests {
		test := test
		n, err := buffer.Write(test.data, int64(test.index)*c.blockSize)
		c.Nil(err)
		c.EqualValues(len(test.data), n)
		data := make([]byte, len(test.data))
		_, err = buffer.Read(data, int64(test.index)*c.blockSize)
		c.Nil(err)
		c.Equal(test.data, data)
	}

	data := make([]byte, 0)
	lo.ForEach(tests, func(item testdata, _ int) {
		data = append(data, item.data...)
	})

	type testdata2 struct {
		buf       []byte
		ino       uint64
		off       uint64
		expectN   uint64
		expectErr error
	}

	tests2 := []testdata2{
		{
			buf:       make([]byte, 2),
			ino:       ino,
			off:       0,
			expectN:   2,
			expectErr: nil,
		},
		{
			buf:       make([]byte, 4),
			ino:       ino,
			off:       2,
			expectN:   4,
			expectErr: nil,
		},
		{
			buf:       make([]byte, 8),
			ino:       ino,
			off:       6,
			expectN:   8,
			expectErr: nil,
		},
		{
			buf:       make([]byte, c.blockSize),
			ino:       ino,
			off:       uint64(c.blockSize) + 6,
			expectN:   uint64(c.blockSize),
			expectErr: nil,
		},
		{
			buf:       make([]byte, c.blockSize),
			ino:       ino,
			off:       uint64(c.blockSize) + 11,
			expectN:   uint64(c.blockSize),
			expectErr: nil,
		},
		{
			buf:       make([]byte, c.blockSize),
			ino:       ino,
			off:       uint64(c.blockSize)*5 + 5,
			expectN:   uint64(c.blockSize - 5),
			expectErr: io.EOF,
		},
	}

	for _, test := range tests2 {
		test := test
		id := fmt.Sprintf("r-%d-%d-%d", test.ino, test.off, time.Now().UnixNano())
		n, err := buffer.Read(test.buf, int64(test.off))
		c.lg.Debug("test read finished", zap.Any("ino", test.ino), zap.Any("off", test.off), zap.String("id", id), zap.String("data", string(test.buf[:n])))

		// _ = err
		c.ErrorIs(err, test.expectErr)
		c.EqualValues(test.expectN, n)

		off := test.off
		end := test.off + uint64(len(test.buf))

		if !bytes.Equal(data[off:end], test.buf) {
			c.lg.Error(string(data[off:end])+"<<--->>"+string(test.buf), zap.Any("ino", test.ino), zap.Any("off", off), zap.Any("end", end), zap.String("id", id))
		}

		c.EqualValues(string(data[off:end]), string(test.buf), fmt.Sprintf("ino:%v, off:%v, end:%v, id: %v", test.ino, off, end, id))
	}

}

func (c *BufferTestSuite) TestKeepRWOneFile() {
	c.T().Skip()
	opts := []Option[*meta.Options]{
		meta.WithBlockSize(c.blockSize),
	}
	idx := 1

	buffer := NewDirectRWBuffer(uint64(idx), c.storage, NewThrottler(c.ctx, 2, c.lg), c.lg, opts...)
	c.dynamicTask(buffer, uint64(idx))
	buffer.Destroy()
}
func (c *BufferTestSuite) dynamicTask(buffer meta.RWBuffer, ino uint64) {
	type testdata struct {
		Data string `fake:"{sentence:10}"` // Can call with parameters
		ino  uint64

		// Name          string         `fake:"{firstname}"`  // Any available function all lowercase
		// Sentence      string         `fake:"{sentence:3}"` // Can call with parameters
		// RandStr       string         `fake:"{randomstring:[hello,world]}"`
		// Number        string         `fake:"{number:1,10}"`       // Comma separated for multiple values
		// Regex         string         `fake:"{regex:[abcdef]{5}}"` // Generate string from regex
		// Map           map[string]int `fakesize:"2"`
		// Array         []string       `fakesize:"2"`
		// ArrayRange    []string       `fakesize:"2,6"`
		// Skip          *string        `fake:"skip"` // Set to "skip" to not generate data for
		// Created       time.Time      // Can take in a fake tag as well as a format tag
		// CreatedFormat time.Time      `fake:"{year}-{month}-{day}" format:"2006-01-02"`
	}

	genTestData := func() testdata {
		var td testdata
		gofakeit.Struct(&td)
		td.ino = ino
		return td
	}

	var tests []testdata
	for i := 0; i < 1_000; i++ {
		tests = append(tests, genTestData())
	}

	var off int64
	for _, test := range tests {
		test := test
		n, err := buffer.Write([]byte(test.Data), off)
		c.Nil(err)
		c.EqualValues(len(test.Data), n)
		data := make([]byte, len(test.Data))
		_, err = buffer.Read(data, off)
		c.Nil(err)
		c.Equal(test.Data, string(data))
		off += int64(n)
	}

	data := make([]byte, 0)
	lo.ForEach(tests, func(item testdata, index int) {
		data = append(data, item.Data...)
	})

}

func TestRWBuffer(t *testing.T) {
	suite.Run(t, new(BufferTestSuite))
}
