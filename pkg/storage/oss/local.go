package oss

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/pingcap/log"
	"go.uber.org/zap"
)

type Local struct {
	path string
}

func (s *Local) Creator() CreateFunction {
	return func(ctx context.Context, c Config) (CloudKV, error) {
		kv := &Local{
			path: path.Join(c.BucketName, c.Prefix),
		}

		err := os.MkdirAll(c.BucketName, 0777)

		return kv, err
	}
}

// Get method is used for get data from cloud
func (s *Local) Get(ctx context.Context, key string) ([]byte, error) {
	fname := s.abs(key)

	_, err := os.Lstat(fname)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}
	return os.ReadFile(fname)
}

// GetMetadata method is used for load the metadata of a specified object
func (s *Local) GetMetadata(ctx context.Context, key string) (*Metadata, error) {
	info, err := os.Lstat(s.abs(key))
	if err != nil {
		return nil, err
	}
	return &Metadata{LastModified: info.ModTime()}, nil
}

// Put method is used for persisting some data on cloud
func (s *Local) Put(ctx context.Context, key string, value []byte) error {
	fname := s.abs(key)
	log.Info("Put new file [key, full]", zap.String("key", key), zap.String("full", fname))
	dir, _ := path.Split(fname)
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	return os.WriteFile(fname, value, 0644)
}

// ListByPrefix list metadata of objects which has specified prefix in KV
func (s *Local) ListByPrefix(ctx context.Context, prefix string) ([]string, error) {
	r := []string{}
	filepath.Walk(s.path, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() && strings.HasPrefix(path, prefix) {
			r = append(r, path)
		}
		return nil
	})
	return r, nil
}

// CleanBucket clean all the contents inside a bucket
func (s *Local) CleanBucket(ctx context.Context) error {
	entries, err := os.ReadDir(s.path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := os.RemoveAll(entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Local) abs(key string) string {
	return path.Join(s.path, key)
}

func (s *Local) MinioClient() *minio.Client {
	panic("implement me")
}

func (s *Local) GetObject(ctx context.Context, key string) (*minio.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Local) PutObject(ctx context.Context, opts *PutObjectOptions) (*minio.UploadInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Local) RemoveObjects(ctx context.Context, keys []string) []error {
	//TODO implement me
	panic("implement me")
}

func (s *Local) StatObject(ctx context.Context, key string) (*minio.ObjectInfo, error) {
	//TODO implement me
	panic("implement me")
}

func init() {
	GetFactory().Put(PlatformInfoLOCAL, &Local{})
}

var _ CloudKV = (*Local)(nil)
