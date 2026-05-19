package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/anomalyco/storage-bucket/internal/storage"
)

type Handler struct {
	store storage.Storage
}

func New(store storage.Storage) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := h.store.ListBuckets(r.Context())
	if err != nil {
		mapStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ListBucketsResponse{Buckets: buckets})
}

func (h *Handler) CreateBucket(w http.ResponseWriter, r *http.Request) {
	var req CreateBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeBadRequest(w, "bucket name is required")
		return
	}

	if err := h.store.CreateBucket(r.Context(), req.Name); err != nil {
		mapStorageError(w, err)
		return
	}

	bucket, err := h.store.GetBucket(r.Context(), req.Name)
	if err != nil {
		mapStorageError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, GetBucketResponse{Bucket: bucket})
}

func (h *Handler) GetBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	bucket, err := h.store.GetBucket(r.Context(), name)
	if err != nil {
		mapStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, GetBucketResponse{Bucket: bucket})
}

func (h *Handler) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.store.DeleteBucket(r.Context(), name); err != nil {
		mapStorageError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListObjects(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	marker := r.URL.Query().Get("marker")

	maxKeys := 1000
	if v := r.URL.Query().Get("max-keys"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err == nil && parsed > 0 {
			maxKeys = parsed
		}
	}

	result, err := h.store.ListObjects(r.Context(), name, prefix, delimiter, maxKeys, marker)
	if err != nil {
		mapStorageError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ListObjectsResponse{
		Objects:          result.Objects,
		ListObjectsResult: result,
	})
}

func (h *Handler) PutObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("name")
	key := r.PathValue("key")

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	size := r.ContentLength

	info, err := h.store.PutObject(r.Context(), bucket, key, r.Body, size, contentType, nil)
	if err != nil {
		mapStorageError(w, err)
		return
	}

	w.Header().Set("ETag", info.ETag)
	writeJSON(w, http.StatusCreated, info)
}

func (h *Handler) GetObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("name")
	key := r.PathValue("key")

	data, info, err := h.store.GetObject(r.Context(), bucket, key)
	if err != nil {
		mapStorageError(w, err)
		return
	}
	defer data.Close()

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.ModTime.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, data)
}

func (h *Handler) HeadObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("name")
	key := r.PathValue("key")

	info, err := h.store.HeadObject(r.Context(), bucket, key)
	if err != nil {
		mapStorageError(w, err)
		return
	}

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.ModTime.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("name")
	key := r.PathValue("key")

	if err := h.store.DeleteObject(r.Context(), bucket, key); err != nil {
		mapStorageError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
