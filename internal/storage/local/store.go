package local

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anomalyco/storage-bucket/internal/storage"
)

const metaSuffix = ".meta.json"

type Store struct {
	basePath string
}

func New(basePath string) (*Store, error) {
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolve base path: %w", err)
	}

	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("create base directory: %w", err)
	}

	return &Store{basePath: abs}, nil
}

func (s *Store) bucketPath(name string) string {
	return filepath.Join(s.basePath, name)
}

func (s *Store) objectPath(bucket, key string) string {
	return filepath.Join(s.basePath, bucket, key)
}

func (s *Store) metadataPath(bucket, key string) string {
	return s.objectPath(bucket, key) + ".meta.json"
}

func (s *Store) CreateBucket(_ context.Context, name string) error {
	path := s.bucketPath(name)

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return storage.ErrBucketAlreadyExist
		}
		return fmt.Errorf("path exists but is not a directory: %s", name)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("stat bucket path: %w", err)
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("create bucket directory: %w", err)
	}

	return nil
}

func (s *Store) GetBucket(_ context.Context, name string) (storage.BucketInfo, error) {
	path := s.bucketPath(name)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.BucketInfo{}, storage.ErrBucketNotFound
		}
		return storage.BucketInfo{}, fmt.Errorf("stat bucket: %w", err)
	}

	if !info.IsDir() {
		return storage.BucketInfo{}, storage.ErrBucketNotFound
	}

	return storage.BucketInfo{
		Name:      name,
		CreatedAt: info.ModTime().UTC(),
	}, nil
}

func (s *Store) ListBuckets(_ context.Context) ([]storage.BucketInfo, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("read base directory: %w", err)
	}

	var result []storage.BucketInfo
	for _, e := range entries {
		if e.IsDir() {
			info, err := e.Info()
			if err != nil {
				continue
			}
			result = append(result, storage.BucketInfo{
				Name:      e.Name(),
				CreatedAt: info.ModTime().UTC(),
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (s *Store) DeleteBucket(_ context.Context, name string) error {
	path := s.bucketPath(name)

	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.ErrBucketNotFound
		}
		return fmt.Errorf("read bucket directory: %w", err)
	}

	if len(entries) > 0 {
		return storage.ErrBucketNotEmpty
	}

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove bucket directory: %w", err)
	}

	return nil
}

func (s *Store) putObject(bucket, key string, data io.Reader, size int64, contentType string, metadata map[string]string) (storage.ObjectInfo, error) {
	dir := filepath.Dir(s.objectPath(bucket, key))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("create object directory: %w", err)
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
	objectPath := s.objectPath(bucket, key)
	now := time.Now().UTC()

	if err := os.WriteFile(objectPath, buf.Bytes(), 0644); err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("write object file: %w", err)
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}

	info := storage.ObjectInfo{
		Bucket:      bucket,
		Key:         key,
		Size:        n,
		ContentType: contentType,
		ETag:        etag,
		ModTime:     now,
		Metadata:    metadata,
	}

	metaBytes, err := json.Marshal(info)
	if err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("marshal metadata: %w", err)
	}

	if err := os.WriteFile(s.metadataPath(bucket, key), metaBytes, 0644); err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("write metadata file: %w", err)
	}

	return info, nil
}

func (s *Store) PutObject(ctx context.Context, bucket, key string, data io.Reader, size int64, contentType string, metadata map[string]string) (storage.ObjectInfo, error) {
	bucketPath := s.bucketPath(bucket)

	info, err := os.Stat(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.ObjectInfo{}, storage.ErrBucketNotFound
		}
		return storage.ObjectInfo{}, fmt.Errorf("stat bucket: %w", err)
	}

	if !info.IsDir() {
		return storage.ObjectInfo{}, storage.ErrBucketNotFound
	}

	return s.putObject(bucket, key, data, size, contentType, metadata)
}

func (s *Store) GetObject(_ context.Context, bucket, key string) (io.ReadCloser, storage.ObjectInfo, error) {
	info, err := s.HeadObject(context.Background(), bucket, key)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}

	data, err := os.ReadFile(s.objectPath(bucket, key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ObjectInfo{}, storage.ErrObjectNotFound
		}
		return nil, storage.ObjectInfo{}, fmt.Errorf("read object file: %w", err)
	}

	return io.NopCloser(bytes.NewReader(data)), info, nil
}

func (s *Store) HeadObject(_ context.Context, bucket, key string) (storage.ObjectInfo, error) {
	bucketPath := s.bucketPath(bucket)

	info, err := os.Stat(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.ObjectInfo{}, storage.ErrBucketNotFound
		}
		return storage.ObjectInfo{}, fmt.Errorf("stat bucket: %w", err)
	}

	if !info.IsDir() {
		return storage.ObjectInfo{}, storage.ErrBucketNotFound
	}

	objectPath := s.objectPath(bucket, key)
	finfo, err := os.Stat(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.ObjectInfo{}, storage.ErrObjectNotFound
		}
		return storage.ObjectInfo{}, fmt.Errorf("stat object: %w", err)
	}

	meta := loadMetadata(s.metadataPath(bucket, key))
	if meta == nil {
		meta = &storage.ObjectInfo{}
	}

	meta.Bucket = bucket
	meta.Key = key
	meta.Size = finfo.Size()
	meta.ModTime = finfo.ModTime().UTC()

	if meta.ETag == "" {
		meta.ETag = computeETag(objectPath)
	}
	if meta.Metadata == nil {
		meta.Metadata = make(map[string]string)
	}
	if meta.ContentType == "" {
		meta.ContentType = detectContentType(key)
	}

	return *meta, nil
}

func (s *Store) DeleteObject(_ context.Context, bucket, key string) error {
	bucketPath := s.bucketPath(bucket)

	info, err := os.Stat(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.ErrBucketNotFound
		}
		return fmt.Errorf("stat bucket: %w", err)
	}

	if !info.IsDir() {
		return storage.ErrBucketNotFound
	}

	objectPath := s.objectPath(bucket, key)
	if err := os.Remove(objectPath); err != nil {
		if os.IsNotExist(err) {
			return storage.ErrObjectNotFound
		}
		return fmt.Errorf("remove object: %w", err)
	}

	metaPath := s.metadataPath(bucket, key)
	os.Remove(metaPath)

	return nil
}

func (s *Store) ListObjects(_ context.Context, bucket string, prefix string, delimiter string, maxKeys int, marker string) (storage.ListObjectsResult, error) {
	bucketPath := s.bucketPath(bucket)

	info, err := os.Stat(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.ListObjectsResult{}, storage.ErrBucketNotFound
		}
		return storage.ListObjectsResult{}, fmt.Errorf("stat bucket: %w", err)
	}

	if !info.IsDir() {
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

	var objInfos []storage.ObjectInfo
	commonPrefixes := make(map[string]bool)

	err = filepath.Walk(bucketPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(bucketPath, path)
		if err != nil {
			return nil
		}

		key := filepath.ToSlash(rel)

		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}
		if strings.HasSuffix(key, metaSuffix) {
			return nil
		}

		if delimiter != "" {
			remainder := key[len(prefix):]
			if idx := strings.Index(remainder, delimiter); idx >= 0 {
				cp := prefix + remainder[:idx+len(delimiter)]
				commonPrefixes[cp] = true
				return nil
			}
		}

		objInfo := storage.ObjectInfo{
			Bucket:  bucket,
			Key:     key,
			Size:    fi.Size(),
			ModTime: fi.ModTime().UTC(),
		}

		objInfo.ETag = computeETag(path)

		meta := loadMetadata(path + metaSuffix)
		if meta != nil {
			objInfo.ContentType = meta.ContentType
			objInfo.Metadata = meta.Metadata
		}

		objInfos = append(objInfos, objInfo)
		return nil
	})
	if err != nil {
		return storage.ListObjectsResult{}, fmt.Errorf("walk bucket: %w", err)
	}

	sort.Slice(objInfos, func(i, j int) bool {
		return objInfos[i].Key < objInfos[j].Key
	})

	if marker != "" {
		idx := sort.Search(len(objInfos), func(i int) bool {
			return objInfos[i].Key >= marker
		})
		if idx < len(objInfos) && objInfos[idx].Key == marker {
			idx++
		}
		objInfos = objInfos[idx:]
	}

	for cp := range commonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, cp)
	}
	sort.Strings(result.CommonPrefixes)

	result.Objects = objInfos

	total := len(result.Objects) + len(result.CommonPrefixes)
	if total > maxKeys {
		result.IsTruncated = true
		if len(result.Objects) > maxKeys {
			result.Objects = result.Objects[:maxKeys]
			result.CommonPrefixes = nil
			result.NextMarker = result.Objects[len(result.Objects)-1].Key
		} else {
			remain := maxKeys - len(result.Objects)
			if remain < len(result.CommonPrefixes) {
				result.CommonPrefixes = result.CommonPrefixes[:remain]
				result.NextMarker = result.CommonPrefixes[len(result.CommonPrefixes)-1]
			}
		}
	}

	return result, nil
}

func loadMetadata(path string) *storage.ObjectInfo {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var info storage.ObjectInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil
	}
	return &info
}

func computeETag(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", md5.Sum(data))
}

func detectContentType(key string) string {
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".gz":
		return "application/gzip"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}
