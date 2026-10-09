package store

import (
	"fmt"
	"sync"
)

const DEFAULT_CACHE_CAP int = 10000

// decorates any underlying Store with an in-memory read-through cache.
// infact it intercepts Read calls to serve hot links in O(1) time and bypasses disk IO (good for high-trafic read calls)
type CachedStore struct {
	underlying Store
	mu         sync.RWMutex
	cache      map[string]LinkRecord
	capacity   int
}

// wraps an existing store with in-memory caching capacity
// capacity<=0 => capacity = DEFAULT_CACHE_CAP (10K)
func NewCachedStore(underlying Store, capacity int) *CachedStore {
	if capacity <= 0 {
		capacity = DEFAULT_CACHE_CAP
	}
	return &CachedStore{
		underlying: underlying,
		cache:      make(map[string]LinkRecord, capacity),
		capacity:   capacity,
	}
}

// it checks cache, if hit return val, else (miss) calls underlying store Read
func (c *CachedStore) Read(code string) (LinkRecord, error) {
	c.mu.RLock()
	rec, exists := c.cache[code]
	c.mu.RUnlock()
	if exists {
		return rec, nil
	}

	// cache miss: fall back to persistent/underlying store
	rec, err := c.underlying.Read(code)
	if err != nil {
		return LinkRecord{}, fmt.Errorf("cache miss, error while underlying store read: %w", err)
	}

	// populate cache (we want a fresh cache for next cache hit)
	c.mu.Lock()
	if len(c.cache) < c.capacity {
		// checks it so we don't accidently extend the capacity of cache (overhead of slices)
		c.cache[code] = rec
	}
	c.mu.Unlock()

	return rec, nil
}

// Write persists to the underlying store first, then updates the cache.
func (c *CachedStore) Write(record LinkRecord) error {
	// idea for later:	lazy cache, doesn't write on store, holds in cache
	// 					and some worker will write it on IO in more idle times

	if err := c.underlying.Write(record); err != nil {
		return fmt.Errorf("using cache, error while underlyting store write: %w", err)
	}

	c.mu.Lock()
	if len(c.cache) < c.capacity {
		c.cache[record.Code] = record
	}
	c.mu.Unlock()

	return nil
}
