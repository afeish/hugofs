package tests

import (
	"context"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"
	"testing"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/util/grace"
	"github.com/afeish/hugo/pkg/util/size"

	"github.com/hanwen/go-fuse/v2/posixtest"
	"github.com/pingcap/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type FuseTestSuite struct {
	suite.Suite
	harness *gateway.StorageHarness
	mp      string
	dbn     string
	dbp     string
	ctx     context.Context
	cancel  context.CancelFunc

	umount mountlib.UnmountFn
	lg     *zap.Logger
}

func (s *FuseTestSuite) SetupSuite() {
	if global.EnvOrDefault("CI_PIPELINE_ID", "-") != "-" { // ci mode, supress the log
		s.lg, _ = zap.NewProduction()
	} else {
		s.lg, _ = zap.NewDevelopment()
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.harness = gateway.NewStorageHarness(
		s.ctx,
		gateway.WithTestLog(s.lg),
		gateway.WithVolSize(size.MebiByte.Int()),
		gateway.WithInodeTemplate(""),
	)
	mp, err := os.MkdirTemp("", "hugomp")
	s.NoError(err)

	s.mp = mp

	s.dbn = s.harness.GetMetaDB()
	s.dbp = s.harness.GetMetaDBArg()

	opt := mountOptions(mp, s.dbn, s.dbp, size.Byte*512, s.lg)

	mount := mountlib.LoadMountFn(mountlib.NodeFsName)
	_, unmount, err := mount(s.ctx, mp, opt)
	s.Nil(err)

	s.umount = unmount

	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			s.cancel()

			if err := s.umount(); err != nil {
				log.Error("umount error", zap.Error(err))

				buf, _ := exec.Command("umount", "-l", mp).CombinedOutput()
				log.Warn(string(buf))

				os.Exit(1)
			}
		})

	}
	grace.OnInterrupt(finalise)
}
func (s *FuseTestSuite) TearDownSuite() {
	s.lg.Debug("teardown suite")
	s.cancel()
	if err := s.umount(); err != nil {
		s.lg.Error("unmount failed", zap.Error(err))
		panic(err)
	}

	s.harness.StorageBackend.Destory()
	s.harness.TearDownSuite()
}

func (s *FuseTestSuite) BeforeTest(suiteName, testName string) {
	s.lg.Debug("before test", zap.String("suite", suiteName), zap.String("test", testName))
	entries, err := os.ReadDir(s.mp)

	s.Nil(err)
	for _, entry := range entries {
		os.RemoveAll(path.Join(s.mp, entry.Name()))
	}
}
func (s *FuseTestSuite) AfterTest(suiteName, testName string) {
	entries, err := os.ReadDir(s.mp)
	s.Nil(err)

	for _, entry := range entries {
		s.lg.Debug("entry", zap.String("path", path.Join(s.mp, entry.Name())))
		os.RemoveAll(path.Join(s.mp, entry.Name()))
	}

	entries, err = os.ReadDir(s.mp)
	s.Nil(err)

	for _, entry := range entries {
		s.lg.Debug("entry", zap.String("path", path.Join(s.mp, entry.Name())))
	}
	s.EqualValues(0, len(entries))
}

func (s *FuseTestSuite) TestAppendWrite() {
	posixtest.AppendWrite(s.T(), s.mp)
}
func (s *FuseTestSuite) TestSymlinkReadlink() {
	posixtest.SymlinkReadlink(s.T(), s.mp)
}
func (s *FuseTestSuite) TestFileBasic() {
	posixtest.FileBasic(s.T(), s.mp)
}
func (s *FuseTestSuite) TestTruncateFile() {
	posixtest.TruncateFile(s.T(), s.mp)
}
func (s *FuseTestSuite) TestTruncateNoFile() {
	posixtest.TruncateNoFile(s.T(), s.mp)
}
func (s *FuseTestSuite) TestFdLeak() {
	for i := 0; i < 100; i++ {
		posixtest.FdLeak(s.T(), s.mp)
	}
}
func (s *FuseTestSuite) TestMkdirRmdir() {
	posixtest.MkdirRmdir(s.T(), s.mp)
}
func (s *FuseTestSuite) TestNlinkZero() {
	posixtest.NlinkZero(s.T(), s.mp)
}
func (s *FuseTestSuite) TestFstatDeleted() {
	posixtest.FstatDeleted(s.T(), s.mp)
}
func (s *FuseTestSuite) TestParallelFileOpen() {
	posixtest.ParallelFileOpen(s.T(), s.mp)
}
func (s *FuseTestSuite) TestLink() {
	posixtest.Link(s.T(), s.mp)
}
func (s *FuseTestSuite) TestLinkUnlinkRename() {
	posixtest.LinkUnlinkRename(s.T(), s.mp)
}
func (s *FuseTestSuite) TestRenameOverwriteDestNoExist() {
	posixtest.RenameOverwriteDestNoExist(s.T(), s.mp)
}
func (s *FuseTestSuite) TestRenameOverwriteDestExist() {
	posixtest.RenameOverwriteDestExist(s.T(), s.mp)
}
func (s *FuseTestSuite) TestRenameOpenDir() {
	posixtest.RenameOpenDir(s.T(), s.mp)
}
func (s *FuseTestSuite) TestReadDir() {
	posixtest.ReadDir(s.T(), s.mp)
}
func (s *FuseTestSuite) TestReadDirPicksUpCreate() {
	posixtest.ReadDirPicksUpCreate(s.T(), s.mp)
}
func (s *FuseTestSuite) TestDirectIO() {
	posixtest.DirectIO(s.T(), s.mp)
}
func (s *FuseTestSuite) TestOpenAt() {
	posixtest.OpenAt(s.T(), s.mp)
}
func (s *FuseTestSuite) TestFallocate() {
	// posixtest.Fallocate(s.T(), s.mp)
}
func (s *FuseTestSuite) TestDirSeek() {
	posixtest.DirSeek(s.T(), s.mp)
}
func (s *FuseTestSuite) TestStatFS() {
	var st syscall.Statfs_t
	err := syscall.Statfs(s.mp, &st)
	s.Nil(err)
	s.EqualValuesf(4096, st.Bsize, "bsize should be 4096 but got %d", st.Bsize)
	s.EqualValuesf(0, st.Blocks-st.Bavail, "used blocks should be 0 but got %d", st.Blocks-st.Bavail)
	s.NotEqualf(0, st.Files-st.Ffree, "used files should be 0 but got %d", st.Files)
}
func (s *FuseTestSuite) TestGetLk() {
	// Create a temporary file for testing
	file, err := os.CreateTemp(s.mp, "testfile")
	require.NoError(s.T(), err)
	defer func() {
		file.Close()
		os.Remove(file.Name())
	}()

	// Lock the file using fcntl
	lock := syscall.Flock_t{Type: syscall.F_WRLCK}
	err = syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	require.NoError(s.T(), err)

	// Unlock the file using fcntl
	lock.Type = syscall.F_UNLCK
	err = syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	require.NoError(s.T(), err)
}
func (s *FuseTestSuite) TestSetLK() {
	// Create a temporary file for testing
	file, err := os.CreateTemp(s.mp, "testfile")
	require.NoError(s.T(), err)
	defer func() {
		file.Close()
		os.Remove(file.Name())
	}()

	// Lock the file using fcntl
	lock := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: 0}
	err = syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	require.NoError(s.T(), err)

	// Unlock the file using fcntl
	lock.Type = syscall.F_UNLCK
	err = syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	require.NoError(s.T(), err)

	// Lock the file again using fcntl
	lock.Type = syscall.F_WRLCK
	err = syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	require.NoError(s.T(), err)

	// Unlock the file using fcntl
	lock.Type = syscall.F_UNLCK
	err = syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	require.NoError(s.T(), err)
}

func TestFuse(t *testing.T) {
	suite.Run(t, new(FuseTestSuite))
}
