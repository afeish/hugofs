package tests

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strconv"
	"sync"
	"testing"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/util/grace"
	"github.com/afeish/hugo/pkg/util/iotools"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/pingcap/log"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type FusePerfTestSuite struct {
	suite.Suite
	harness *gateway.StorageHarness
	mp      string
	dbn     string
	dbp     string
	ctx     context.Context
	cancel  context.CancelFunc

	maxSize size.SizeSuffix

	umount mountlib.UnmountFn
	lg     *zap.Logger
}

func (s *FusePerfTestSuite) SetupSuite() {
	if global.EnvOrDefault("CI_PIPELINE_ID", "-") != "-" { // ci mode, supress the log
		s.lg, _ = zap.NewProduction()
	} else {
		s.lg, _ = zap.NewDevelopment()
	}
	s.maxSize = size.MebiByte * 200
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.harness = gateway.NewStorageHarness(
		s.ctx,
		gateway.WithTestLog(s.lg),
		gateway.WithVolSize(s.maxSize.Int()),
		gateway.WithInodeTemplate(""),
	)
	mp, err := os.MkdirTemp("", "hugomp")
	s.NoError(err)

	s.mp = mp
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.dbn = s.harness.GetMetaDB()
	s.dbp = s.harness.GetMetaDBArg()

	mount := mountlib.LoadMountFn(mountlib.NodeFsName)

	opt := mountOptions(mp, s.dbn, s.dbp, 32*size.KibiByte, s.lg)
	_, s.umount, err = mount(s.ctx, mp, opt)
	s.Nil(err)

	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			s.cancel()

			if err := s.umount(); err != nil {
				log.Error("umount error", zap.Error(err))
			}

			buf, _ := exec.Command("umount", "-l", mp).CombinedOutput()
			log.Warn(string(buf))
		})

	}
	grace.OnInterrupt(finalise)

}
func (s *FusePerfTestSuite) TearDownSuite() {
	s.cancel()
	if err := s.umount(); err != nil {
		s.lg.Error("unmount failed", zap.Error(err))
		panic(err)
	}

	s.harness.StorageBackend.Destory()
	s.harness.TearDownSuite()
}

func (s *FusePerfTestSuite) BeforeTest(suiteName, testName string) {
	log.Debug("before test", zap.String("suite", suiteName), zap.String("test", testName))
	entries, err := os.ReadDir(s.mp)
	s.Nil(err)
	for _, entry := range entries {
		err = os.RemoveAll(path.Join(s.mp, entry.Name()))
		s.Nil(err)
	}
}
func (s *FusePerfTestSuite) AfterTest(suiteName, testName string) {
	// err := os.RemoveAll(s.mp)
	// s.Nil(err)
	entries, err := os.ReadDir(s.mp)
	s.Nil(err)

	for _, entry := range entries {
		os.RemoveAll(path.Join(s.mp, entry.Name()))
	}

	entries, err = os.ReadDir(s.mp)
	s.Nil(err)
	s.EqualValues(0, len(entries))
}

func (s *FusePerfTestSuite) TestBatchRW() {
	minSz := size.MebiByte
	maxSz := size.MebiByte * 20
	type table struct {
		name       string
		size       int
		copiedSize int
		hashSize   int
		originMd5  string
		copiedMd5  string
		matched    bool
	}

	files := make([]string, 0)
	dstfiles := make([]string, 0)

	mu := sync.Mutex{}
	tables := make([]table, 0)

	batchSize := int(s.maxSize / maxSz)
	g, _ := errgroup.WithContext(s.ctx)
	for i := 0; i < batchSize; i++ {
		g.Go(func() error {
			sz := minSz.Int() + rand.Intn(maxSz.Int()-minSz.Int())
			f, originMd5, err := iotools.RandFile("mock", int64(sz))
			if err != nil {

				return errors.Wrap(err, "create rand file failed")
			}

			fIn, err := os.Open(f)
			if err != nil {
				return errors.Wrap(err, "err open rand file")
			}
			defer fIn.Close()

			dstFile := path.Join(s.mp, path.Base(f))
			mu.Lock()
			files = append(files, f)
			dstfiles = append(dstfiles, dstFile)
			mu.Unlock()

			fOut, err := os.Create(dstFile)
			if err != nil {
				return errors.Wrap(err, "err open dst file to copy")
			}
			defer fOut.Close()

			h := md5.New()
			n, err := io.Copy(fOut, io.TeeReader(fIn, h))
			if err != nil {
				return err
			}

			dstMd5 := hex.EncodeToString(h.Sum(nil))
			fi, _ := fIn.Stat()

			tab := table{
				size:       sz,
				name:       path.Base(f),
				originMd5:  originMd5,
				copiedSize: int(n),
				hashSize:   int(fi.Size()),
				copiedMd5:  dstMd5,
				matched:    originMd5 == dstMd5,
			}
			mu.Lock()
			tables = append(tables, tab)
			mu.Unlock()
			return nil
		})

	}

	if err := g.Wait(); err != nil {
		s.T().Log(err)
		s.Error(err)
	}

	printRows := func(rows []table) [][]string {
		var ret [][]string
		for i, row := range rows {
			r := []string{
				strconv.Itoa(i + 1),
				row.name,
				humanize.IBytes(uint64(row.size)),
				humanize.IBytes(uint64(row.copiedSize)),
				humanize.IBytes(uint64(row.hashSize)),
				row.originMd5,
				row.copiedMd5,
				strconv.FormatBool(row.matched),
			}
			ret = append(ret, r)
		}
		return ret
	}

	printTable := func(list []table) {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"idx", "NAME", "SIZE", "COPIED_SIZE", "HASH-SIZE", "ORIGIN-MD5", "COPIED-MD5", "MATCHED"})
		table.SetRowLine(true)
		table.AppendBulk(printRows(list))
		table.Render()
		os.Stdout.WriteString("\n")
	}

	printTable(tables)
}

func TestPerf(t *testing.T) {
	suite.Run(t, new(FusePerfTestSuite))
}
