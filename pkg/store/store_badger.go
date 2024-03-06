package store

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/dgraph-io/badger/v3"
	"github.com/pingcap/log"
	"github.com/samber/mo"
	"go.uber.org/zap"
)

const (
	Seperator  = string(os.PathSeparator)
	BadgerName = "badger"
)

var (
	_ Store = (*BadgerStore)(nil)
)

func init() {
	if err := RegisterCreator(&BadgerStore{}); err != nil {
		log.Fatal("failed to register memory store")
	}
}

type BadgerStore struct {
	DB *badger.DB
}

func newMemBadgerDB() (*BadgerStore, error) {
	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true).WithMemTableSize(16 << 20))
	if err != nil {
		return nil, err
	}
	return &BadgerStore{DB: db}, nil
}
func newBadgerDB(path string) (*BadgerStore, error) {
	database, err := badger.Open(badger.DefaultOptions(path))
	if err != nil {
		log.Info("failed to open badger, retry...", zap.Error(err))
		if err = os.RemoveAll(path); err != nil {
			return nil, err
		}

		database, err = badger.Open(badger.DefaultOptions(path))
		if err != nil {
			log.Error("second try still failed, give up...", zap.Error(err))
			return nil, err
		}

		return &BadgerStore{DB: database}, nil
	}
	return &BadgerStore{
		DB: database,
	}, nil
}
func (db *BadgerStore) Name() string {
	return BadgerName
}
func (db *BadgerStore) Creator(arg mo.Option[string]) (Store, error) {
	p := arg.MustGet()
	if strings.HasPrefix(p, "localhost:") {
		temp, err := os.MkdirTemp("", "badger")
		if err != nil {
			return nil, err
		}
		p = temp
	}
	return newBadgerDB(p)
}
func (db *BadgerStore) Close() error {
	return db.DB.Close()
}
func (db *BadgerStore) IsClosed() bool {
	return db.DB.IsClosed()
}
func (db *BadgerStore) Get(ctx context.Context, key string) ([]byte, error) {
	if key != Seperator {
		key = strings.TrimSuffix(key, Seperator)
	}
	var valCopy []byte
	cb := func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		valCopy, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return nil
	}
	err := db.txnViewWrapper(ctx, cb)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNotFound
	}
	return valCopy, err
}
func (db *BadgerStore) Scan(ctx context.Context, prefix, begin, end string) ([]*Entry, error) {
	var ret []*Entry
	cb := func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		it.Rewind()
		for it.Seek([]byte(begin)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()
			itemKey := string(item.Key())
			if end != "" && itemKey > end {
				break
			}
			valueCopy, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			ret = append(ret, &Entry{
				Key:   itemKey,
				Value: valueCopy,
			})
		}
		return nil
	}
	err := db.txnViewWrapper(ctx, cb)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNotFound
	}
	return ret, err
}
func (db *BadgerStore) GetByBucket(ctx context.Context, key string) (map[string][]byte, error) {
	return db.doGetValuesByPrefix(ctx, key, true)
}
func (db *BadgerStore) doGetValuesByPrefix(ctx context.Context, key string, ignoreExact bool) (map[string][]byte, error) {
	if key != Seperator {
		key = strings.TrimSuffix(key, Seperator)
	}
	ret := make(map[string][]byte)
	cb := func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(key)
		if key != Seperator {
			prefix = []byte(key + Seperator)
		}
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			itemKey := string(item.Key())
			if ignoreExact && itemKey == key {
				continue
			}
			if key == Seperator && strings.Count(itemKey, Seperator) > 1 {
				continue
			}
			valueCopy, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			ret[itemKey] = valueCopy
		}
		return nil
	}
	err := db.txnViewWrapper(ctx, cb)

	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNotFound
	}
	return ret, err
}
func (db *BadgerStore) Delete(ctx context.Context, key string) error {
	cb := func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	}
	err := db.txnUpdateWrapper(ctx, cb)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return ErrNotFound
	}
	return err
}

func (db *BadgerStore) SaveInBucket(ctx context.Context, bucket string, key string, value []byte) error {
	key = bucket + Seperator + key
	return db.Save(ctx, key, value)
}

func (db *BadgerStore) GetInBucket(ctx context.Context, bucket string, key string) ([]byte, error) {
	key = bucket + Seperator + key
	return db.Get(ctx, key)
}

func (db *BadgerStore) DeleteInBucket(ctx context.Context, bucket string, key string) error {
	return db.Delete(ctx, bucket+Seperator+key)
}
func (db *BadgerStore) DeleteByBucket(ctx context.Context, prefix string) error {
	if !strings.HasSuffix(prefix, Seperator) {
		prefix = prefix + Seperator
	}
	cb := func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			if err := txn.Delete(it.Item().Key()); err != nil {
				return err
			}
		}
		return nil
	}

	return db.txnUpdateWrapper(ctx, cb)
}
func (db *BadgerStore) Incr(ctx context.Context, key string, count uint64) (uint64, error) {
	seq, err := db.DB.GetSequence([]byte(key), count)
	if err != nil {
		return 0, err
	}
	defer seq.Release()
	num, err := seq.Next()
	num++ // by default, badger return a num starting from 0, we adjust it to 1
	if err != nil {
		return 0, err
	}
	return uint64(num), nil
}
func (db *BadgerStore) Cleanup(ctx context.Context) error {
	cb := func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			if err := txn.Delete(it.Item().Key()); err != nil {
				return err
			}
		}
		return nil
	}
	return db.txnUpdateWrapper(context.Background(), cb)
}
func (db *BadgerStore) Save(ctx context.Context, key string, value []byte) error {
	cb := func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), value)
		return txn.SetEntry(e)
	}
	return db.txnUpdateWrapper(ctx, cb)
}
func (db *BadgerStore) txnUpdateWrapper(ctx context.Context, cb func(txn *badger.Txn) error) error {
	_txn := TxnFromContext(ctx)
	if _txn == nil {
		return db.DB.Update(cb)
	}
	txn := _txn.ToNative().(*badger.Txn)
	return cb(txn)
}
func (db *BadgerStore) txnViewWrapper(ctx context.Context, cb func(txn *badger.Txn) error) error {
	_txn := TxnFromContext(ctx)
	if _txn == nil {
		return db.DB.View(cb)
	}
	txn := _txn.ToNative().(*badger.Txn)
	return cb(txn)
}
func (db *BadgerStore) BatchSave(ctx context.Context, ignoreErr bool, entries []*Entry) error {
	cb := func(txn *badger.Txn) error {
		for _, entry := range entries {
			err := txn.Set([]byte(entry.Key), entry.Value)
			if ignoreErr && err != nil {
				continue
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	return db.txnUpdateWrapper(ctx, cb)
}
func (db *BadgerStore) BatchDeleteByKey(ctx context.Context, ignoreErr bool, keys []string) error {
	cb := func(txn *badger.Txn) error {
		for _, key := range keys {
			err := txn.Delete([]byte(key))
			if ignoreErr && err != nil {
				continue
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	err := db.txnUpdateWrapper(ctx, cb)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return ErrNotFound
	}
	return nil
}
func (db *BadgerStore) NewTransaction(ctx context.Context) (Transaction, error) {
	d := any(db.DB)
	return GetTxn(db.Name(), mo.Some(d))
}
