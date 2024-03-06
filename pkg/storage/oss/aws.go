package oss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pingcap/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type AwsS3 struct {
	config Config

	*minio.Client
}

func s3CompliantCreator[T interface {
	~struct {
		CloudKV
	}
	CloudKV
}]() CreateFunction {
	return func(ctx context.Context, c Config) (CloudKV, error) {
		aws := &AwsS3{}
		creator := aws.Creator()
		delegate, err := creator(ctx, c)
		if err != nil {
			return nil, err
		}

		kv := T{
			CloudKV: delegate,
		}

		return kv, nil
	}
}

func (s *AwsS3) Creator() CreateFunction {
	return func(ctx context.Context, c Config) (CloudKV, error) {
		s3Client, err := minio.New(c.toEndpoint(), &minio.Options{
			Creds:  credentials.NewStaticV4(c.AccessKey, c.SecureKey, ""),
			Region: c.Region,
			Secure: true,
		})
		if err != nil {
			return nil, err
		}

		kv := &AwsS3{
			config: c,
			Client: s3Client,
		}

		if err := kv.checkIfBucketExists(ctx); err != nil {
			return nil, err
		}

		kv.Client, _ = minio.New(c.toEndpoint(), &minio.Options{
			Creds:  credentials.NewStaticV4(c.AccessKey, c.SecureKey, ""),
			Region: c.Region,
			Secure: true,
		}) // tencent cos scenario: even we create the bucket, but may still get bucket not exists error. So we reset our client

		return kv, nil
	}
}

func (s *AwsS3) Get(ctx context.Context, key string) ([]byte, error) {
	object, err := s.Client.GetObject(ctx, s.config.effectiveBucket(), s.config.effectiveKey(key), minio.GetObjectOptions{})
	if err != nil {
		if err, ok := err.(minio.ErrorResponse); ok {
			if err.Code == "NoSuchKey" {
				return nil, ErrObjectNotFound
			}
		}
		return nil, err
	}

	defer object.Close()

	var out bytes.Buffer
	if _, err = io.Copy(&out, object); err != nil {
		if err, ok := err.(minio.ErrorResponse); ok {
			if err.Code == "NoSuchKey" {
				return nil, ErrObjectNotFound
			}
		}
		return nil, err
	}

	return out.Bytes(), nil

}

func (s *AwsS3) GetMetadata(ctx context.Context, key string) (*Metadata, error) {
	stat, err := s.Client.StatObject(ctx, s.config.effectiveBucket(), s.config.effectiveKey(key), minio.StatObjectOptions{})
	if err != nil {
		if err, ok := err.(minio.ErrorResponse); ok {
			if err.Code == "NoSuchKey" {
				return nil, ErrObjectNotFound
			}
		}
		return nil, err
	}

	return &Metadata{
		LastModified: stat.LastModified,
	}, nil

}

func (s *AwsS3) Put(ctx context.Context, key string, value []byte) error {
	bucket := s.config.effectiveBucket()

	log.S().Debugf("put object key [ %s ] to bucket [ %s ]", key, bucket)
	_, err := s.Client.PutObject(ctx, bucket, s.config.effectiveKey(key), bytes.NewBuffer(value), int64(len(value)), minio.PutObjectOptions{})
	if err, ok := err.(minio.ErrorResponse); ok {
		if err.Code == "NoSuchBucket" {
			log.Error("the bucket should be created first")
		}
	}

	return err
}

func (s *AwsS3) ListByPrefix(ctx context.Context, prefix string) ([]string, error) {
	ch := s.Client.ListObjects(ctx, s.config.effectiveBucket(), minio.ListObjectsOptions{Prefix: s.config.effectiveKey(prefix), Recursive: true})

	var keys []string
	for c := range ch {
		keys = append(keys, c.Key)
	}
	return keys, nil
}

// CleanBucket clean all the contents inside a bucket
func (s *AwsS3) CleanBucket(ctx context.Context) error {
	objInfos := s.Client.ListObjects(ctx, s.config.effectiveBucket(), minio.ListObjectsOptions{Prefix: s.config.Prefix, Recursive: true})

	err := <-s.Client.RemoveObjects(ctx, s.config.effectiveBucket(), objInfos, minio.RemoveObjectsOptions{})
	if err.Err != nil {
		return errors.Wrap(err.Err, "err remove object")
	}
	return nil
}

func (s *AwsS3) checkIfBucketExists(ctx context.Context) error {
	bucket := s.config.effectiveBucket()

	bucketExists, err := s.Client.BucketExists(ctx, bucket)
	if err != nil {
		return errors.Wrapf(err, "err check bucket [ %s ] exists", bucket)
	}
	if !bucketExists {
		log.Error("bucket not existed", zap.String("bucket", bucket))
		return fmt.Errorf("not find bucket: [%s], check your config", bucket)
	}

	log.Info("bucket exists", zap.String("bucket", bucket))
	return nil
}

func (s *AwsS3) GetObject(ctx context.Context, key string) (*minio.Object, error) {
	object, err := s.Client.GetObject(ctx,
		s.config.effectiveBucket(),
		s.config.storageKey(key),
		minio.GetObjectOptions{})
	if err != nil {
		if err, ok := err.(minio.ErrorResponse); ok {
			switch err.StatusCode {
			case http.StatusNotFound:
				return nil, ErrObjectNotFound
			case http.StatusForbidden:
				return nil, ErrAccessDenied
			case http.StatusConflict:
				return nil, ErrConflict
			}
			if err.Code == "NoSuchKey" {
				return nil, ErrObjectNotFound
			}
		}
		return nil, err
	}
	return object, nil
}

func (s *AwsS3) StatObject(ctx context.Context, key string) (*minio.ObjectInfo, error) {
	object, err := s.Client.StatObject(ctx,
		s.config.effectiveBucket(),
		s.config.storageKey(key),
		minio.GetObjectOptions{})
	if err != nil {
		if err, ok := err.(minio.ErrorResponse); ok {
			switch err.StatusCode {
			case http.StatusNotFound:
				return nil, ErrObjectNotFound
			case http.StatusForbidden:
				return nil, ErrAccessDenied
			case http.StatusConflict:
				return nil, ErrConflict
			}
			if err.Code == "NoSuchKey" {
				return nil, ErrObjectNotFound
			}
		}
		return nil, err
	}
	return &object, nil
}

func (s *AwsS3) PutObject(ctx context.Context, opts *PutObjectOptions) (*minio.UploadInfo, error) {
	if opts == nil {
		err := errors.New("invalid list object options")
		return nil, err
	}

	uploadInfo, err := s.Client.PutObject(ctx,
		s.config.effectiveBucket(),
		s.config.storageKey(opts.Key),
		opts.Reader,
		int64(opts.Size),
		minio.PutObjectOptions{
			UserMetadata: opts.Metadata,
		})
	if err, ok := err.(minio.ErrorResponse); ok {
		if err.Code == "NoSuchBucket" {
			log.Error("the bucket should be created first")
		}
	}

	return &uploadInfo, err
}

func (s *AwsS3) RemoveObjects(ctx context.Context, keys []string) []error {
	var (
		removeObjErr []error
		wg           sync.WaitGroup
	)
	for _, k := range keys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			if err := s.Client.RemoveObject(ctx,
				s.config.effectiveBucket(),
				s.config.storageKey(key),
				minio.RemoveObjectOptions{}); err != nil {
				removeObjErr = append(removeObjErr, err)
			}
		}(k)
	}
	wg.Wait()
	return removeObjErr
}

func init() {
	GetFactory().Put(PlatformInfoAWS, &AwsS3{})
}
