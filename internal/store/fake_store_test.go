package store

import (
	"errors"
	"testing"
)

func TestFakeStore_ReadWrite(t *testing.T) {
	store := NewFakeStore()

	record, err := store.Read("non-existent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	err = store.Write(LinkRecord{
		Code: "abc123",
		Url:  "https://go.dev",
	})
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	record, err = store.Read("abc123")
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if record.Url != "https://go.dev" {
		t.Fatalf("expected %q, got %q", "https://go.dev", record.Url)
	}

	err = store.Write(LinkRecord{
		Code: "abc123",
		Url:  "https://go.dev/doc",
	})
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	record, err = store.Read("abc123")
	if err != nil || record.Url != "https://go.dev/doc" {
		t.Fatalf("expected %q, got %q", "https://go.dev/doc", record.Url)
	}
}

func TestFakeStore_Errors(t *testing.T) {
	store := NewFakeStore()
	expectedErr := errors.New("fake store error")

	store.ErrToReturn = expectedErr

	record, err := store.Read("abc123")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if record != (LinkRecord{}) {
		t.Fatalf("expected empty record, got %+v", record)
	}

	err = store.Write(LinkRecord{
		Code: "abc123",
		Url:  "https://go.dev",
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	store.ErrToReturn = nil

	_, err = store.Read("abc123")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
