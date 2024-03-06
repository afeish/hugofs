//go:build !excludeTest

package tests

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/util"
	"github.com/afeish/hugo/pkg/util/grace"
	"github.com/afeish/hugo/pkg/util/size"
	"go.uber.org/zap"

	"github.com/pingcap/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type hugoTestSuite struct {
	gateway.StorageHarness

	mp  string
	dbn string // meta db name
	dbp string // meta db arg

	ctx    context.Context
	cancel context.CancelFunc

	umount mountlib.UnmountFn
	lg     *zap.Logger
}

func (s *hugoTestSuite) SetupTest() {
	mp, err := os.MkdirTemp("", "hugomp")
	require.NoError(s.T(), err)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.mp = mp

	s.dbn = s.StorageHarness.GetMetaDB()
	s.dbp = s.StorageHarness.GetMetaDBArg()
	s.lg, _ = zap.NewDevelopment()

	mount := mountlib.LoadMountFn(mountlib.NodeFsName)

	opt := mountOptions(mp, s.dbn, s.dbp, size.Byte*512, s.lg)
	_, unmount, err := mount(s.ctx, mp, opt)
	s.Nil(err)

	s.umount = unmount

	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			s.cancel()

			if err := s.umount(); err != nil {
				log.Error("umount error", zap.Error(err))
			}

			buf, _ := exec.Command("umount", "-l", mp).CombinedOutput()
			log.Warn(string(buf))

			os.Exit(1)
		})

	}
	grace.OnInterrupt(finalise)
}
func (s *hugoTestSuite) TearDownTest() {
	s.cancel()
	if err := s.umount(); err != nil {
		s.lg.Error("unmount failed", zap.Error(err))
		panic(err)
	}
	s.StorageHarness.TearDownSuite()
}

func (s *hugoTestSuite) filename(x string) string {
	return fmt.Sprintf("%s/%s", s.mp, x)
}
func (s *hugoTestSuite) randFilename(x string) string {
	return fmt.Sprintf("%s/%s-%d", s.mp, x, rand.Int())
}

func (s *hugoTestSuite) TestOpenRegFile() {
	err := os.WriteFile(s.filename("hello.txt"), []byte("hello world"), 0644)
	s.NoError(err)

	s.T().Run("test open existed file", func(t *testing.T) {
		f, err := os.Open(s.filename("hello.txt"))
		defer func() {
			err := f.Close()
			require.NoError(s.T(), err)
		}()
		s.NoError(err)
		s.NotNil(f)
	})
	s.T().Run("open nonexistent file", func(t *testing.T) {
		f, err := os.Open(s.randFilename("hello1.txt"))
		s.Error(err)
		s.Nil(f)
	})
}
func (s *hugoTestSuite) TestCreateRegFile() {
	s.T().Run("create existed file", func(t *testing.T) {
		f, err := os.Create(s.filename("hello.txt"))
		defer func() {
			err := f.Close()
			require.NoError(s.T(), err)
		}()
		s.NoError(err)
		s.NotNil(f)

		stat, err := f.Stat()
		s.NoError(err)
		s.False(stat.IsDir())
		s.EqualValues(util.MakeMode(0644, 0).String(), stat.Mode().String())
	})
	s.T().Run("create nonexistent file", func(t *testing.T) {
		f, err := os.Create(s.randFilename("hello.txt"))
		defer func() {
			err := f.Close()
			require.NoError(s.T(), err)
		}()
		s.NoError(err)
		s.NotNil(f)

		stat, err := f.Stat()
		s.NoError(err)
		s.False(stat.IsDir())
		// FIXME: some problem in create file with mode
		s.EqualValues(util.MakeMode(0644, 0).String(), stat.Mode().String())
	})
}
func (s *hugoTestSuite) TestWrite() {
	Write(s.mp, s.T())
}
func (s *hugoTestSuite) TestReadReg() {
	ReadReg(s.mp, s.T())
}
func (s *hugoTestSuite) TestReadDir() {
	ReadDir(s.mp, s.T())
}

func Testhugo(t *testing.T) {
	suite.Run(t, new(hugoTestSuite))
}
