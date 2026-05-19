# Storage Bucket API — Frontend Reference

Base URL: `http://localhost:8080`

All responses are JSON. All timestamps are ISO 8601 / RFC 3339.

---

## 1. Health Check

```
GET /health
```

**Response `200`**

```json
{ "status": "ok" }
```

---

## 2. List All Buckets

```
GET /buckets
```

**Response `200`**

```json
{
  "buckets": [
    {
      "name": "photos",
      "created_at": "2026-05-19T17:53:13Z"
    }
  ]
}
```

---

## 3. Create a Bucket

```
POST /buckets
Content-Type: application/json

{ "name": "my-bucket" }
```

**Response `201`**

```json
{
  "bucket": {
    "name": "my-bucket",
    "created_at": "2026-05-19T17:53:13Z"
  }
}
```

**Errors**

| Status | Code | When |
|--------|------|------|
| `400` | `BadRequest` | Missing or empty name |
| `409` | `Conflict` | Bucket already exists |

---

## 4. Get Bucket Info

```
GET /buckets/{name}
```

**Response `200`**

```json
{
  "bucket": {
    "name": "my-bucket",
    "created_at": "2026-05-19T17:53:13Z"
  }
}
```

**Errors:** `404` — bucket not found.

---

## 5. Delete a Bucket

```
DELETE /buckets/{name}
```

**Response `204`** (no body).

**Errors**

| Status | Code | When |
|--------|------|------|
| `404` | `NotFound` | Bucket doesn't exist |
| `409` | `Conflict` | Bucket still has objects inside |

> Your UI should show an error if the bucket isn't empty. The user must
> delete all objects first.

---

## 6. List Objects in a Bucket

```
GET /buckets/{name}/objects
```

### Query Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `prefix` | string | — | Filter keys starting with this prefix |
| `delimiter` | string | — | Group keys by a delimiter (e.g. `/` for folder-like behaviour) |
| `max-keys` | int | `1000` | Max items per page |
| `marker` | string | — | Start after this key (for pagination) |

### Example Requests

```
GET /buckets/photos/objects
GET /buckets/photos/objects?prefix=2026/
GET /buckets/photos/objects?prefix=2026/&delimiter=/
GET /buckets/photos/objects?max-keys=50&marker=2026/05/sunset.jpg
```

### Response `200`

```json
{
  "objects": [
    {
      "bucket": "photos",
      "key": "2026/05/sunset.jpg",
      "size": 2847291,
      "content_type": "image/jpeg",
      "etag": "a1b2c3d4e5f6...",
      "mod_time": "2026-05-19T17:53:13Z",
      "metadata": {}
    }
  ],
  "is_truncated": false,
  "marker": "",
  "next_marker": "",
  "prefix": "",
  "delimiter": "",
  "max_keys": 1000,
  "common_prefixes": []
}
```

### Pagination

- `is_truncated: true` means there are more results.
- Use `next_marker` as the `marker` parameter in the next request.
- When using `delimiter`, `common_prefixes` contains grouped "folder" names
  (e.g. `["2026/", "2025/"]`), and objects directly under the prefix appear
  in the `objects` array.

### UI Hint

Treat `common_prefixes` as folders and `objects` as files in the current
directory. Your file browser can show a breadcrumb or tree view using the
prefix + delimiter pattern.

---

## 7. Upload an Object

```
PUT /buckets/{name}/objects/{key}
Content-Type: image/jpeg          (or whatever the file type is)
Content-Length: 2847291           (optional, the server can auto-detect)

<binary file data>
```

> `{key}` can include slashes (`/`) to simulate folders. For example:
> `PUT /buckets/photos/objects/2026/05/vacation.jpg`

**Response `201`**

```json
{
  "bucket": "photos",
  "key": "2026/05/vacation.jpg",
  "size": 2847291,
  "content_type": "image/jpeg",
  "etag": "a1b2c3d4e5f6...",
  "mod_time": "2026-05-19T17:53:13Z",
  "metadata": {}
}
```

Also returns an `ETag` response header with the MD5 hash.

**Errors**

| Status | Code | When |
|--------|------|------|
| `404` | `NotFound` | Bucket doesn't exist |

### Frontend Upload UI

Use `FormData` or send the raw file bytes as the request body.  If you want
to show a progress bar, you'll need `XMLHttpRequest` with the `upload`
progress event — `fetch` doesn't natively support upload progress.

Example (using fetch):

```js
const response = await fetch(
  `http://localhost:8080/buckets/photos/objects/sunset.jpg`,
  { method: 'PUT', body: fileBlob }
);
const info = await response.json();
```

---

## 8. Download an Object

```
GET /buckets/{name}/objects/{key}
```

**Response `200`** — the raw file bytes.

Response headers:

| Header | Value |
|--------|-------|
| `Content-Type` | The type sent during upload (or auto-detected) |
| `Content-Length` | File size in bytes |
| `ETag` | MD5 hash |
| `Last-Modified` | Timestamp |

### Frontend Download UI

You can create a download link or trigger a browser download:

```js
// Option 1: Direct link (opens in browser / download)
window.open(`http://localhost:8080/buckets/photos/objects/sunset.jpg`);

// Option 2: Fetch + save (for custom UI)
const response = await fetch(
  `http://localhost:8080/buckets/photos/objects/sunset.jpg`
);
const blob = await response.blob();
const url = URL.createObjectURL(blob);
const a = document.createElement('a');
a.href = url;
a.download = 'sunset.jpg';
a.click();
URL.revokeObjectURL(url);
```

**Errors:** `404` — bucket or object not found.

---

## 9. Get Object Metadata (HEAD)

```
HEAD /buckets/{name}/objects/{key}
```

Same as download but without the body. Use this to check if a file exists,
get its size, or get its ETag before deciding whether to download.

Response headers are identical to download (`Content-Type`, `Content-Length`,
`ETag`, `Last-Modified`).

**Response `200`** — headers only, no body.

---

## 10. Delete an Object

```
DELETE /buckets/{name}/objects/{key}
```

**Response `204`** (no body).

**Errors:** `404` — bucket or object not found.

---

## Error Response Format

Every error returns JSON:

```json
{
  "code": "NotFound",
  "message": "The specified bucket does not exist"
}
```

### All Error Codes

| HTTP Status | Code | Typical Cause |
|------------|------|---------------|
| `400` | `BadRequest` | Invalid JSON, missing required fields |
| `404` | `NotFound` | Bucket or object doesn't exist |
| `409` | `Conflict` | Bucket already exists / bucket not empty |
| `500` | `InternalError` | Something went wrong on the server |

---

## Quick Reference For Frontend State

```
┌──────────────────────────────────────────────────┐
│  GET  /buckets              → bucket list       │
│  POST /buckets              → create bucket     │
│  DELETE /buckets/:name      → delete bucket     │
│                                                    │
│  GET  /buckets/:name/objects        → file list  │
│  PUT  /buckets/:name/objects/:key   → upload     │
│  GET  /buckets/:name/objects/:key   → download   │
│  HEAD /buckets/:name/objects/:key   → metadata   │
│  DELETE /buckets/:name/objects/:key  → delete    │
└──────────────────────────────────────────────────┘
```

---

## CORS

Currently CORS is not configured. If the frontend runs on a different origin
(e.g. `http://localhost:5173` for Vite), you'll hit CORS errors. Let me know
if you need CORS headers added to the server.
