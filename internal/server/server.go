package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/anomalyco/storage-bucket/internal/api"
	"github.com/anomalyco/storage-bucket/internal/storage"
)

type Server struct {
	httpServer *http.Server
	handler    *api.Handler
}

func New(store storage.Storage, addr, corsOrigin string) *Server {
	mux := http.NewServeMux()
	handler := api.New(store)

	mux.HandleFunc("GET /health", handler.Health)

	mux.HandleFunc("GET /buckets", handler.ListBuckets)
	mux.HandleFunc("POST /buckets", handler.CreateBucket)
	mux.HandleFunc("GET /buckets/{name}", handler.GetBucket)
	mux.HandleFunc("DELETE /buckets/{name}", handler.DeleteBucket)

	mux.HandleFunc("GET /buckets/{name}/objects", handler.ListObjects)
	mux.HandleFunc("PUT /buckets/{name}/objects/{key...}", handler.PutObject)
	mux.HandleFunc("GET /buckets/{name}/objects/{key...}", handler.GetObject)
	mux.HandleFunc("HEAD /buckets/{name}/objects/{key...}", handler.HeadObject)
	mux.HandleFunc("DELETE /buckets/{name}/objects/{key...}", handler.DeleteObject)

	wrapped := withCORS(withLogger(mux), corsOrigin)

	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      wrapped,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		handler: handler,
	}
}

func (s *Server) Start() error {
	log.Printf("storage server listening on %s", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

func withCORS(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lrw.status, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
