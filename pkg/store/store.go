package store

import (
	"context"
	"errors"
)

var (
	ErrNilSerializer = errors.New("nil serializer")
	ErrEmptyKey      = errors.New("empty key")
	ErrEmptyPrefix   = errors.New("empty prefix")
	ErrNotFound      = errors.New("not found")
	ErrKeyEqPrefix   = errors.New("formatted key equals to formatted prefix")

	TxnKey TransKey = "__txn_key"
)

type SerializerBuilder[T any] func() (Serializer[T], error)
type TransKey string

func DoInTxn(ctx context.Context, store Store, cb func(context.Context, Transaction) error) (err error) {
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			txn.Rollback(ctx)
		} else {
			txn.Commit(ctx)
		}
	}()
	txn.Begin(ctx)
	ctx = NewTxnContext(ctx, txn)
	err = cb(ctx, txn)
	return
}

func NewTxnContext(ctx context.Context, txn Transaction) context.Context {
	return context.WithValue(ctx, TxnKey, txn)
}

func TxnFromContext(ctx context.Context) Transaction {
	txn, ok := ctx.Value(TxnKey).(Transaction)
	if !ok {
		return nil
	}
	return txn
}

func GetValueByKey[T any](ctx context.Context, store Store, builder SerializerBuilder[T]) (*T, error) {
	ser, err := builder()
	if err != nil {
		return nil, err
	}
	return DoGetValueByKey(ctx, store, ser)
}
func BatchGetValueByKey[T any](ctx context.Context, store Store, builder []SerializerBuilder[T]) ([]*T, error) {
	var s = make([]Serializer[T], len(builder))
	for _, b := range builder {
		ser, err := b()
		if err != nil {
			return nil, err
		}
		s = append(s, ser)
	}

	return BatchDoGetValueByKey(ctx, store, s)
}
func SaveInKVPair[T any](ctx context.Context, store Store, builder SerializerBuilder[T]) error {
	ser, err := builder()
	if err != nil {
		return err
	}
	return DoSaveInKVPair(ctx, store, ser)
}
func GetValuesByPrefix[T any](ctx context.Context, store Store, builder SerializerBuilder[T]) (map[string]*T, error) {
	ser, err := builder()
	if err != nil {
		return nil, err
	}
	return DoGetValuesByPrefix(ctx, store, ser)
}
func Incr[T any](ctx context.Context, store Store, s Serializer[T]) (uint64, error) {
	if s == nil {
		return 0, ErrNilSerializer
	}
	key := s.FormatKey()
	if key == "" {
		return 0, ErrEmptyPrefix
	}

	return store.Incr(ctx, key, 1)
}
func DeleteByKey[T any](ctx context.Context, store Store, builder SerializerBuilder[T]) error {
	ser, err := builder()
	if err != nil {
		return err
	}
	return DoDeleteByKey(ctx, store, ser)
}
func DeleteByPrefix[T any](ctx context.Context, store Store, builder SerializerBuilder[T]) error {
	ser, err := builder()
	if err != nil {
		return err
	}
	return DoDeleteByPrefix(ctx, store, ser)
}
func DoSaveInKVPair[T any](ctx context.Context, store Store, s Serializer[T]) error {
	if s == nil {
		return ErrNilSerializer
	}
	key := s.FormatKey()
	if key == "" {
		return ErrEmptyKey
	}
	if key == s.FormatPrefix() {
		return ErrKeyEqPrefix
	}

	if err := s.BeforeSerialize(); err != nil {
		return err
	}
	value, err := s.Serialize()
	if err != nil {
		return err
	}
	if err := s.AfterSerialize(); err != nil {
		return err
	}
	return store.Save(ctx, key, value)
}
func BatchDoGetValueByKey[T any](ctx context.Context, store Store, s []Serializer[T]) ([]*T, error) {
	var result = make([]*T, len(s))

	for _, _s := range s {
		val, err := DoGetValueByKey(ctx, store, _s)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

func DoGetValueByKey[T any](ctx context.Context, store Store, s Serializer[T]) (*T, error) {
	if s == nil {
		return nil, ErrNilSerializer
	}
	key := s.FormatKey()
	if key == "" {
		return nil, ErrEmptyKey
	}
	if key == s.FormatPrefix() {
		return nil, ErrKeyEqPrefix
	}

	if err := s.BeforeDeserialize(); err != nil {
		return nil, err
	}
	buf, err := store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	value, err := s.Deserialize(buf)
	if err != nil {
		return nil, err
	}
	if err := s.AfterDeserialize(); err != nil {
		return nil, err
	}
	return value, nil
}
func DoGetValuesByPrefix[T any](ctx context.Context, store Store, s Serializer[T]) (map[string]*T, error) {
	if s == nil {
		return nil, ErrNilSerializer
	}
	prefix := s.FormatPrefix()
	if prefix == "" {
		return nil, ErrEmptyPrefix
	}
	bufMap, err := store.GetByBucket(ctx, prefix)
	if err != nil {
		return nil, err
	}
	var result = make(map[string]*T, len(bufMap))
	for key, buf := range bufMap {
		value, err := s.Deserialize(buf)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}
func DoDeleteByKey[T any](ctx context.Context, store Store, s Serializer[T]) error {
	if s == nil {
		return ErrNilSerializer
	}
	key := s.FormatKey()
	if key == "" {
		return ErrEmptyKey
	}
	if key == s.FormatPrefix() {
		return ErrKeyEqPrefix
	}

	return store.Delete(ctx, key)
}
func DoDeleteByPrefix[T any](ctx context.Context, store Store, s Serializer[T]) error {
	if s == nil {
		return ErrNilSerializer
	}
	prefix := s.FormatPrefix()
	if prefix == "" {
		return ErrEmptyPrefix
	}
	if prefix == s.FormatKey() {
		return ErrKeyEqPrefix
	}

	return store.Delete(ctx, prefix)
}
func DoBatchSaveStrictly[T any](ctx context.Context, store Store, s []Serializer[T]) error {
	var entries []*Entry
	for _, e := range s {
		val, err := e.Serialize()
		if err != nil {
			return err
		}
		entries = append(entries, &Entry{
			Key:   e.FormatKey(),
			Value: val,
		})
	}
	return store.BatchSave(ctx, false, entries)
}
func DoBatchSaveAnyStrictly(ctx context.Context, store Store, s []LossySerializer) error {
	var entries []*Entry
	for _, e := range s {
		val, err := e.Serialize()
		if err != nil {
			return err
		}
		entries = append(entries, &Entry{
			Key:   e.FormatKey(),
			Value: val,
		})
	}
	return store.BatchSave(ctx, false, entries)
}
func DoBatchDeleteStrictly[T any](ctx context.Context, store Store, s []Serializer[T]) error {
	var entries []string
	for _, e := range s {
		entries = append(entries, e.FormatKey())
	}
	return store.BatchDeleteByKey(ctx, false, entries)
}

// Store just represents a basic interface for a key-value store.
// Use the generic method ranter than the raw store interface.
type Store interface {
	Close() error
	IsClosed() bool
	Cleanup(context.Context) error
	Save(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Scan(ctx context.Context, bucket, begin, end string) ([]*Entry, error)
	Delete(ctx context.Context, key string) error
	// SaveInBucket save the given k-v pair into designated bucket
	SaveInBucket(ctx context.Context, bucket string, key string, value []byte) error
	// GetInBucket get the given k-v pair in the designated bucket
	GetInBucket(ctx context.Context, bucket string, key string) ([]byte, error)
	// DeleteInBucket delete the given k-v pair in the designated bucket
	DeleteInBucket(ctx context.Context, bucket, key string) error
	// GetByBucket get all the given k-v pairs in the designated bucket
	GetByBucket(ctx context.Context, bucket string) (map[string][]byte, error)
	// DeleteByBucket delete all the given k-v pairs in the designated bucket
	DeleteByBucket(ctx context.Context, bucket string) error
	Incr(ctx context.Context, key string, step uint64) (uint64, error)
	BatchSave(ctx context.Context, ignoreErr bool, entry []*Entry) error
	BatchDeleteByKey(ctx context.Context, ignoreErr bool, key []string) error
	NewTransaction(ctx context.Context) (Transaction, error)
}

type Entry struct {
	Key   string
	Value []byte
}

type Serializer[T any] interface {
	// FormatKey returns the key to store the value in the store.
	// if the key is empty, the generic function will return immediately.
	FormatKey() string
	FormatPrefix() string
	Serialize() ([]byte, error)
	BeforeSerialize() error
	AfterSerialize() error
	Deserialize([]byte) (*T, error)
	BeforeDeserialize() error
	AfterDeserialize() error
	Self() *T
}

type UnimplementedSerializer struct{}

func (u UnimplementedSerializer) FormatKey() string {
	//TODO implement me
	panic("implement me")
}
func (u UnimplementedSerializer) FormatPrefix() string {
	//TODO implement me
	panic("implement me")
}
func (u UnimplementedSerializer) Serialize() ([]byte, error) {
	//TODO implement me
	panic("implement me")
}
func (u UnimplementedSerializer) BeforeSerialize() error {
	return nil
}
func (u UnimplementedSerializer) AfterSerialize() error {
	return nil
}
func (u UnimplementedSerializer) Deserialize(bytes []byte) (*UnimplementedSerializer, error) {
	//TODO implement me
	panic("implement me")
}
func (u UnimplementedSerializer) BeforeDeserialize() error {
	return nil
}
func (u UnimplementedSerializer) AfterDeserialize() error {
	return nil
}
func (u UnimplementedSerializer) Self() *UnimplementedSerializer {
	panic("implement me")
}

var _ Serializer[UnimplementedSerializer] = UnimplementedSerializer{}

type LossySerializer interface {
	Serialize() ([]byte, error)
	FormatKey() string
}
