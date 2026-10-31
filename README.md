# supabase-insert-helper

A lightweight, production-oriented Go microservice for **mass concurrent ingestion of images and records into Supabase**. It receives a batch of public image URLs or database records via HTTP, validates the payload immediately, and delegates the heavy lifting to a configurable pool of workers so the web server never blocks.

This project follows the [Standard Go Project Layout](https://github.com/golang-standards/project-layout) and uses only the Go standard library plus `log/slog` and `godotenv`. No web frameworks such as Gin or Fiber are used.

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [How It Works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Local Installation](#local-installation)
- [Configuration](#configuration)
- [Running with Docker](#running-with-docker)
- [API Reference](#api-reference)
- [Python Client Example](#python-client-example)
- [Tuning the Worker Pool](#tuning-the-worker-pool)
- [Security Notes](#security-notes)
- [Project Background](#project-background)

---

## Features

- **Concurrent image ingestion**: download public images and upload them to Supabase Storage using a fixed-size goroutine worker pool.
- **Non-blocking HTTP API**: every request is validated, enqueued, and acknowledged with `202 Accepted` while workers process in the background.
- **Singleton Supabase HTTP client**: one reusable `http.Client` is initialized at startup and shared by all workers.
- **Structured logging**: all events are emitted through `log/slog` with key/value attributes.
- **Dockerized deployment**: multi-stage Dockerfile plus Docker Compose service ready to run.
- **Python example client**: a script that sends batches of image URLs without writing HTTP boilerplate.
- **Configurable concurrency**: `WORKER_COUNT` can be adjusted without recompiling.

---

## Architecture

```text
supabase-insert-helper/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: env loading, singleton init, HTTP server
├── internal/
│   ├── api/
│   │   ├── image_handler.go     # HTTP validation + JSON parsing for /ingest/images
│   │   └── response.go          # Standard JSON response/error wrapper
│   ├── models/
│   │   └── models.go            # Data structures: payloads, jobs, results, client
│   ├── storage/
│   │   ├── supabase_client.go   # Thread-safe Supabase client singleton
│   │   └── upload.go            # Download image + upload to Supabase Storage
│   └── workers/
│       ├── pool.go              # Goroutine worker pool engine
│       └── processor.go         # Storage processor + result collector
├── scripts/
│   └── upload_images.py         # Example Python client
├── Dockerfile                   # Multi-stage Go build
├── docker-compose.yml           # Docker Compose service
├── .env                         # Environment variables (not versioned)
├── go.mod
└── go.sum
```

### Key Components

| Package | Responsibility |
|---|---|
| `cmd/server` | Wires everything together and starts the HTTP server. |
| `internal/api` | Parses requests, validates payloads, and writes standardized JSON responses. |
| `internal/models` | Defines the shape of HTTP payloads, worker jobs, and results. |
| `internal/storage` | Singleton Supabase client and the actual Storage REST upload logic. |
| `internal/workers` | Worker pool: channels, goroutines, WaitGroup, and context-aware shutdown. |

---

## How It Works

The microservice receives a batch of image URLs via `POST /ingest/images`, validates the payload, and returns `202 Accepted` immediately. Each URL is converted into a job and submitted to a worker pool that downloads and uploads images concurrently in the background.

For a detailed explanation of the worker pool design — including goroutines, channels, backpressure, and graceful shutdown — see [docs/worker_pools.md](docs/worker_pools.md).

### Object Paths in Supabase Storage

Each uploaded image is stored under a deterministic path:

```text
{folder}/{index}_{hash}_{name}{extension}
```

- `{folder}`: the `folder` sent in the request, or `uploads` when it is omitted.
- `{index}`: position of the URL in the request.
- `{hash}`: first 8 bytes (16 hex characters) of the SHA-256 of `url + index`. It keeps keys unique, so two different URLs with the same file name never overwrite each other.
- `{name}`: the `name` of that image in `images` when sent, otherwise the file name from the last segment of the URL path, without its extension (query strings are ignored). Known image extensions such as `.jpg` are removed from the name, and unsafe characters are replaced with `_`. Omitted when there is no usable name.
- `{extension}`: taken from the response `Content-Type`, falling back to the URL's extension, then `.bin`.

For example, `https://picsum.photos/id/101/200/300` at index 0 is stored as `uploads/0_fdaad454967159a7_300.jpg` without a folder, or as `directorio/0_fdaad454967159a7_300.jpg` with `"folder": "directorio"`.

The service sends the header `x-upsert: true`, so re-submitting the same URLs overwrites existing objects instead of failing with a duplicate-key error.

---

## Prerequisites

- [Go](https://go.dev/dl/) 1.25+ (matches `go.mod`)
- A Supabase project with:
  - `SUPABASE_URL`
  - `SUPABASE_SERVICE_ROLE_KEY`
  - A Storage bucket already created (e.g., `prueba`)
  - Storage policies that allow writes with the service role key
- (Optional) [Docker](https://www.docker.com/) and Docker Compose for containerized deployment
- (Optional) Python 3 and `requests` for the example client

---

## Local Installation

### 1. Clone the repository

```bash
git clone https://github.com/Alberto-jhen/supabase-insert-helper.git
cd supabase-insert-helper
```

### 2. Create the environment file

```bash
touch .env
```

Edit `.env`:

```bash
SUPABASE_URL=https://<project-ref>.supabase.co
SUPABASE_SERVICE_ROLE_KEY=<your-service-role-key>
PORT=8081
WORKER_COUNT=10
```

`DATABASE_URL` is optional.

### 3. Run locally

```bash
go run ./cmd/server
```

The server will log:

```text
Server listening on port 8081...
```

### 4. Send a test request

```bash
curl -X POST http://localhost:8081/ingest/images \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_name": "prueba",
    "images": [
      {"url": "https://example.com/image1.jpg", "name": "imagen-1"},
      {"url": "https://example.com/image2.jpg"}
    ]
  }'
```

Expected response:

```json
{
  "status": "accepted",
  "bucket": "prueba",
  "count": 2
}
```

---

## Configuration

All configuration is provided through environment variables. `godotenv` reads them from `.env` at startup.

| Variable | Required | Default | Description |
|---|---|---|---|
| `SUPABASE_URL` | Yes | — | Supabase project URL. |
| `SUPABASE_SERVICE_ROLE_KEY` | Yes | — | Service role key for Storage writes. |
| `DATABASE_URL` | No | — | Optional PostgreSQL connection string. |
| `PORT` | No | `8081` | HTTP server port. `8081` avoids collisions with common `8080` backends. |
| `WORKER_COUNT` | No | `10` | Number of concurrent workers that download and upload images. |

### Why port `8081`?

Many local backend services and proxies default to `8080`. Using `8081` reduces the chance of a port conflict when both this microservice and another application run on the same machine.

---

## Running with Docker

### Build and start

```bash
docker compose build
docker compose up -d
```

### Verify logs

```bash
docker compose logs -f supabase-insert-helper
```

### Stop

```bash
docker compose down
```

The Docker image is built in two stages:

1. **Builder stage** (`golang:1.25-alpine`) compiles the binary.
2. **Runtime stage** (`alpine:latest`) runs only the compiled binary and CA certificates, keeping the final image small.

The `.dockerignore` file explicitly excludes `.env`, `.git`, `scripts/`, markdown files, and Docker files themselves so secrets and unnecessary files never enter the image layers.

---

## API Reference

### `POST /ingest/images`

Ingest a batch of public image URLs into a Supabase Storage bucket.

#### Request Headers

```text
Content-Type: application/json
```

#### Request Body

```json
{
  "bucket_name": "string",
  "folder": "string (optional)",
  "images": [
    {"url": "string", "name": "string (optional)"}
  ]
}
```

Example:

```json
{
  "bucket_name": "prueba",
  "folder": "directorio",
  "images": [
    {"url": "https://ejemplo.com/fotos/camisa-roja.jpg", "name": "camisa-roja"}
  ]
}
```

#### Validation Rules

- `bucket_name` is required, non-empty, and must not contain `/`. Use `folder` for subfolders.
- `folder` is optional and defaults to `uploads`. When present, it must be a relative path made of segments with letters, digits, `.`, `_` or `-`, separated by `/`. It cannot start or end with `/`, and cannot contain `.` or `..` segments.
- `images` must contain at least one item.
- Each item needs a non-empty `url`.
- `name` is optional. When it is missing or empty, the file name from the URL is used. Known image extensions such as `.jpg` are removed, because the real extension comes from the file type.

#### Response

| Status | Meaning |
|---|---|
| `202 Accepted` | Payload valid; jobs enqueued for background processing. |
| `400 Bad Request` | Invalid JSON or validation failure. |
| `415 Unsupported Media Type` | `Content-Type` is not `application/json`. |
| `503 Service Unavailable` | The worker pool has been stopped and cannot accept jobs. |

`202 Accepted` body:

```json
{
  "status": "accepted",
  "bucket": "prueba",
  "count": 10
}
```

### `POST /ingest/records`

Placeholder endpoint. It currently responds with a plain text message and will be implemented in a future iteration for PostgreSQL record batch insertion.

---

## Python Client Example

A ready-to-use example is included at `scripts/upload_images.py`. It accepts a bucket name and a list of public image URLs, either from the command line or from a file.

### Install the dependency

```bash
pip install requests
```

### Usage

Send URLs directly:

```bash
python scripts/upload_images.py \
  --bucket prueba \
  https://example.com/1.jpg \
  https://example.com/2.jpg
```

Send URLs from a file:

```bash
python scripts/upload_images.py \
  --bucket prueba \
  --file urls.txt
```

Put the images in a folder inside the bucket:

```bash
python scripts/upload_images.py \
  --bucket prueba \
  --folder directorio \
  https://example.com/1.jpg
```

Give each image its own name (one `--name` per URL, in the same order):

```bash
python scripts/upload_images.py \
  --bucket prueba \
  --folder directorio \
  --name camisa-roja \
  https://example.com/fotos/camisa-roja.jpg
```

Point it to a remote or Dockerized instance:

```bash
python scripts/upload_images.py \
  --url http://localhost:8081/ingest/images \
  --bucket prueba \
  --file urls.txt
```

### Use it from another Python backend

```python
import requests

MICROSERVICE_URL = "http://localhost:8081/ingest/images"

def ingest_images(bucket_name: str, images: list[dict], folder: str = "") -> dict:
    payload = {"bucket_name": bucket_name, "images": images}
    if folder:
        payload["folder"] = folder
    response = requests.post(
        MICROSERVICE_URL,
        json=payload,
        headers={"Content-Type": "application/json"},
        timeout=30,
    )
    response.raise_for_status()
    return response.json()
```

### Important Asynchronous Note

The microservice returns `202 Accepted` immediately. It does **not** wait for uploads to finish. Success or failure of individual uploads is logged by the server; the client only receives confirmation that the batch was accepted.

---

## Tuning the Worker Pool

`WORKER_COUNT` controls how many images can be downloaded and uploaded simultaneously. For a detailed discussion of trade-offs and practical guidelines, see [docs/worker_pools.md](docs/worker_pools.md).

---

## Security Notes

- **Never commit `.env`**. It contains the Supabase service role key.
- **Never copy `.env` into the Docker image**. The `.dockerignore` file prevents this, and `docker-compose.yml` loads `.env` at runtime through `env_file`.
- **Use the service role key only server-side**. Do not expose it to browsers or mobile clients.
- **Keep bucket policies strict**. The service role key bypasses RLS, so the bucket should only be writable by this microservice and readable as needed.

---

## Project Background

This microservice was developed as part of a **final degree project (TFG)** and is intended to be published as open source. The goals are:

- Demonstrate idiomatic Go concurrency (goroutines, channels, `sync.WaitGroup`, `context`).
- Provide a clean, framework-free HTTP API using only the standard library.
- Show how a small backend service can integrate with Supabase Storage at scale without blocking the web server.

---

## License

MIT License — see `LICENSE` for details.
