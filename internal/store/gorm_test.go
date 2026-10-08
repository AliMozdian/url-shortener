package store

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestGormDatabase_RestartSimulation(t *testing.T) {
	// Create a temp directory automatically cleaned up after test completion
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "restart_test.db")

	testRecord := LinkRecord{
		Code:      "go1234",
		Url:       "https://go.dev/doc",
		CreatedAt: time.Now().UTC().Truncate(time.Second), // match SQLite second precision
	}

	// instance A opens the DB, writes data, and closes (simulating process run & shutdown)
	instanceA, err := NewGormStore(dbPath)
	if err != nil {
		t.Fatalf("instanceA failed to open store: %v", err)
	}

	if err := instanceA.Write(testRecord); err != nil {
		t.Fatalf("instanceA failed to write link: %v", err)
	}

	if err := instanceA.Close(); err != nil {
		t.Fatalf("instanceA failed to close cleanly: %v", err)
	}

	// instance B opens the SAME file (simulating server restart)
	instanceB, err := NewGormStore(dbPath)
	if err != nil {
		t.Fatalf("instanceB failed to open store after restart: %v", err)
	}
	defer instanceB.Close()

	// instance B reads what Instance A persisted
	loadedRecord, err := instanceB.Read(testRecord.Code)
	if err != nil {
		t.Fatalf("instanceB failed to read persisted record: %v", err)
	}

	if loadedRecord.Code != testRecord.Code {
		t.Errorf("expected code %s, got %s", testRecord.Code, loadedRecord.Code)
	}
	if loadedRecord.Url != testRecord.Url {
		t.Errorf("expected url %s, got %s", testRecord.Url, loadedRecord.Url)
	}
	if !loadedRecord.CreatedAt.Equal(testRecord.CreatedAt) {
		t.Errorf("expected created_at %v, got %v", testRecord.CreatedAt, loadedRecord.CreatedAt)
	}

	// test missing key on restarted DB
	_, err = instanceB.Read("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing code, got %v", err)
	}
}

func TestGormDatabase_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "concurrent_test.db")

	store, err := NewGormStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open gorm store: %v", err)
	}
	defer store.Close()

	// Pre-populate one record
	initialRec := LinkRecord{
		Code:      "root01",
		Url:       "https://go.dev",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Write(initialRec); err != nil {
		t.Fatalf("failed to insert initial record: %v", err)
	}

	var wg sync.WaitGroup
	workers := 20

	// Concurrent writes
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec := LinkRecord{
				Code:      time.Now().Format("150405.000000") + string(rune('a'+idx)),
				Url:       "https://example.com",
				CreatedAt: time.Now().UTC(),
			}
			_ = store.Write(rec)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.Read("root01")
		}()
	}

	wg.Wait()
}
