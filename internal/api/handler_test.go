package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	// Using a dummy port and base URL for test setup
	srv, err := NewServer("http://localhost:8080", "8080")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	return srv
}

func TestHandleShorten_SuccessAndIdempotency(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"url": "https://go.dev/doc"}`

	// First request: 201 Created
	req1 := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()

	srv.mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w1.Code, w1.Body.String())
	}

	var res1 shortenRspBody
	if err := json.Unmarshal(w1.Body.Bytes(), &res1); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res1.Code == "" || res1.ShortUrl == "" {
		t.Fatalf("expected non-empty code and short_url, got %+v", res1)
	}

	// Second request with same URL: must return 201 with identical code
	req2 := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	srv.mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w2.Code)
	}

	var res2 shortenRspBody
	_ = json.Unmarshal(w2.Body.Bytes(), &res2)

	if res1.Code != res2.Code {
		t.Errorf("idempotency broken: got code %s first, then %s", res1.Code, res2.Code)
	}
}

func TestHandleRedirect_Success(t *testing.T) {
	srv := setupTestServer(t)

	// Pre-populate via Shorten handler
	body := `{"url": "https://go.dev/doc"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var res shortenRspBody
	_ = json.Unmarshal(w.Body.Bytes(), &res)

	// Request redirect: GET /{code}
	redirectReq := httptest.NewRequest(http.MethodGet, "/"+res.Code, nil)
	redirectW := httptest.NewRecorder()

	srv.mux.ServeHTTP(redirectW, redirectReq)

	if redirectW.Code != http.StatusFound {
		t.Errorf("expected 302 Found, got %d", redirectW.Code)
	}

	loc := redirectW.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://go.dev/doc") {
		t.Errorf("expected Location header to point to target URL, got %q", loc)
	}
}

func TestTable_BadRequestsAndNotFound(t *testing.T) {
	srv := setupTestServer(t)

	tests := []struct {
		name           string
		method         string
		target         string
		body           string
		expectedStatus int
	}{
		{
			name:           "empty url in body",
			method:         http.MethodPost,
			target:         "/api/shorten",
			body:           `{"url": ""}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "non-http/https scheme",
			method:         http.MethodPost,
			target:         "/api/shorten",
			body:           `{"url": "ftp://files.example.com"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid json format",
			method:         http.MethodPost,
			target:         "/api/shorten",
			body:           `{"url": http://broken}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "method not allowed on shorten",
			method:         http.MethodGet,
			target:         "/api/shorten",
			body:           "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "unknown redirect code",
			method:         http.MethodGet,
			target:         "/unknownCode",
			body:           "",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, tc.target, bytes.NewBufferString(tc.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.target, nil)
			}

			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("[%s] expected status %d, got %d", tc.name, tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandleShorten_Concurrent(t *testing.T) {
	srv := setupTestServer(t)

	var wg sync.WaitGroup
	workers := 50
	payload := `{"url": "https://go.dev/play"}`

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("concurrent request returned status %d", w.Code)
			}
		}()
	}
	wg.Wait()
}
