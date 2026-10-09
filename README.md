# URL Shortener Service

A concurrent, high-performance URL shortener HTTP service built in Go (1.22+) using standard library HTTP routing and clean package isolation (`cmd/server` and `internal/`).

---

## Overview & Architecture

The application is structured into discrete layers under `internal/`:
* `internal/api`: HTTP router (`net/http.ServeMux`), request decoding/validation, JSON encoding, and HTTP status code mappings.
* `internal/shortener`: Business logic, deterministic Base62 hashing, collision management, and URL normalization.
* `internal/store`: Abstraction layer (`Store` interface) supporting both ephemeral in-memory storage (`RamDatabase`), persistent relational storage (`GormDatabase`), and test doubles (`FakeStore`).
* `cmd/server`: CLI entry point, flag parsing (`-addr`, `-base`, `-db`, `-dsn`), and dependency wiring.

---

## Features Implemented (Parts 1–4)

### Part 1 — MVP Shortening & Redirects
* **`POST /api/shorten`**: Generates a 6–8 character URL-safe Base62 code and returns HTTP **`201 Created`** with `code` and `short_url`.
* **Deterministic Idempotency**: Submitting the same normalized URL always yields the identical short code and short URL.
* **`GET /{code}`**: Fast redirect returning HTTP **`302 Found`** with the `Location` header pointing to the long URL.
* **URL Validation & Normalization**:
  * Only `http` and `https` schemes permitted; empty hosts rejected (returns HTTP `400 Bad Request`).
  * No external network requests performed during validation (SSRF-safe).
  * URL casing (scheme and host) and path slashes are normalized so identical web destinations hash consistently.
* **Deterministic Collision Resolution**: Hashes long URLs across lengths 6, 7, and 8 sequentially. If a collision occurs with a distinct URL, it steps up the code length before persisting.

### Part 2 — API Metadata, Sentinel Errors & Store Decoupling
* **`GET /api/v1/links/{code}`**: Link inspection endpoint returning HTTP **`200 OK`** with JSON metadata (`{"url": "...", "created_at": "..."}`) formatted in RFC 3339, or HTTP **`404 Not Found`** for unknown codes.
* **Domain Sentinel Errors**:
  * `store.ErrNotFound`
  * `shortener.ErrInvalidURL`
  * `shortener.ERRCollision`
* **Error Wrapping & Mapping**: Internal errors wrap sentinels using `fmt.Errorf("%w", ...)`; handlers unwrap and map them via `errors.Is` to appropriate HTTP status codes (`400`, `404`, `500`).
* **Decoupled `Store` Interface**: Consumers depend only on the `store.Store` interface, verified with `FakeStore` in unit tests.

### Part 3 — Performance, Timeouts & Profiling
* **Configured Server Timeouts**:
  * `ReadTimeout: 5s` (mitigates Slowloris / hanging read connections).
  * `WriteTimeout: 10s` (mitigates slow client write blocks).
  * `IdleTimeout: 120s` (reclaims inactive keep-alive connections).
* **Concurrency Optimization**: `sync.RWMutex` read locks (`RLock`) allow non-blocking concurrent lookups across redirect and metadata requests.

### Part 4 — Persistence (GORM + SQLite)
* **Durable SQL Storage**: Implemented via GORM with the SQLite driver (`gorm.io/gorm` and `gorm.io/driver/sqlite`).
* **Automated Migrations**: Automatically runs `db.AutoMigrate(&GormLink{})` on startup to ensure tables and indexes are ready.
* **Schema Definition**:
  * Table: `gorm_links`
  * Columns: `code` (string, primary key), `url` (string, indexed), `created_at` (datetime, not null).
* **Persist Before 201**: Database writes execute synchronous `INSERT` transactions via `db.Create(&row)` before `POST /api/shorten` returns HTTP `201 Created`.
* **Restart Survival**: Restarting the process preserves existing mappings and timestamps without data loss.
* **Single Connection Serialization**: Configured with `SetMaxOpenConns(1)` to avoid file lock contention under concurrent SQLite write traffic.

---

## Installation & Prerequisites

* **Go**: Version 1.22 or higher
* **GCC / CGo**: SQLite uses CGo bindings via `gorm.io/driver/sqlite`. Ensure `gcc` is installed on your system.

```bash
git clone https://github.com/AliMozdian/url-shortener.git
cd url-shortener
go mod tidy
```

---

## Running the Application

### 1. Persistent SQLite Mode (Default)
Runs the service with persistent storage in an SQLite file (`links.db`):
```bash
go run ./cmd/server -addr 8080 -base http://localhost:8080 -db sqlite -dsn links.db
```
* If `links.db` does not exist, GORM creates it and auto-migrates tables.
* Data remains preserved across process terminations and restarts.

### 2. In-Memory Mode
Runs the service purely in memory using `RamDatabase`:
```bash
go run ./cmd/server -addr 8080 -base http://localhost:8080 -db ram
```

### CLI Options & Flags
| Flag | Default | Description |
|---|---|---|
| `-addr` | `8080` | Listen port or address for the HTTP server |
| `-base` | `http://localhost:8080` | Base URL used to construct the `short_url` response field |
| `-db` | `sqlite` | Storage engine choice: `sqlite` (persistent) or `ram` (in-memory) |
| `-dsn` | `links.db` | Data Source Name / file path for the SQLite database |

---

## API Documentation & Example Requests

### 1. Create Short Link
```bash
curl -s -X POST http://localhost:8080/api/shorten \
  -H "Content-Type: application/json" \
  -d '{"url": "https://go.dev/doc/"}'
```
**Response (`201 Created`):**
```json
{
  "code": "dIzT6t",
  "short_url": "http://localhost:8080/dIzT6t"
}
```

### 2. Follow Redirect
```bash
curl -sI http://localhost:8080/dIzT6t
```
**Response (`302 Found`):**
```http
HTTP/1.1 302 Found
Location: https://go.dev/doc
Date: Sat, 10 Oct 2026 02:20:00 GMT
```

Follow redirect directly:
```bash
curl -L http://localhost:8080/dIzT6t
```

### 3. Retrieve Link Metadata
```bash
curl -s http://localhost:8080/api/v1/links/dIzT6t
```
**Response (`200 OK`):**
```json
{
  "url": "https://go.dev/doc",
  "created_at": "2026-10-08T23:25:52Z"
}
```

---

## Testing & Quality Assurance

### 1. Run Complete Test Suite
```bash
go test ./...
```

### 2. Concurrency & Race Detector (`-race`)
Verifies that concurrent reads and writes across both in-memory and GORM backends are free of data races:
```bash
go test -race ./...
```

### 3. Persistence & Restart Simulation Test
The test `TestGormDatabase_RestartSimulation` in `internal/store/gorm_test.go` verifies durability across process restarts:
1. Creates a temporary SQLite database file via `t.TempDir()`.
2. Connects using `instanceA`, writes records, and closes the connection cleanly.
3. Initializes a new store `instanceB` pointing to the exact same file.
4. Verifies that `instanceB` can read all previously inserted records with identical codes, URLs, and timestamps.

Run persistence tests directly:
```bash
go test -v ./internal/store -run TestGormDatabase_RestartSimulation
```

### 4. Statement Coverage
Statements coverage across all packages exceeds the mandatory 70% threshold:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -n1
```
**Result:**
```text
total: (statements)    88.4%
```

---

## Benchmarks & Profiling (Part 3)

### Benchmark Execution
Benchmarks measure throughput and allocations for both write and read operations:
```bash
go test -bench=. -benchmem -benchtime=2s ./internal/api/
```

**Benchmark Results:**
```text
BenchmarkHandleShorten-16     233398    8853 ns/op    7475 B/op    41 allocs/op
BenchmarkHandleRedirect-16    391255    5694 ns/op    6383 B/op    23 allocs/op
```

### Profiling Analysis
Captured via `go tool pprof -top cpu.prof` over 7.27 seconds of total CPU sampling:
* **Redirect Path (`GET /{code}`)**:
  * Handled via `BenchmarkHandleRedirect` at ~5.6 μs/op.
  * CPU time is largely spent in `net/http` route evaluation and HTTP test recorder allocations.
  * In-memory storage reads under `sync.RWMutex.RLock` consume less than 1% flat CPU time with zero lock contention.
* **Shorten Path (`POST /api/shorten`)**:
  * Handled via `BenchmarkHandleShorten` at ~8.8 μs/op.
  * Time is spent in request payload deserialization (`encoding/json`) and URL URI parsing/validation (`net/url.ParseRequestURI`).
  * Base62 FNV-64 hashing (`hashToN`) accounts for ~2.75% of cumulative CPU time, verifying that the deterministic mathematical mapping introduces negligible overhead.
