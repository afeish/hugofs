package store

import (
	"context"

	"github.com/pingcap/log"
	"github.com/samber/mo"
)

const MemoryName = "memory"

var (
	_ Store = (*MemoryStore)(nil)
)

func init() {
	if err := RegisterCreator(&MemoryStore{}); err != nil {
		log.Fatal("failed to register memory store")
	}
}

type MemoryStore struct {
	*BadgerStore
}

func newMemoryStore() Store {
	s, err := newMemBadgerDB()
	if err != nil {
		panic(err)
	}
	return &MemoryStore{
		BadgerStore: s,
	}
}

func (m *MemoryStore) Name() string {
	return MemoryName
}
func (m *MemoryStore) Creator(arg mo.Option[string]) (Store, error) {
	return newMemoryStore(), nil
}
func (m *MemoryStore) NewTransaction(ctx context.Context) (Transaction, error) {
	return GetTxn(MemoryName, mo.Some[any](m))
}
