package store

import (
	"sync"
	"testing"
)

func TestRamDatabase_ReadWrite(t *testing.T) {
	db := NewRam()

	// 1. Read non-existing key
	record, err := db.Read("non-existent")
	if err != nil {
		t.Fatalf("expected err=nil and empty record, got exists=%v, val=%q", err, record.Url)
	}

	// 2. Write key
	err = db.Write(LinkRecord{Code: "abc123", Url: "https://go.dev"})
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// 3. Read back existing key
	record, err = db.Read("abc123")
	if err != nil {
		t.Fatalf("expected key to exist")
	}
	if record.Url != "https://go.dev" {
		t.Fatalf("expected %q, got %q", "https://go.dev", record.Url)
	}

	// 4. Overwrite existing key
	err = db.Write(LinkRecord{Code: "abc123", Url: "https://go.dev/doc"})
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	record, err = db.Read("abc123")
	if err != nil || record.Url != "https://go.dev/doc" {
		t.Fatalf("expected overwritten value %q, got %q", "https://go.dev/doc", record.Url)
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
			_ = db.Write(LinkRecord{Code: "key", Url: "https://go.dev"})
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
