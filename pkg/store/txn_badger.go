package store

import (
	"context"
	"sync"

	"github.com/dgraph-io/badger/v3"
	"github.com/samber/mo"
)

var _ Transaction = (*BadgerTxn)(nil)

type BadgerTxn struct {
	mu  sync.RWMutex
	DB  *badger.DB
	txn *badger.Txn
}

func init() {
	RegisterTxnCreator(&BadgerTxn{})
}

func newBadgerTxn(db *badger.DB) *BadgerTxn {
	bt := &BadgerTxn{DB: db}
	return bt
}

func (t *BadgerTxn) Create(arg mo.Option[any]) (Transaction, error) {
	db := arg.MustGet()
	badgerDB, ok := db.(*badger.DB)
	if !ok {
		panic("invalid badger db type")
	}
	return newBadgerTxn(badgerDB), nil
}

func (t *BadgerTxn) Name() string {
	return BadgerName
}

func (t *BadgerTxn) Begin(ctx context.Context) error {
	t.txn = t.DB.NewTransaction(true)
	return nil
}

func (t *BadgerTxn) Commit(ctx context.Context) error {
	return t.txn.Commit()
}

func (t *BadgerTxn) Rollback(ctx context.Context) error {
	t.txn.Discard()
	return nil
}

func (t *BadgerTxn) ToNative() any {
	return t.txn
}

func (t *BadgerTxn) Lock() {
	t.mu.Lock()
}

func (t *BadgerTxn) Unlock() {
	t.mu.Unlock()
}
