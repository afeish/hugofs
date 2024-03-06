package store

import (
	"context"
	"sync"

	"github.com/samber/mo"
	"github.com/tikv/client-go/v2/tikv"
	"github.com/tikv/client-go/v2/txnkv"
)

var _ Transaction = (*TikvTxn)(nil)

type TikvTxn struct {
	mu     sync.RWMutex
	Client *txnkv.Client
	txn    *tikv.KVTxn
}

func init() {
	RegisterTxnCreator(&TikvTxn{})
}

func (t *TikvTxn) Create(arg mo.Option[any]) (Transaction, error) {
	cli := arg.MustGet()
	client, ok := cli.(*txnkv.Client)
	if !ok {
		panic("invalid tikv client type")
	}
	return newTikvTxn(client), nil
}

func (t *TikvTxn) Name() string {
	return TikvName
}

func newTikvTxn(client *txnkv.Client) *TikvTxn {
	bt := &TikvTxn{Client: client}
	return bt
}

func (t *TikvTxn) Begin(ctx context.Context) (err error) {
	t.txn, err = t.Client.Begin()
	return err
}
func (t *TikvTxn) Commit(ctx context.Context) error {
	return t.txn.Commit(ctx)
}
func (t *TikvTxn) Rollback(ctx context.Context) error {
	return t.txn.Rollback()
}
func (t *TikvTxn) ToNative() any {
	return t.txn
}

func (t *TikvTxn) Lock() {
	t.mu.Lock()
}

func (t *TikvTxn) Unlock() {
	t.mu.Unlock()
}
