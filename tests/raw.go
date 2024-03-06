package tests

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"go.uber.org/zap"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	_ "github.com/afeish/hugo/pkg/hugofs/mount"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/util/size"

	"github.com/stretchr/testify/require"
)

func mountOptions(mp, dbn, dba string, blockSize size.SizeSuffix, lg *zap.Logger) *mountlib.Options {
	optFuncs := []Option[*mountlib.Options]{
		mountlib.WithTest(adaptor.TestOption{
			Enabled: true,
			DBName:  dbn,
			DBArg:   dba,
		}),
		mountlib.WithLogger(lg),
		mountlib.WithMountpoint(mp),
		mountlib.WithDBName(dbn),
		mountlib.WithDBArg(fmt.Sprintf("%s/client", dba)),
		mountlib.WithTransport(adaptor.TransportTypeMOCK),
		mountlib.WithBlockSize(blockSize.Int()),
	}
	if EnvOrDefault("CI_PIPELINE_ID", "-") == "-" { // ci mode, supress the log
		optFuncs = append(optFuncs, mountlib.WithDebug())
	}

	opt := mountlib.DefaultOption
	ApplyOptions(&opt, optFuncs...)

	GetEnvCfg().BlockSize = blockSize
	GetEnvCfg().PageSize = blockSize
	return &opt
}

// The reason needs Raw is we cannot debug the test case in IDE when we run mount hugo in Test.

func filename(mp, x string) string {
	return fmt.Sprintf("%s/%s", mp, x)
}
func randFilename(mp, x string) string {
	return fmt.Sprintf("%s/%s-%d", mp, x, rand.Int())
}

func Create(mp string, t *testing.T) {
	t.Run("create file and check", func(t *testing.T) {
		fn := randFilename(mp, "hello.txt")
		f, err := os.Create(fn)
		require.NoError(t, err)
		require.NotNil(t, f)
		err = f.Close()
		require.NoError(t, err)
		// check the file is created

		stat, err := os.Stat(fn)
		require.NoError(t, err)
		require.False(t, stat.IsDir())

		open, err := os.Open(fn)
		require.NoError(t, err)
		require.NotNil(t, open)
	})
}
func Write(mp string, t *testing.T) {
	t.Run("write without fsync", func(t *testing.T) {
		t.Run("create and write", func(t *testing.T) {
			content := []byte("hello")
			name := randFilename(mp, "hello.txt")
			f, err := os.Create(name)
			defer func() {
				err := f.Close()
				require.NoError(t, err)
			}()
			require.NoError(t, err)
			require.NotNil(t, f)

			n, err := f.Write(content)
			require.NoError(t, err)
			require.Equal(t, len(content), n)

			stat, err := f.Stat()
			require.NoError(t, err)
			require.Equal(t, 5, int(stat.Size()))

			buf := make([]byte, 5)
			n, err = f.ReadAt(buf, 0)
			require.NoError(t, err)
			require.Equal(t, 5, n)
			require.True(t, reflect.DeepEqual(buf, content))
		})
		t.Run("write to existed file by opening file", func(t *testing.T) {
			content := []byte("hello")
			fn := filename(mp, "hello.txt")
			var (
				f   *os.File
				err error
			)
			defer func() {
				err := f.Close()
				require.NoError(t, err)
			}()
			t.Run("1 open", func(t *testing.T) {
				f, err = os.OpenFile(fn, os.O_RDWR, 0666)
				require.NoError(t, err)
				require.NotNil(t, f)
			})
			t.Run("2 write", func(t *testing.T) {
				n, err := f.Write(content)
				require.NoError(t, err)
				require.Equal(t, len(content), n)
			})
			t.Run("3 read", func(t *testing.T) {
				buf := make([]byte, 5)
				n, err := f.ReadAt(buf, 0)
				require.NoError(t, err)
				require.Equal(t, 5, n)
				require.True(t, reflect.DeepEqual(buf, content))
			})
		})
	})
	t.Run("write with fsync", func(t *testing.T) {
		content := []byte("hello")
		name := randFilename(mp, "hello.txt")
		f, err := os.Create(name)
		require.NoError(t, err)
		require.NotNil(t, f)

		n, err := f.Write(content)
		require.NoError(t, err)
		require.Equal(t, len(content), n)
		require.NoError(t, f.Close())

		stat, err := os.Stat(name)
		require.NoError(t, err)
		require.Equal(t, 5, int(stat.Size()))

		buf, err := os.ReadFile(name)
		require.NoError(t, err)
		require.True(t, reflect.DeepEqual(buf, content))
	})
}
func ReadReg(mp string, t *testing.T) {
	t.Run("read from existed file", func(t *testing.T) {
		fn := filename(mp, "hello.txt")
		f, err := os.Open(fn)
		require.NoError(t, err)
		require.NotNil(t, f)

		buf := make([]byte, 5)
		_, err = f.Read(buf)
		require.Equal(t, io.EOF, err)
	})
}
func ReadDir(mp string, t *testing.T) {
	t.Run("read dir", func(t *testing.T) {
		f, err := os.Open(mp)
		require.NoError(t, err)
		require.NotNil(t, f)

		names, err := f.Readdirnames(0)
		require.NoError(t, err)
		t.Log(names)
		require.True(t, len(names) != 0)
	})
}
