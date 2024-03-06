package vfs

import (
	"context"
	"testing"
	"time"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type fsSuite struct {
	suite.Suite

	storage *gateway.StorageHarness
	ctx     context.Context
	cancel  context.CancelFunc

	fs *Fs

	lg *zap.Logger
}

func (s *fsSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.storage = gateway.NewStorageHarness(
		s.ctx,
		gateway.WithVolSize(size.MebiByte.Int()),
	)

	lg, _ := zap.NewDevelopment()
	s.lg = lg.Named("fsSuite")

	opts := &mountlib.Options{
		DirCacheTime: time.Second * 1,
		Logger:       lg,
	}
	s.fs = NewFs(s.storage.MetaServiceClient, s.storage, opts)
}

func (s *fsSuite) TeardownSuite() {

}

func (s *fsSuite) TestList() {
	entries, err := s.fs.List(s.ctx, 1)
	s.Nil(err)
	s.NotEmpty(entries)
}

func TestFs(t *testing.T) {
	suite.Run(t, new(fsSuite))
}
