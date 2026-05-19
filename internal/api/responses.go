package api

import (
	"encoding/json"
	"net/http"

	"github.com/anomalyco/storage-bucket/internal/storage"
)

type ListBucketsResponse struct {
	Buckets []storage.BucketInfo `json:"buckets"`
}

type GetBucketResponse struct {
	Bucket storage.BucketInfo `json:"bucket"`
}

type CreateBucketRequest struct {
	Name string `json:"name"`
}

type ListObjectsResponse struct {
	Objects []storage.ObjectInfo `json:"objects,omitempty"`
	storage.ListObjectsResult
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
