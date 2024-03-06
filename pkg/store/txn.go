package store

import (
	"context"
	"errors"
	"sync"

	"github.com/samber/mo"
)

type (
	TxnCreator interface {
		Create(arg mo.Option[any]) (Transaction, error)
		Name() string
	}
	fnTxnCreator func(arg mo.Option[any]) (Transaction, error)
)

var (
	ErrTxnHasBeenCreated = errors.New("txn has been created")
	_txnFactory          = struct{ list map[string]fnTxnCreator }{list: make(map[string]fnTxnCreator)}
)

func RegisterTxnCreator[T TxnCreator](x T) error {
	_, ok := _txnFactory.list[x.Name()]
	if ok {
		return ErrTxnHasBeenCreated
	}
	_txnFactory.list[x.Name()] = x.Create
	return nil
}

func GetTxn(name string, arg mo.Option[any]) (Transaction, error) {
	creator, ok := _txnFactory.list[name]
	if !ok {
		return nil, errors.New("unknown transaction type " + name)
	}
	return creator(arg)
}

type Transaction interface {
	sync.Locker
	Begin(ctx context.Context) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	ToNative() any
}
