# Design Decisions

## Dependencies
- Standard library only (`net/http`, `sync`, `hash/fnv`, `math/big`, `flag`, `time`, `net/url`, `strconv`, etc.). No external frameworks or routers used for Parts 1 & 2.

## Part 1
- **Code generation**: Keyed 64-bit FNV hash (`hash/fnv`) over the normalized URL with an application key, mapped into Base62 characters using `math/big` integer modulo arithmetic. Output codes are padded with leading zeros to guarantee exact lengths between 6 and 8.
- **How same URL → same code is stored**: The hash algorithm is deterministic. Re-hashing the same normalized URL always yields the identical candidate code bucket. If an entry exists and its stored URL matches the input URL, the existing code is returned directly without creating duplicate entries or requiring a secondary reverse index map (`url -> code`).
- **Collision handling**: Length-based probing across lengths 6, 7, and 8. When `hashToN(URL, 6)` points to an existing entry containing a different URL (a collision), the shortener steps up to length 7 (`hashToN(URL, 7)`), and then length 8. If all lengths collide with distinct URLs, a wrapped `ERRCollision` sentinel error is returned.
- **URL normalization**: Evaluated via standard library `net/url.ParseRequestURI`. Scheme and host are lowercased. Empty paths and single root paths (`/`) are normalized consistently, and trailing slashes on subpaths are trimmed so URLs pointing to identical targets produce identical hashes.
- **Mutex type and locking**: `sync.RWMutex` on in-memory storage. Read paths (`Read` for redirect and metadata) acquire `RLock` for concurrent access. Write paths (`Write`) acquire exclusive `Lock`.
- **Package layout**:
  - `cmd/server`: CLI entrypoint, flag parsing (`-addr`, `-base`, `-db`), and service startup.
  - `internal/store`: `Store` interface, `LinkRecord` entity, sentinel `ErrNotFound`, and thread-safe `RamDatabase` alongside test doubles (`FakeStore`).
  - `internal/shortener`: URL generation, Base62 conversion, collision resolution, idempotency checks, and sentinel `ErrInvalidURL`.
  - `internal/api`: HTTP router (`net/http.ServeMux`), request decoding/validation, JSON serialization, and error mapping via `errors.Is`.

## Part 2
- **`Store` interface location and methods**:
  - Defined in `internal/store` and accepted by `internal/shortener` via dependency injection (`shortener.New(db store.Store)`).
  - Methods:
    - `Read(code string) (LinkRecord, error)`
    - `Write(record LinkRecord) error`
  - Stored model: `LinkRecord` holding `Code`, `Url`, and `CreatedAt` (`time.Time`).
- **Sentinel errors and HTTP mapping**:
  - Sentinels defined: `store.ErrNotFound` and `shortener.ErrInvalidURL`.
  - Domain and store layers wrap underlying errors using `fmt.Errorf("...: %w", err)`.
  - Handlers unwrap errors using `errors.Is`:
    - `errors.Is(err, store.ErrNotFound)` maps to HTTP `404 Not Found`.
    - Request validation failures or `shortener.ErrInvalidURL` map to HTTP `400 Bad Request`.
    - Unknown or unexpected storage errors map to HTTP `500 Internal Server Error`.
- **Testing with FakeStore**:
  - `store.FakeStore` implements `store.Store` with a configurable `ErrToReturn` field.
  - Injected in HTTP unit tests via `api.NewServerWithShortner` to verify both missing key paths (`404`) and storage failure paths (`500`) without depending on `RamDatabase`.
- **Idempotency retention**:
  - Calling `POST /api/shorten` on an existing URL returns HTTP `201 Created` with the original code and preserves the initial `CreatedAt` timestamp in storage.

## Part 3
- **Locking Choice**:
  - `sync.RWMutex` is selected over `sync.Mutex` for `RamDatabase`. In production URL shortener traffic, reads (redirects via `GET /{id}` and metadata lookups via `GET /api/v1/links/{id}`) heavily exceed writes (`POST /api/shorten`). Profiling confirmed that under high concurrency, read operations via `RLock()` execute with zero lock contention, accounting for less than 1% flat CPU time.
- **Timeout Values & Slow-Client Mitigation**:
  - `ReadTimeout: 5s`: Limits the duration allowed to read client headers and bodies, defending against Slowloris attacks.
  - `WriteTimeout: 10s`: Limits response writing duration, closing idle or stalled sockets before server resources degrade.
  - `IdleTimeout: 120s`: Reclaims inactive keep-alive connections after two minutes of inactivity.
- **Eviction, Capacity & Collision Analysis**:
  - In-memory storage is left unbounded for Part 3 to preserve deterministic idempotency and the length-stepping collision chain (6 → 7 → 8).
  - **Capacity Spaces ($M_k = 62^k$)**:
    - Length 6 ($M_6$): $62^6 \approx 56.80 \times 10^9$ possible codes.
    - Length 7 ($M_7$): $62^7 \approx 3.52 \times 10^{12}$ possible codes.
    - Length 8 ($M_8$): $62^8 \approx 218.34 \times 10^{12}$ possible codes.
  - **Collision Expectations**:
    - *First Global Collision*: Under the Birthday Paradox ($N_{50\%} \approx \sqrt{2 M_6 \ln(2)}$), the first length-6 collision is expected after approximately 280,600 unique URLs are inserted.
    - *Length 7 Migration*: When a collision occurs at length 6, the insertion probe moves to length 7. At a scale of $N = 10,000,000$ links, the probability of an individual insert colliding at length 6 is only $P_6 = \frac{10^7}{5.68 \times 10^{10}} \approx 0.0176\%$ (1 in 5,680).
    - *Length 8 Collision & Failure Ceiling*: Returning an error requires a URL to collide consecutively at length 6, length 7, and length 8. Since $M_7$ and $M_8$ expand exponentially, the compound probability of failure $P(\text{ERRCollision}) = P_6 \times P_7 \times P_8$ at 10 million links is less than $10^{-20}$, guaranteeing virtually unlimited collision resilience before persistent storage limits are reached in Part 4.


## Part 4
- **Storage Choice**:
  - GORM with SQLite driver (`gorm.io/driver/sqlite`). SQLite provides zero-infrastructure, self-contained single-file durability that runs across environments without requiring external daemon setup.
- **Schema, Models & Migrations**:
  - Model `GormLink` maps to table `gorm_links` with columns `code` (string, primary key), `url` (string, indexed), and `created_at` (timestamp, not null).
  - Schema migrations run automatically upon connection startup via `db.AutoMigrate(&GormLink{})`.
- **Atomicity & Crash Safety (Persist Before 201)**:
  - Database writes use `db.Create(&row)` which executes an immediate synchronous SQL INSERT within SQLite's ACID transaction boundaries. The handler returns HTTP 201 Created only after `Write` completes without error.
  - SQLite connection pool is configured with `SetMaxOpenConns(1)` to serialize write operations at the connection level, preventing file-level write lock thrashing under concurrent access.
- **Timestamp Storage**:
  - `CreatedAt` is stored as UTC timestamp. In tests and migrations, comparisons use second-level truncation (`Truncate(time.Second)`) to maintain fidelity across SQLite DATETIME conversions.
- **Persistence & Idempotency Across Restarts**:
  - Because code generation is derived deterministically from the URL hash, submitting the same URL after a server restart generates the exact same candidate code. The query `Read(code)` locates the existing record in SQLite, returning the original code and preserving the initial `CreatedAt` timestamp.


## Part 5 — Millions of Requests (Architecture & Scaling)

### 1. Stateless App Replicas & Shared Persistence (LB → N Apps → Shared Store)
- **Stateless Tier**:
  - The Go web application instances retain zero local state. All configuration (listen address, external base URL, database connection string, secret hash key) is passed via CLI flags or environment variables.
  - Instances run inside containers (e.g., Kubernetes Pods or Amazon ECS tasks) horizontally auto-scaled based on CPU utilization and request rate.
- **Layer 7 Load Balancer**:
  - Traffic enters through a reverse proxy / Layer 7 load balancer (e.g., AWS Application Load Balancer, Cloudflare, or NGINX).
  - The load balancer performs TLS termination, connection reuse via HTTP keep-alives, and health checks (`GET /healthz`), distributing requests round-robin across active application replicas.
- **Shared Persistence Tier**:
  - The single-file SQLite database is replaced with a managed, distributed relational cluster (e.g., PostgreSQL / Amazon Aurora) or a distributed key-value store.
  - Replicas connect to the shared database pool via pgBouncer or built-in Go `database/sql` connection pooling, avoiding connection exhaustion while scaling to tens of instances.

### 2. Read Path & Edge/CDN Caching for HTTP 302s
- **Edge Cache Strategy**:
  - The redirect endpoint (`GET /{code}`) returns HTTP `302 Found` with an explicit cache directive:
    ```http
    HTTP/1.1 302 Found
    Location: [https://example.com/target](https://example.com/target)
    Cache-Control: public, max-age=86400, s-maxage=604800, stale-while-revalidate=3600
    ```
  - `s-maxage=604800` instructs CDN edge nodes (Cloudflare, Fastly, AWS CloudFront) to cache the redirect response for 7 days.
  - When millions of users visit a viral short link, 99.9% of redirect requests are terminated at the CDN point of presence (PoP) nearest to the user, never reaching the Go application servers or database.
- **Trade-offs & Stale Redirects**:
  - *Trade-off (Immutability vs Invalidation)*: Short links are designed to be immutable once created. If a URL target is ever modified or deleted, CDN caches will serve the stale redirect until TTL expiration unless an explicit CDN cache purge API call (`POST /purge-cache`) is triggered.
  - *Why not 301 Moved Permanently?*: HTTP 301 is aggressively and permanently cached by client browsers with no guaranteed mechanism for the server to invalidate it. HTTP 302 with CDN-targeted `s-maxage` keeps cache control in the hands of edge proxies rather than client browser caches.

### 3. Write-Path Scaling
To handle spikes in `POST /api/shorten` without overwhelming the database write throughput:
- **Pre-Generated Short Code Pools (Token Vending Machine)**:
  - Instead of dynamically hashing and performing multiple database collision checks under write bursts, a background worker pre-generates blocks of unique Base62 codes (e.g., chunks of 10,000 codes allocated to each application replica).
  - Each app instance consumes from its assigned local memory buffer when creating links, turning code allocation into an in-memory operation ($O(1)$) with zero collision checks on insert.
- **Asynchronous Ingestion via Message Queue**:
  - For massive batch URL creation, the API endpoint publishes the creation payload to an event bus (e.g., Apache Kafka or RabbitMQ) and immediately returns an assigned code.
  - A pool of asynchronous background consumers flushes records in batched SQL inserts (`INSERT INTO ... VALUES (...), (...)`) directly into storage, smoothing out traffic spikes and maximizing database IOPS efficiency.
- **Token Bucket Rate Limiting**:
  - Token-bucket rate limiting per IP address or API token prevents single clients from exhausting available link code spaces or flooding write queues.

### 4. Database Sharding & Partitioning Strategy
When dataset size or write throughput surpasses a single database instance's storage or IOPS limits:
- **Consistent Hashing by Short Code**:
  - Partition the data across $N$ database shards using consistent hashing on the short code (e.g., `MurmurHash3(code) % NumShards`).
  - Read lookups (`GET /{code}`) route deterministically to the exact shard responsible for that code key in $O(1)$ time without scatter-gather queries.
- **Base62 Range Partitioning**:
  - Alternatively, partition by the leading character of the Base62 code (`[0-9a-zA-Z]`, 62 partitions).
  - Each shard owns a specific character prefix bucket (e.g., Shard 1 handles codes starting with `0-9`, Shard 2 handles `a-z`, Shard 3 handles `A-Z`).
- **Idempotency Across Shards**:
  - Because URL normalization and hash computation are deterministic, generating the short code first allows the application router to identify the target shard *before* issuing either the write or lookup query.

### 5. Bonus Code Implementation: Read-Through Cache Layer
- Implemented `CachedStore` in `internal/store/cache.go`, wrapping any persistent store (such as `GormDatabase`) with an in-memory cache.
- Read operations (`Read`) inspect the cache first in $O(1)$ time, eliminating database round-trips for hot links.
- Write operations (`Write`) update persistent storage first and populate the cache atomically upon successful write.
- Validated with unit and concurrent race tests in `internal/store/cache_test.go`.

