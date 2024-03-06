package cluster

import (
	"context"
	"fmt"
	"testing"

	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"

	"github.com/pingcap/log"
	"github.com/samber/mo"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ManagerTestSuite struct {
	suite.Suite

	ctx       context.Context
	metaStore store.Store
	manager   *Service
}

func (s *ManagerTestSuite) SetupSuite() {
	s.ctx = context.Background()
	var err error
	s.metaStore, err = store.GetStore(store.MemoryName, mo.None[string]())
	if !s.Nil(err) {
		s.FailNow(err.Error())
	}
	s.manager, err = NewService(s.metaStore)
	if !s.Nil(err) {
		s.FailNow(err.Error())
	}
}

func (s *ManagerTestSuite) BeforeTest(suiteName, testName string) {
	log.Debug("before test", zap.String("suite", suiteName), zap.String("test", testName))
	var err error
	s.metaStore, err = store.GetStore(store.MemoryName, mo.None[string]())
	if !s.Nil(err) {
		s.FailNow(err.Error())
	}
}

func (s *ManagerTestSuite) TestAdd() {
	type T struct {
		Req *pb.AddStorageNode_Request
	}
	testdata := []*T{
		{
			Req: &pb.AddStorageNode_Request{
				Ip:   "127.0.0.1",
				Port: 3,
				Cap:  50 * (1 << 30),
			},
		},
		{
			Req: &pb.AddStorageNode_Request{
				Ip:   "127.0.0.2",
				Port: 4,
				Cap:  20 * (1 << 30),
			},
		},
	}

	for _, e := range testdata {
		_, err := s.manager.AddStorageNode(s.ctx, e.Req)
		if err != nil {
			s.FailNow(err.Error())
		}
	}

	var nodes []*StorageNode
	for _, e := range testdata {
		node, err := s.manager.GetStorageNode(s.ctx, &pb.GetStorageNode_Request{Arg: &pb.GetStorageNode_Request_Address{Address: fmt.Sprintf("%s:%d", e.Req.Ip, e.Req.Port)}})
		s.Nil(err)
		nodes = append(nodes, &StorageNode{
			StorageNode: node,
		})
	}

	s.Equalf(2, len(nodes), "len(nodes) should be 2, but got %d", len(nodes))
	s.Equalf(nodes[0].Peers, nodes[1].Peers, "peers should be equal, but got %v and %v", nodes[0].Peers, nodes[1].Peers)
}

func TestManager(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}
