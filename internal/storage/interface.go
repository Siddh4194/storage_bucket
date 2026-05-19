package storage

import (
	"context"
	"io"
)

type Storage interface {
	CreateBucket(ctx context.Context, name string) error
	GetBucket(ctx context.Context, name string) (BucketInfo, error)
	ListBuckets(ctx context.Context) ([]BucketInfo, error)
	DeleteBucket(ctx context.Context, name string) error

	PutObject(ctx context.Context, bucket, key string, data io.Reader, size int64, contentType string, metadata map[string]string) (ObjectInfo, error)
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, ObjectInfo, error)
	HeadObject(ctx context.Context, bucket, key string) (ObjectInfo, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	ListObjects(ctx context.Context, bucket string, prefix string, delimiter string, maxKeys int, marker string) (ListObjectsResult, error)
}
