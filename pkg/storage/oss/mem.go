package oss

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
)

var (
	publicMem sync.Map
)

var _ CloudKV = (*MemStorage)(nil)

type MemStorage struct {
	m          *sync.Map
	bucketName string
	view       map[string][]byte
}

func init() {
	GetFactory().Put(PlatformInfoMEMORY, &MemStorage{})
}

func (m *MemStorage) Creator() CreateFunction {
	return func(ctx context.Context, c Config) (CloudKV, error) {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context error: %w", ctx.Err())
		}
		kv := &MemStorage{
			m:          &publicMem,
			bucketName: c.BucketName,
			view:       make(map[string][]byte),
		}

		if err := kv.CheckExist(ctx); err != nil {
			return nil, err
		}

		return kv, nil
	}
}

func (m *MemStorage) ListByPrefix(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *MemStorage) GetObject(ctx context.Context, key string) (*minio.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MemStorage) StatObject(ctx context.Context, key string) (*minio.ObjectInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MemStorage) PutObject(ctx context.Context, opts *PutObjectOptions) (*minio.UploadInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MemStorage) RemoveObjects(ctx context.Context, keys []string) []error {
	for i := range keys {
		key := fmt.Sprintf("%s/%s", m.bucketName, keys[i])
		delete(m.view, key)
		m.m.Delete(key)
	}

	return nil
}

func (m *MemStorage) Put(ctx context.Context, key string, value []byte) error {
	newKey := fmt.Sprintf("%s/%s", m.bucketName, key)
	publicMem.Store(newKey, value)
	m.view[newKey] = value
	return nil
}

func (m *MemStorage) Get(ctx context.Context, key string) ([]byte, error) {
	v, ok := m.m.Load(fmt.Sprintf("%s/%s", m.bucketName, key))
	if !ok {
		return nil, ErrObjectNotFound
	}
	return v.([]byte), nil
}

func (m *MemStorage) GetMetadata(ctx context.Context, key string) (*Metadata, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MemStorage) CleanBucket(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (m *MemStorage) CheckExist(ctx context.Context) error {
	if strings.Contains(m.bucketName, "^") {
		return fmt.Errorf("invlaid backet")
	}
	return nil
}

func (m *MemStorage) CheckHasBeenDeleted(keys []string) error {
	for i := range keys {
		key := fmt.Sprintf("%s/%s", m.bucketName, keys[i])
		if _, ok := m.view[key]; ok {
			return fmt.Errorf("key %s still exists", key)
		}
		if _, ok := m.m.Load(key); ok {
			return fmt.Errorf("key %s still exists", key)
		}
	}
	return nil
}
