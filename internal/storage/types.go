package storage

import "time"

type BucketInfo struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ObjectInfo struct {
	Bucket      string            `json:"bucket"`
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type,omitempty"`
	ETag        string            `json:"etag,omitempty"`
	ModTime     time.Time         `json:"mod_time"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ListObjectsResult struct {
	IsTruncated   bool         `json:"is_truncated"`
	Marker        string       `json:"marker,omitempty"`
	NextMarker    string       `json:"next_marker,omitempty"`
	Prefix        string       `json:"prefix,omitempty"`
	Delimiter     string       `json:"delimiter,omitempty"`
	MaxKeys       int          `json:"max_keys"`
	CommonPrefixes []string    `json:"common_prefixes,omitempty"`
	Objects       []ObjectInfo `json:"objects,omitempty"`
}
