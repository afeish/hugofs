package oss

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrObjectNotFound = errors.New("object not found")
	ErrAccessDenied   = errors.New("access denied")
	ErrConflict       = errors.New("conflict, bucket not empty")
)

type Metadata struct {
	// LastModified Creation date of the object.
	LastModified time.Time
}

// CloudKV abstracts a K/V storage based on S3 from a multiple cloud platform.
type CloudKV interface {
	Storage

	// Get method is used for get data from cloud
	Get(ctx context.Context, key string) ([]byte, error)
	// The GetMetadata method is used for load the metadata of a specified object
	GetMetadata(ctx context.Context, key string) (*Metadata, error)
	// Put method is used for persisting some data on cloud
	Put(ctx context.Context, key string, value []byte) error
	// ListByPrefix list metadata of objects which has specified prefix in KV
	ListByPrefix(ctx context.Context, prefix string) ([]string, error)
	// CleanBucket clean all the contents inside a bucket
	CleanBucket(ctx context.Context) error
}

// BuildCloudKV the builder for different cloud platform's CloudKV.
func BuildCloudKV(ctx context.Context, c Config) (CloudKV, error) {
	fn, ok := GetFactory().Get(c._platform)
	if !ok {
		return nil, fmt.Errorf("unknown platform :%s", c._platform.String())
	}

	return fn(ctx, c)
}
