# Phase 1 — Foundation

## What we are building

A mini-S3 server in Go. Think of it like running your own little AWS S3 on your
laptop or server. You can create buckets, upload files, download them, list
what you've stored, and delete stuff — all through HTTP requests, just like
the real S3 API.

This phase lays the groundwork so everything after can be built on solid
ground.

---

## Project layout (what goes where)

```
storage_bucket/
├── cmd/
│   └── storage-server/main.go      # The actual program you run
├── internal/
│   ├── storage/                    # Core storage logic
│   │   ├── types.go                # The data shapes (Bucket, Object, etc.)
│   │   ├── interface.go            # The contract every storage backend must follow
│   │   ├── errors.go               # Errors that make sense ("bucket not found")
│   │   ├── memory/                 # In-memory backend (fast, ephemeral)
│   │   │   └── store.go
│   │   └── local/                  # Filesystem backend (persistent to disk)
│   │       └── store.go
│   ├── server/                     # HTTP server & routing
│   │   └── server.go
│   ├── api/                        # Request handlers & response formatting
│   │   ├── handlers.go
│   │   ├── responses.go
│   │   └── errors.go
│   └── config/                     # Loading settings (port, data dir, etc.)
│       └── config.go
├── docs/                           # You are here
├── go.mod                          # Go module definition
└── go.sum                          # Dependency checksums
```

Why `internal/`? Go treats the `internal` package specially — nothing outside
the project can import it. That's perfect because these are our internals.
The public API (if we ever make one) would live in `pkg/`.

---

## The storage contract (the interface)

Every storage backend — whether it keeps data in RAM, on disk, or (later)
proxies to real S3 — must implement the same set of methods:

```go
type Storage interface {
    // Bucket operations
    CreateBucket(ctx, name)
    GetBucket(ctx, name)        → BucketInfo
    ListBuckets(ctx)             → []BucketInfo
    DeleteBucket(ctx, name)

    // Object operations
    PutObject(ctx, bucket, key, data, size, contentType, metadata)
        → ObjectInfo

    GetObject(ctx, bucket, key) → data + ObjectInfo
    HeadObject(ctx, bucket, key)→ ObjectInfo
    DeleteObject(ctx, bucket, key)

    ListObjects(ctx, bucket, prefix, delimiter, maxKeys, marker)
        → ListObjectsResult
}
```

This means we can swap backends without touching any handler code. The HTTP
layer talks to the interface; the backend does the heavy lifting.

---

## The backends (Phase 1)

### In-memory (`internal/storage/memory/`)

- Data lives in Go maps: `map[string]Bucket`
- Each bucket has `map[string]Object`
- Blazing fast, nothing survives a restart
- Perfect for tests and demos

### Local filesystem (`internal/storage/local/`)

- Buckets = directories on disk
- Objects = files inside those directories
- Metadata stored as a tiny JSON sidecar file per object
- Data survives restarts — you can point it at a folder and it just works
- Great for development and small-scale production

---

## The HTTP server

We use Go's built-in `net/http` package — no external framework needed for
Phase 1. The server listens on a configurable address (default `:8080`) and
routes requests to handlers that speak a JSON API.

### Endpoints (Phase 1 — JSON flavour)

| Method | Path | What it does |
|--------|------|-------------|
| GET | `/buckets` | List all buckets |
| POST | `/buckets` | Create a bucket |
| GET | `/buckets/{name}` | Get bucket info |
| DELETE | `/buckets/{name}` | Delete a bucket |
| GET | `/buckets/{name}/objects` | List objects (supports `?prefix=&delimiter=&max-keys=&marker=`) |
| PUT | `/buckets/{name}/objects/{key...}` | Upload an object |
| GET | `/buckets/{name}/objects/{key...}` | Download an object |
| HEAD | `/buckets/{name}/objects/{key...}` | Get object metadata |
| DELETE | `/buckets/{name}/objects/{key...}` | Delete an object |

*Note: In Phase 2 we'll switch to S3-compatible paths (`/` for ListBuckets,
`/{bucket}` for bucket ops, `/{bucket}/{key}` for object ops) and XML
responses. Phase 1 keeps it simple with a clean JSON API.*

---

## Configuration

Settings come from environment variables or a config file:

| Variable | Default | What it controls |
|----------|---------|-----------------|
| `STORAGE_BACKEND` | `memory` | Which backend to use (`memory` or `local`) |
| `STORAGE_LOCAL_PATH` | `./data` | Where local backend stores files |
| `STORAGE_LISTEN` | `:8080` | Address the HTTP server binds to |

---

## How to run

```bash
# In-memory mode (default)
go run ./cmd/storage-server

# Local filesystem mode
$env:STORAGE_BACKEND="local"
go run ./cmd/storage-server

# Custom port
$env:STORAGE_LISTEN=":9000"
go run ./cmd/storage-server
```

Then point your browser or curl at `http://localhost:8080`.

---

## What's coming in Phase 2

- S3-compatible endpoint paths (`/`, `/{bucket}`, `/{bucket}/{key}`)
- XML response format (matching real S3)
- Multipart uploads
- ETag support for caching
- Better error codes matching S3 semantics
- A third backend that proxies to real AWS S3
