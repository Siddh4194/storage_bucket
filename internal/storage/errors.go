package storage

import "errors"

var (
	ErrBucketNotFound    = errors.New("bucket not found")
	ErrBucketAlreadyExist = errors.New("bucket already exists")
	ErrBucketNotEmpty    = errors.New("bucket is not empty")
	ErrObjectNotFound    = errors.New("object not found")
)
