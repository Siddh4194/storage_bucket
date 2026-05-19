package api

import (
	"encoding/json"
	"net/http"

	"github.com/anomalyco/storage-bucket/internal/storage"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Code: code, Message: message})
}

func writeNotFound(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotFound, "NotFound", msg)
}

func writeConflict(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusConflict, "Conflict", msg)
}

func writeBadRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, "BadRequest", msg)
}

func writeInternalError(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusInternalServerError, "InternalError", msg)
}

func mapStorageError(w http.ResponseWriter, err error) {
	switch err {
	case storage.ErrBucketNotFound:
		writeNotFound(w, "The specified bucket does not exist")
	case storage.ErrBucketAlreadyExist:
		writeConflict(w, "The requested bucket already exists")
	case storage.ErrBucketNotEmpty:
		writeConflict(w, "The bucket you tried to delete is not empty")
	case storage.ErrObjectNotFound:
		writeNotFound(w, "The specified object does not exist")
	default:
		writeInternalError(w, err.Error())
	}
}
