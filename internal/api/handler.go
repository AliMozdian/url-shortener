package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AliMozdian/url-shortener/internal/store"
)

// POST /api/shorten - create a new short URL
// GET /{id} - get the original URL by short ID

// Validate: http or https only, no empty host
// Normalize: lower-case scheme and host; strip trailing slash if path != "/"
func ValidateAndNormalizeURL(rawURL string) (string, error) {
	// this function might be anti-pattern, but I'm not sure about it!
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("Error while Parsing URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("scheme must be http or https")
	}
	if u.Host == "" {
		return "", errors.New("host cannot be empty")
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if u.Path == "" || u.Path == "/" {
		u.Path = "/"
	} else {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String(), nil
}

// request & response body structs
type shortenReqBody struct {
	Url string `json:"url"`
}

type shortenRspBody struct {
	Code     string `json:"code"`
	ShortUrl string `json:"short_url"`
}

type metadataRspBody struct {
	Url       string `json:"url"`
	CreatedAt string `json:"created_at"` // RFC3339 format is implied
}

func (s *Server) handleShorten(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var newReqBody shortenReqBody
	err := json.NewDecoder(r.Body).Decode(&newReqBody)
	if err != nil {
		http.Error(w, `{"error": "Invalid JSON format"}`, http.StatusBadRequest)
		return
	}

	if newReqBody.Url == "" {
		http.Error(w, "Missing URL", http.StatusBadRequest)
		return
	}

	normalizedURL, err := ValidateAndNormalizeURL(newReqBody.Url)
	if err != nil {
		errMsg := fmt.Sprintf("Error in URL Validation: %v", err)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	id, err := s.shortener.Shorten(normalizedURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	shortURL := fmt.Sprintf("%s/%s", s.base, id)
	fmt.Println("original url:", normalizedURL, "short-form of:", shortURL)
	newRspBody := &shortenRspBody{Code: id, ShortUrl: shortURL}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newRspBody)
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	originalURL, err := s.shortener.Redirect(id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "ID not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Unexpected Server Error!", http.StatusInternalServerError)
	}
	http.Redirect(w, r, originalURL, http.StatusFound)
}

func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")

	record, err := s.shortener.GetMetadata(id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, `{"error": "link not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	rsp := metadataRspBody{
		Url:       record.Url,
		CreatedAt: record.CreatedAt.Format(time.RFC3339),
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rsp) // we're sure there is no error for this :0
}
