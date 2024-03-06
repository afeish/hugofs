package cluster

import (
	"fmt"

	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"

	"google.golang.org/protobuf/proto"
)

type StorageNode struct {
	store.UnimplementedSerializer
	*pb.StorageNode
}

func (s *StorageNode) FormatKey() string {
	return fmt.Sprintf("%s%d", s.FormatPrefix(), s.Id)
}
func (s *StorageNode) FormatPrefix() string {
	return "storage_node/"
}
func (s *StorageNode) Serialize() ([]byte, error) {
	return proto.Marshal(s)
}
func (s *StorageNode) Deserialize(bytes []byte) (*StorageNode, error) {
	var out = StorageNode{StorageNode: &pb.StorageNode{}}

	if err := proto.Unmarshal(bytes, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (s *StorageNode) Self() *StorageNode {
	return s
}

func (s *StorageNode) Addr() string {
	return fmt.Sprintf("%s:%d", s.Ip, s.Port)
}

type StorageNodeConfig struct {
	IP   string
	Port int
	Cap  uint64
}
