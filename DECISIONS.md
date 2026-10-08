# Design decisions

## Dependencies
- Standard library only (`net/http`, `sync`, `hash/fnv`, `math/big`, `flag`, `time`, `net/url`, `strconv`, etc.). No external frameworks or routers used for Parts 1 & 2.
## Part 1
- **Code generation**: We use a keyed 64-bit FNV hash (`hash/fnv`) over the long URL and a fixed server key. The 64-bit integer output is mapped to a Base62 string using big integer arithmetic (`math/big`), producing URL-safe strings of length 6 to 8 padded with leading zeros when necessary.
- **How same URL → same code is stored**: We avoid maintaining a secondary reverse map (`longURL -> code`). Because the hash function is deterministic, calculating the code for the same URL will point to the identical bucket in storage. If the stored URL matches the incoming URL, it returns the existing code immediately (idempotency).
- **Collision handling**: When `hashToN(URL, 6)` points to an existing entry with a *different* URL, the algorithm steps up code length to 7 characters (`hashToN(URL, 7)`), and subsequently to 8 characters. If collisions occur across lengths 6, 7, and 8, an error is returned.
- **URL normalization**: URLs are currently matched as provided in request bodies. (Note: standardizing scheme/host casing and trimming trailing slashes can be applied prior to hash invocation).
- **Mutex type and locking**: `sync.RWMutex` in `RamDatabase`. Read operations (`Read`) acquire `RLock()` to allow concurrent reads on redirect; write operations (`Write`) acquire full write lock (`Lock()`).
- **Package layout**:
  - `cmd/server/main.go`: Entrypoint for CLI flag parsing and process bootstrapping.
  - `internal/store`: In-memory storage abstraction and concurrency controls.
  - `internal/shortener`: Hash-based Base62 generation, idempotency resolution, and collision management.
  - `internal/api`: HTTP handlers, routing, status code mappings, and JSON serialization.


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
