package store

import (
	"sync"
	"testing"
)

func TestRamDatabase_ReadWrite(t *testing.T) {
	db := NewRam()

	// 1. Read non-existing key
	val, exists := db.Read("non-existent")
	if exists || val != "" {
		t.Fatalf("expected exists=false and empty string, got exists=%v, val=%q", exists, val)
	}

	// 2. Write key
	err := db.Write("abc123", "https://go.dev")
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// 3. Read back existing key
	val, exists = db.Read("abc123")
	if !exists {
		t.Fatalf("expected key to exist")
	}
	if val != "https://go.dev" {
		t.Fatalf("expected %q, got %q", "https://go.dev", val)
	}

	// 4. Overwrite existing key
	err = db.Write("abc123", "https://go.dev/doc")
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	val, exists = db.Read("abc123")
	if !exists || val != "https://go.dev/doc" {
		t.Fatalf("expected overwritten value %q, got %q", "https://go.dev/doc", val)
	}
}

func TestRamDatabase_ConcurrentReadWrite(t *testing.T) {
	db := NewRam()
	var wg sync.WaitGroup

	// Concurrently write
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = db.Write("key", "https://go.dev")
		}(i)
	}

	// Concurrently read
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = db.Read("key")
		}()
	}

	wg.Wait()
}
