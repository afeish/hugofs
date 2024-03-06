package inode

import (
	"context"
	"io/fs"
	"os"
	"testing"
	"time"

	. "github.com/afeish/hugo/global"
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type InodeTestSuite struct {
	suite.Suite

	ctx context.Context
	dbp string
	db  store.Store
	svc *Service
}

func (s *InodeTestSuite) SetupSuite() {
	dbp, err := os.MkdirTemp("", "badger")
	require.NoError(s.T(), err)
	ss, err := store.GetStore(store.BadgerName, mo.Some(dbp))
	require.NoError(s.T(), err)
	s.db = ss
	svc, err := NewService(s.ctx, ss)
	require.NoError(s.T(), err)
	s.svc = svc
}
func (s *InodeTestSuite) TearDownSuite() {
	os.RemoveAll(s.dbp)
}

func (s *InodeTestSuite) TestUpdateAttr() {
	s.T().Run("create and update", func(t *testing.T) {
		createInodeResp, err := s.svc.CreateInode(s.ctx, &pb.CreateInode_Request{
			ParentIno: 1,
			ItemName:  "1",
			Attr: &pb.ModifyAttr{
				Mode: Ptr(uint32(util.MakeMode(0644, fs.ModeDir))),
			},
		})
		require.NoError(s.T(), err)

		require.Equal(s.T(), uint32(util.MakeMode(0644, fs.ModeDir)), createInodeResp.Attr.Mode)

		updateInodeAttrResp, err := s.svc.UpdateInodeAttr(s.ctx, &pb.UpdateInodeAttr_Request{
			Ino: createInodeResp.Attr.Inode,
			Set: Ptr(uint32(SetAttrSize | SetAttrAtime)),
			Attr: &pb.ModifyAttr{
				Size:    Ptr(uint64(100)),
				Atime:   Ptr(time.Now().Unix()),
				Atimens: Ptr(uint32(time.Now().Nanosecond())),
			},
		})
		require.NoError(s.T(), err)
		require.Equal(s.T(), 100, int(updateInodeAttrResp.Attr.Size))

		getInodeAttrResp, err := s.svc.GetInodeAttr(s.ctx, &pb.GetInodeAttr_Request{
			Ino: createInodeResp.Attr.Inode,
		})
		require.NoError(t, err)
		require.Equal(t, 100, int(getInodeAttrResp.Attr.Size))
	})
}

func TestInodeService(t *testing.T) {
	v := new(InodeTestSuite)
	v.ctx = context.Background()
	suite.Run(t, v)
}
