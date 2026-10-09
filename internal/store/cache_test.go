package store

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCachedStore_ReadThroughAndWrite(t *testing.T) {
	fake := NewFakeStore()
	cached := NewCachedStore(fake, 100)

	rec := LinkRecord{
		Code:      "cached1",
		Url:       "https://go.dev",
		CreatedAt: time.Now().UTC(),
	}

	// 1. Write through cache
	if err := cached.Write(rec); err != nil {
		t.Fatalf("failed to write through cache: %v", err)
	}

	// Check underlying fake has it
	if _, err := fake.Read("cached1"); err != nil {
		t.Fatalf("expected underlying store to have written record: %v", err)
	}

	// 2. Read from cache
	got, err := cached.Read("cached1")
	if err != nil {
		t.Fatalf("failed to read from cache: %v", err)
	}
	if got.Url != rec.Url {
		t.Errorf("expected %s, got %s", rec.Url, got.Url)
	}

	// 3. Test Cache Miss with fallback to underlying
	rec2 := LinkRecord{Code: "direct2", Url: "https://example.com", CreatedAt: time.Now().UTC()}
	_ = fake.Write(rec2) // written directly to underlying, not yet in cache

	got2, err := cached.Read("direct2")
	if err != nil {
		t.Fatalf("expected cache miss to read from underlying: %v", err)
	}
	if got2.Url != rec2.Url {
		t.Errorf("expected %s, got %s", rec2.Url, got2.Url)
	}

	// 4. Missing key propagates ErrNotFound
	_, err = cached.Read("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCachedStore_ConcurrentAccess(t *testing.T) {
	ram := NewRam() // fake doesn't work because it doesn't have mutex and doesn't handle concurrency
	cached := NewCachedStore(ram, 500)

	var wg sync.WaitGroup
	workers := 30

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			code := "code" + string(rune('a'+idx))
			_ = cached.Write(LinkRecord{Code: code, Url: "https://go.dev", CreatedAt: time.Now().UTC()})
			_, _ = cached.Read(code)
		}(i)
	}

	wg.Wait()
}
