package cluster

import (
	"encoding/json"
	"fmt"

	"github.com/afeish/hugo/pkg/store"
)

type NodeAddr struct {
	store.UnimplementedSerializer
	Addr string
	Id   uint64
}

func (s *NodeAddr) FormatKey() string {
	return fmt.Sprintf("%s%s", s.FormatPrefix(), s.Addr)
}
func (s *NodeAddr) FormatPrefix() string {
	return "storage_node_addr/"
}
func (s *NodeAddr) Serialize() ([]byte, error) {
	return json.Marshal(s)
}
func (s *NodeAddr) Deserialize(bytes []byte) (*NodeAddr, error) {
	var tmp NodeAddr
	if err := json.Unmarshal(bytes, &tmp); err != nil {
		return nil, err
	}
	return &tmp, nil
}
func (s *NodeAddr) Self() *NodeAddr {
	return s
}
