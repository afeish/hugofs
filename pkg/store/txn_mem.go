package store

import (
	"sync"

	"github.com/samber/mo"
)

var _ Transaction = (*MemTxn)(nil)

type MemTxn struct {
	mu sync.RWMutex
	*BadgerTxn
}

func init() {
	RegisterTxnCreator(&MemTxn{})
}

func (m *MemTxn) Create(arg mo.Option[any]) (Transaction, error) {
	return &MemTxn{
		BadgerTxn: newBadgerTxn(arg.MustGet().(*MemoryStore).DB),
	}, nil
}

func (m *MemTxn) Name() string {
	return MemoryName
}

func (m *MemTxn) Lock() {
	m.mu.Lock()
}

func (m *MemTxn) Unlock() {
	m.mu.Unlock()
}
