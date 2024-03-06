package oss

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

// Storage for read or write big size data
type Storage interface {
	// // ListObjects list all objects in the storage.
	// ListObjects(ctx context.Context, opts *pb.ListObjectsRequest) (*pb.ListObjectsResponse, error)
	// The GetObject method is used for get data from cloud
	GetObject(ctx context.Context, key string) (*minio.Object, error)
	// StatObject check the object stat
	StatObject(ctx context.Context, key string) (*minio.ObjectInfo, error)
	// PutObject method is used for persisting some data on cloud
	PutObject(ctx context.Context, opts *PutObjectOptions) (*minio.UploadInfo, error)
	// RemoveObjects method is used for remove objects from cloud
	RemoveObjects(ctx context.Context, keys []string) []error
}

type PutObjectOptions struct {
	Key      string
	Size     int
	Reader   io.Reader
	Metadata map[string]string
}
