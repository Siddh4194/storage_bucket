package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anomalyco/storage-bucket/internal/config"
	"github.com/anomalyco/storage-bucket/internal/server"
	"github.com/anomalyco/storage-bucket/internal/storage"
	"github.com/anomalyco/storage-bucket/internal/storage/local"
	"github.com/anomalyco/storage-bucket/internal/storage/memory"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var store storage.Storage
	switch cfg.Backend {
	case "memory":
		store = memory.New()
		log.Println("using in-memory storage backend")
	case "local":
		s, err := local.New(cfg.LocalPath)
		if err != nil {
			log.Fatalf("local storage: %v", err)
		}
		store = s
		log.Printf("using local filesystem storage backend: %s", cfg.LocalPath)
	default:
		log.Fatalf("unknown backend: %s", cfg.Backend)
	}

	srv := server.New(store, cfg.ListenAddr, cfg.CORSOrigin)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-quit
		log.Printf("received signal %v, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("shutdown error: %v", err)
		}
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
