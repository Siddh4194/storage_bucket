package memory

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anomalyco/storage-bucket/internal/storage"
)

type object struct {
	data      []byte
	info      storage.ObjectInfo
}

type bucket struct {
	info    storage.BucketInfo
	objects map[string]*object
}

type Store struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
}

func New() *Store {
	return &Store{
		buckets: make(map[string]*bucket),
	}
}

func (s *Store) CreateBucket(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.buckets[name]; ok {
		return storage.ErrBucketAlreadyExist
	}

	s.buckets[name] = &bucket{
		info: storage.BucketInfo{
			Name:      name,
			CreatedAt: time.Now().UTC(),
		},
		objects: make(map[string]*object),
	}
	return nil
}

func (s *Store) GetBucket(_ context.Context, name string) (storage.BucketInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.buckets[name]
	if !ok {
		return storage.BucketInfo{}, storage.ErrBucketNotFound
	}
	return b.info, nil
}

func (s *Store) ListBuckets(_ context.Context) ([]storage.BucketInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]storage.BucketInfo, 0, len(s.buckets))
	for _, b := range s.buckets {
		result = append(result, b.info)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (s *Store) DeleteBucket(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[name]
	if !ok {
		return storage.ErrBucketNotFound
	}
	if len(b.objects) > 0 {
		return storage.ErrBucketNotEmpty
	}

	delete(s.buckets, name)
	return nil
}

func (s *Store) PutObject(_ context.Context, bucket, key string, data io.Reader, size int64, contentType string, metadata map[string]string) (storage.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[bucket]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrBucketNotFound
	}

	buf := new(bytes.Buffer)
	n, err := io.Copy(buf, data)
	if err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("read data: %w", err)
	}

	if size > 0 && n != size {
		return storage.ObjectInfo{}, fmt.Errorf("size mismatch: expected %d, got %d", size, n)
	}

	etag := fmt.Sprintf("%x", md5.Sum(buf.Bytes()))

	now := time.Now().UTC()
	obj := &object{
		data: buf.Bytes(),
		info: storage.ObjectInfo{
			Bucket:      bucket,
			Key:         key,
			Size:        n,
			ContentType: contentType,
			ETag:        etag,
			ModTime:     now,
			Metadata:    metadata,
		},
	}

	if obj.info.Metadata == nil {
		obj.info.Metadata = make(map[string]string)
	}

	b.objects[key] = obj
	return obj.info, nil
}

func (s *Store) GetObject(_ context.Context, bucket, key string) (io.ReadCloser, storage.ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.buckets[bucket]
	if !ok {
		return nil, storage.ObjectInfo{}, storage.ErrBucketNotFound
	}

	obj, ok := b.objects[key]
	if !ok {
		return nil, storage.ObjectInfo{}, storage.ErrObjectNotFound
	}

	return io.NopCloser(bytes.NewReader(obj.data)), obj.info, nil
}

func (s *Store) HeadObject(_ context.Context, bucket, key string) (storage.ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.buckets[bucket]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrBucketNotFound
	}

	obj, ok := b.objects[key]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrObjectNotFound
	}

	return obj.info, nil
}

func (s *Store) DeleteObject(_ context.Context, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[bucket]
	if !ok {
		return storage.ErrBucketNotFound
	}

	if _, ok := b.objects[key]; !ok {
		return storage.ErrObjectNotFound
	}

	delete(b.objects, key)
	return nil
}

func (s *Store) ListObjects(_ context.Context, bucket string, prefix string, delimiter string, maxKeys int, marker string) (storage.ListObjectsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.buckets[bucket]
	if !ok {
		return storage.ListObjectsResult{}, storage.ErrBucketNotFound
	}

	if maxKeys <= 0 {
		maxKeys = 1000
	}

	result := storage.ListObjectsResult{
		Prefix:    prefix,
		Delimiter: delimiter,
		MaxKeys:   maxKeys,
	}

	keys := make([]string, 0, len(b.objects))
	for k := range b.objects {
		if prefix != "" && !strings.HasPrefix(k, prefix) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if marker != "" {
		idx := sort.SearchStrings(keys, marker)
		if idx < len(keys) && keys[idx] == marker {
			idx++
		}
		keys = keys[idx:]
	}

	seenPrefixes := make(map[string]bool)
	for _, k := range keys {
		if delimiter != "" {
			remainder := k[len(prefix):]
			if idx := strings.Index(remainder, delimiter); idx >= 0 {
				cp := prefix + remainder[:idx+len(delimiter)]
				if !seenPrefixes[cp] {
					seenPrefixes[cp] = true
					result.CommonPrefixes = append(result.CommonPrefixes, cp)
				}
				continue
			}
		}

		result.Objects = append(result.Objects, b.objects[k].info)
	}

	total := len(result.Objects) + len(result.CommonPrefixes)
	if total > maxKeys {
		result.IsTruncated = true

		visible := 0
		var last string

		for _, k := range keys {
			if visible >= maxKeys {
				break
			}

			if delimiter != "" {
				remainder := k[len(prefix):]
				if idx := strings.Index(remainder, delimiter); idx >= 0 {
					cp := prefix + remainder[:idx+len(delimiter)]
					if _, seen := seenPrefixes[cp]; !seen || visible >= maxKeys {
						continue
					}
				}
			}

			visible++
			last = k
		}

		if visible >= maxKeys && last != "" {
			result.NextMarker = last
		}

		numObjs := len(result.Objects)
		numCps := len(result.CommonPrefixes)

		if numObjs+numCps > maxKeys {
			if numObjs > maxKeys {
				result.Objects = result.Objects[:maxKeys]
				result.CommonPrefixes = nil
			} else {
				result.CommonPrefixes = result.CommonPrefixes[:maxKeys-numObjs]
			}
		}
	}

	return result, nil
}
