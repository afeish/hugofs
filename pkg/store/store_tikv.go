package store

import (
	"context"
	"strconv"
	"strings"

	"github.com/afeish/hugo/pkg/util"

	"github.com/pingcap/log"
	"github.com/samber/mo"
	tikverr "github.com/tikv/client-go/v2/error"
	"github.com/tikv/client-go/v2/tikv"
	"github.com/tikv/client-go/v2/txnkv"
)

const TikvName = "tikv"

var _ Store = (*TikvStore)(nil)

func init() {
	if err := RegisterCreator(&TikvStore{}); err != nil {
		log.Fatal("failed to register tikv store")
	}
}

type TikvStore struct {
	txClient *txnkv.Client
}

func (t *TikvStore) Name() string {
	return TikvName
}

func (t *TikvStore) Creator(args mo.Option[string]) (Store, error) {
	return newTikv(args)
}

func newTikv(arg mo.Option[string]) (*TikvStore, error) {
	var (
		txClient *txnkv.Client
	)

	if err := util.Retry("newTikv", func() (err error) {
		txClient, err = txnkv.NewClient([]string{arg.MustGet()})
		return err
	}); err != nil {
		if err != nil {
			panic("tikv new client panic")
		}
	}
	return &TikvStore{
		txClient: txClient,
	}, nil
}

func (t *TikvStore) Close() error {
	return t.txClient.Close()
}

func (t *TikvStore) IsClosed() bool {
	select {
	case <-t.txClient.Closed():
		return true
	default:
	}
	return false
}

func DoInTikvTranaction[T any](t *TikvStore, ctx context.Context, cb func(tx *tikv.KVTxn) (T, error)) (*T, error) {
	_txn := TxnFromContext(ctx)
	if _txn != nil {
		txn, ok := _txn.ToNative().(*tikv.KVTxn)
		if !ok {
			panic("tikv txn invalid")
		}
		r, err := cb(txn)
		return &r, err
	}

	tx, err := t.txClient.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit(ctx)
		}
	}()
	r, err := cb(tx)
	return &r, err
}

func (t *TikvStore) Save(ctx context.Context, key string, value []byte) (err error) {
	_, err = DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) (*int, error) {
		return nil, tx.Set([]byte(key), value)
	})
	return err
}

func (t *TikvStore) Get(ctx context.Context, key string) ([]byte, error) {
	cb := func(tx *tikv.KVTxn) ([]byte, error) {
		bs, err := tx.Get(ctx, []byte(key))
		if err != nil {
			if tikverr.IsErrNotFound(err) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		return bs, nil
	}
	b, err := DoInTikvTranaction(t, ctx, cb)
	if b == nil {
		return nil, err
	}
	return *b, err
}

func (t *TikvStore) Scan(ctx context.Context, prefix, begin, end string) ([]*Entry, error) {
	b, err := DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) ([]*Entry, error) {
		it, err := tx.Iter([]byte(begin), []byte(end))
		if err != nil {
			return nil, err
		}
		defer it.Close()
		var res []*Entry
		for it.Valid() {
			candidateKey := string(it.Key())
			if strings.HasPrefix(candidateKey, prefix) && candidateKey <= end {
				res = append(res, &Entry{
					Key:   candidateKey,
					Value: it.Value(),
				})
			}
			it.Next()
		}
		return res, nil
	})
	if b == nil {
		return nil, err
	}
	return *b, err
}

func (t *TikvStore) Incr(ctx context.Context, key string, step uint64) (uint64, error) {
	b, err := DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) (uint64, error) {
		keyBuf := []byte(key)
		valBuf, err := tx.Get(ctx, keyBuf)

		var counter uint64
		if tikverr.IsErrNotFound(err) {
			err = nil
			counter = step + 1
			tx.Set(keyBuf, []byte(strconv.FormatUint(counter, 10)))
			return counter, nil
		}

		oldV, err := strconv.ParseUint(string(valBuf), 10, 64)
		if err != nil {
			return 0, err
		}
		counter = oldV + step
		err = tx.Set(keyBuf, []byte(strconv.FormatUint(counter, 10)))
		return counter, err
	})
	if b == nil {
		return 0, err
	}
	return *b, err
}

func (t *TikvStore) Delete(ctx context.Context, key string) error {
	_, err := DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) (*int, error) {
		return nil, tx.Delete([]byte(key))
	})
	return err
}

func (t *TikvStore) SaveInBucket(ctx context.Context, bucket string, key string, value []byte) error {
	key = bucket + Seperator + key
	return t.Save(ctx, key, value)
}

func (t *TikvStore) GetInBucket(ctx context.Context, bucket string, key string) ([]byte, error) {
	key = bucket + Seperator + key
	return t.Get(ctx, key)
}

func (t *TikvStore) DeleteInBucket(ctx context.Context, bucket string, key string) error {
	key = bucket + Seperator + key
	return t.Delete(ctx, key)
}

func (t *TikvStore) GetByBucket(ctx context.Context, prefix string) (res map[string][]byte, err error) {
	b, err := DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) (map[string][]byte, error) {
		prefixBuf := []byte(prefix)
		it, err := tx.Iter(prefixBuf, nil)
		if err != nil {
			return nil, err
		}
		defer it.Close()
		res := make(map[string][]byte)
		for it.Valid() {
			candidateKey := string(it.Key())
			if strings.HasPrefix(candidateKey, prefix) { //TODO: fixme using iterator upperBound
				res[candidateKey] = it.Value()
			}
			it.Next()
		}
		return res, nil
	})
	if b == nil {
		return nil, err
	}
	return *b, err
}

func (t *TikvStore) DeleteByBucket(ctx context.Context, bucket string) error {
	_, err := DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) (*int, error) {
		it, err := tx.Iter([]byte(bucket), nil)
		if err != nil {
			return nil, err
		}
		defer it.Close()
		for it.Valid() {
			candidateKey := string(it.Key())
			if strings.HasPrefix(candidateKey, bucket) { //TODO: fixme using iterator upperBound
				if err := tx.Delete(it.Key()); err != nil {
					return nil, err
				}
			}
			it.Next()
		}
		return nil, nil
	})
	return err
}

func (t *TikvStore) Cleanup(ctx context.Context) error {
	_, err := DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) (*int, error) {
		panic("not implemented")
	})
	return err
}

func (t *TikvStore) BatchSave(ctx context.Context, ignoreErr bool, entry []*Entry) error {
	_, err := DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) (*int, error) {
		for _, e := range entry {
			err := tx.Set([]byte(e.Key), e.Value)
			if ignoreErr && err != nil {
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (t *TikvStore) BatchDeleteByKey(ctx context.Context, ignoreErr bool, key []string) error {
	_, err := DoInTikvTranaction(t, ctx, func(tx *tikv.KVTxn) (*int, error) {
		for _, k := range key {
			err := tx.Delete([]byte(k))
			if ignoreErr && err != nil {
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (t *TikvStore) NewTransaction(ctx context.Context) (Transaction, error) {
	return GetTxn(TikvName, mo.Some(any(t.txClient)))
}
