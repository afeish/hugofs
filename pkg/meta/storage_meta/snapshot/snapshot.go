package snapshot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/afeish/hugo/pkg/meta/storage_meta/block_map"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/store"
)

type Manager struct {
	metaStore store.Store
}

func (m *Manager) GetSnapshot(ctx context.Context, id uint64) (*Snapshot, error) {
	panic("implement me")
}

type Snapshot struct {
	store.UnimplementedSerializer

	ID          uint64
	DirKind     bool
	ParentIno   Ino
	InoBlockMap *block_map.InoBlockMap
	Children    map[Ino]*Snapshot
	CreatedAt   uint64
}

func (s *Snapshot) FormatKey() string {
	return fmt.Sprintf("%s%d", s.FormatPrefix(), s.ID)
}
func (s *Snapshot) FormatPrefix() string {
	return fmt.Sprintf("snapshot/%d/", s.ID)
}
func (s *Snapshot) Serialize() ([]byte, error) {
	return json.Marshal(s)
}
func (s *Snapshot) Deserialize(bytes []byte) (*Snapshot, error) {
	var out Snapshot
	if err := json.Unmarshal(bytes, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (s *Snapshot) Self() *Snapshot {
	return s
}
