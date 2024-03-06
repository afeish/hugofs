package vfs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type vfsSuite struct {
	suite.Suite

	storage *gateway.StorageHarness
	ctx     context.Context
	cancel  context.CancelFunc

	vfs *VFS

	lg *zap.Logger

	tempdir string
}

func (s *vfsSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.storage = gateway.NewStorageHarness(
		s.ctx,
		gateway.WithVolSize(size.MebiByte.Int()),
	)

	lg, _ := zap.NewDevelopment()
	s.lg = lg.Named("vfsSuite")
	s.tempdir, _ = os.MkdirTemp("", "hugovfs")

	opts := &mountlib.Options{
		DirCacheTime: time.Second * 1,
		AttrTimeout:  time.Second * 1,
		Logger:       lg,

		IO: adaptor.IoOption{
			Debug:  true,
			Prefix: s.tempdir,
		},
		Env: global.EnvCfg{
			Cache: struct {
				MemSize       size.SizeSuffix `env:"HUGO_CACHE_MEM_SIZE" envDefault:"256M"`
				FileWriteSize size.SizeSuffix `env:"HUGO_CACHE_FILE_SIZE" envDefault:"2G"`
				FileReadSize  size.SizeSuffix `env:"HUGO_CACHE_FILE_READ_SIZE" envDefault:"2G"`
				Dir           string          `env:"HUGO_CACHE_DIR" envDefault:"/tmp/hugo/cache"`
			}{
				MemSize: size.KibiByte,
			},
			BlockSize: 16,
			PageSize:  4,
		},
	}
	fs := NewFs(s.storage.MetaServiceClient, s.storage, opts)
	s.vfs = New(fs, opts)
}

func (s *vfsSuite) TeardownSuite() {
	node, err := s.vfs.Walk("/")
	s.Nil(err)
	s.T().Logf("root tree: \n%s", node.ToPrint())

	err = s.vfs.Rmdir(s.ctx, "/_dir1/_dir3", false)
	s.Equal(ENOTEMPTY, err)

	err = s.vfs.Rmdir(s.ctx, "/_dir1/_dir3", true)
	s.Nil(err)

	_dir3, err := s.vfs.Stat("/_dir1/_dir3")
	s.Nil(_dir3)
	s.Equal(ENOENT, err)

	s.T().Logf("root tree: \n%s", node.ToPrint())

	err = s.vfs.Remove(s.ctx, "/_hello.txt")
	s.Nil(err)

	_hello_txt, err := s.vfs.Stat("/_hello.txt")
	s.Nil(_hello_txt)
	s.Equal(ENOENT, err)

	s.T().Logf("root tree: \n%s", node.ToPrint())

	err = os.RemoveAll(s.tempdir)
	s.Nil(err)
}

func (s *vfsSuite) TestListFile() {
	s.vfs.Stat("_hello.txt")
	s.vfs.Stat("_hello.txt")
	time.Sleep(time.Second * 2)
	node, err := s.vfs.Stat("_hello.txt")
	s.Nil(err)
	s.lg.Debug("_hello.txt", zap.Any("node", node.Path()))
}

func (s *vfsSuite) TestListDir() {
	node, err := s.vfs.Stat("_dir1/_dir3")
	s.Nil(err)
	s.Equal("_dir1/_dir3", node.Path())
}

func (s *vfsSuite) TestListFile2() {
	node, err := s.vfs.Stat("_dir1/_dir3/bar.txt")
	s.Nil(err)
	s.Equal("_dir1/_dir3/bar.txt", node.Path())
}

func (s *vfsSuite) TestListRoot() {
	node, err := s.vfs.Walk("/")
	s.Nil(err)
	s.T().Logf("\n%s", node.ToPrint())

	node, err = s.vfs.Walk("_dir1")
	s.Nil(err)
	s.T().Logf("\n%s", node.ToPrint())
}

func (s *vfsSuite) TestListRoot2() {
	node, err := s.vfs.Walk("_dir1")
	s.Nil(err)
	s.T().Logf("\n%s", node.ToPrint())
}

func (s *vfsSuite) TestStatParent() {
	parent, leaf, err := s.vfs.StatParent("_dir1/_dir3/bar.txt")
	s.Nil(err)
	s.Equal("_dir1/_dir3", parent.Path())
	s.Equal("bar.txt", leaf)
}

func (s *vfsSuite) TestReadDir() {
	infos, err := s.vfs.ReadDir("_dir1/")
	s.Nil(err)
	for _, info := range infos {
		s.T().Logf("\n%s", info.Name())
	}
}

func (s *vfsSuite) TestMkDir() {
	node, err := s.vfs.Walk("/")
	s.Nil(err)
	s.T().Logf("root tree: \n%s", node.ToPrint())

	dir := node.(*Dir)
	hello, err := dir.Mkdir(s.ctx, "hello", 0755)
	s.NotNil(hello)
	s.Nil(err)

	err = s.vfs.Mkdir(s.ctx, "_dir1/_dir3/_hello", os.FileMode(0400))
	s.Nil(err)
	s.T().Logf("root tree: \n%s", node.ToPrint())

}

func (s *vfsSuite) TestCreate() {
	node, err := s.vfs.Walk("/_dir1")
	s.Nil(err)
	s.T().Logf("/_dir1: \n%s", node.ToPrint())

	err = s.vfs.Mkdir(s.ctx, "_dir1/_dir3/hello", os.FileMode(0400))
	s.Nil(err)

	s.T().Logf("/_dir1: \n%s", node.ToPrint())

	fn := func(c chan struct{}) {
		ticker := time.NewTicker(time.Millisecond * 300)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				node, err := s.vfs.Walk("/_dir1")
				s.Nil(err)
				s.T().Logf("/_dir1: \n%s", node.ToPrint())
			case <-c:
				return
			}
		}
	}

	c := make(chan struct{})
	go fn(c)

	go func() {
		times := 10
		var interval time.Duration = time.Millisecond * 500
		var t *time.Timer = time.NewTimer(interval)
		var name string
		for i := 0; i < times; i++ {
			t.Reset(interval)
			if i%5 == 0 {
				name = fmt.Sprintf("_dir1/hello_%d.txt", i)
			} else if i%3 == 0 {
				name = fmt.Sprintf("_dir1/_dir3/hello_%d.txt", i)
			} else {
				name = fmt.Sprintf("_dir1/_dir3/hello/hello_%d.txt", i)
			}
			h, err := s.vfs.OpenFile(name, os.O_RDWR|os.O_CREATE, os.FileMode(0400))
			s.Nil(err)
			h.Close()

			<-t.C
		}

		<-time.After(interval)
		close(c)
	}()

	<-c
}

func (s *vfsSuite) TestWrite() {
	h, err := s.vfs.OpenFile("hello.txt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() {
		s.Nil(h.Close())
	}()
	s.Nil(err)
	n, err := h.WriteAt([]byte("hello world"), 0)
	s.Nil(err)
	s.Equal(11, n)
}

func (s *vfsSuite) TestWriteDebugBatch() {

	type data struct {
		Data string `fake:"{sentence:50}"` // Can call with parameters
		// ino  uint64

		Name string `fake:"{firstname}"` // Any available function all lowercase
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

	genTestData := func() data {
		var td data
		gofakeit.Struct(&td)
		return td
	}

	type dataEval struct {
		data
		off      int64
		expected int64
	}

	var tests []dataEval
	for i := 0; i < 1_0; i++ {
		d := genTestData()
		tests = append(tests, dataEval{
			data:     d,
			off:      0,
			expected: int64(len(d.Data)),
		})
	}

	for _, test := range tests {
		test := test
		h, err := s.vfs.OpenFile(test.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		defer func() {
			s.Nil(h.Close())
		}()
		s.Nil(err)
		n, err := h.WriteAt([]byte(test.Data), 0)
		s.Nil(err)
		s.EqualValues(test.expected, n)

		d := make([]byte, len(test.Data))
		n, err = h.ReadAt(d, test.off)
		s.Nil(err)
		s.EqualValues(test.expected, n)
		s.EqualValues(test.Data, d)
	}
}

func (s *vfsSuite) TestWriteBatch() {
	type data struct {
		Data string `fake:"{sentence:50}"` // Can call with parameters
		// ino  uint64

		Name string `fake:"{firstname}"` // Any available function all lowercase
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

	genTestData := func() data {
		var td data
		gofakeit.Struct(&td)
		return td
	}

	type dataEval struct {
		data
		off      int64
		expected int64
	}

	var tests []dataEval
	for i := 0; i < 1_0; i++ {
		d := genTestData()
		tests = append(tests, dataEval{
			data:     d,
			off:      0,
			expected: int64(len(d.Data)),
		})
	}

	s.vfs.Opt.IO.Debug = false // test the real link

	for _, test := range tests {
		test := test
		h, err := s.vfs.OpenFile(test.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		defer func() {
			s.Nil(h.Close())
		}()
		s.Nil(err)
		n, err := h.WriteAt([]byte(test.Data), 0)
		s.Nil(err)
		s.EqualValues(test.expected, n)

		d := make([]byte, len(test.Data))
		n, err = h.ReadAt(d, test.off)
		s.Nil(err)
		s.EqualValues(test.expected, n)
		s.EqualValues(test.Data, d)
	}
}

func TestVfs(t *testing.T) {
	suite.Run(t, new(vfsSuite))
}
