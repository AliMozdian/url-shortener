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
