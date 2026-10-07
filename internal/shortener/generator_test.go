package shortener

import (
	"sync"
	"testing"
)

func TestShortener_Idempotency(t *testing.T) {
	s := New()
	targetURL := "https://go.dev/doc"

	id1, err := s.Shorten(targetURL)
	if err != nil {
		t.Fatalf("unexpected error on first shorten: %v", err)
	}

	id2, err := s.Shorten(targetURL)
	if err != nil {
		t.Fatalf("unexpected error on second shorten: %v", err)
	}

	if id1 != id2 {
		t.Errorf("expected same code for same URL, got %s and %s", id1, id2)
	}

	if len(id1) < 6 || len(id1) > 8 {
		t.Errorf("expected code length between 6 and 8, got %d", len(id1))
	}
}

func TestShortener_Redirect(t *testing.T) {
	s := New()
	targetURL := "https://example.com"

	id, err := s.Shorten(targetURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotURL, found := s.Redirect(id)
	if !found {
		t.Fatalf("expected code %s to be found in store", id)
	}
	if gotURL != targetURL {
		t.Errorf("expected %s, got %s", targetURL, gotURL)
	}

	_, found = s.Redirect("nonexistent")
	if found {
		t.Errorf("expected nonexistent code to return found=false")
	}
}

func TestShortener_ConcurrentAccess(t *testing.T) {
	s := New()
	urls := []string{
		"https://google.com",
		"https://go.dev",
		"https://github.com",
		"https://quera.org",
	}

	var wg sync.WaitGroup
	// Run 40 concurrent workers shortening URLs
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			url := urls[idx%len(urls)]
			id, err := s.Shorten(url)
			if err != nil {
				t.Errorf("concurrent shorten failed: %v", err)
				return
			}
			_, found := s.Redirect(id)
			if !found {
				t.Errorf("concurrent redirect lookup failed for id: %s", id)
			}
		}(i)
	}
	wg.Wait()
}
