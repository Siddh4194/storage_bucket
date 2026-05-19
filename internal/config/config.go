package config

import (
	"fmt"
	"os"
)

type Config struct {
	Backend    string
	LocalPath  string
	ListenAddr string
	CORSOrigin string
}

func Load() (*Config, error) {
	backend := envOrDefault("STORAGE_BACKEND", "local")
	if backend != "memory" && backend != "local" {
		return nil, fmt.Errorf("invalid STORAGE_BACKEND %q: must be 'memory' or 'local'", backend)
	}

	localPath := envOrDefault("STORAGE_LOCAL_PATH", "/storage-bucket-data")

	listen := envOrDefault("STORAGE_LISTEN", ":8080")
	corsOrigin := envOrDefault("STORAGE_CORS_ORIGIN", "http://localhost:3000")

	return &Config{
		Backend:    backend,
		LocalPath:  localPath,
		ListenAddr: listen,
		CORSOrigin: corsOrigin,
	}, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
