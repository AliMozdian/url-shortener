package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AliMozdian/url-shortener/internal/shortener"
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

type Server struct {
	base       string
	port       string
	mux        *http.ServeMux
	httpServer *http.Server
	shortener  *shortener.Shortener
}

// creates a new Server (my struct for handling APIs)
func NewServer(base, port string) (*Server, error) {
	// error handling for port (checkInt, check not empty)
	port = strings.TrimPrefix(port, ":")
	portAsInt, err := strconv.Atoi(port)
	if err != nil || portAsInt < 0 || portAsInt > 65535 {
		return nil, fmt.Errorf("port must be an int, and between 0 and 65535!, not %q", port)
	}

	s := &Server{base: base, port: port}
	s.shortener = shortener.New(store.NewRam()) // for now we have only in-memory store (db)

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/api/shorten", s.handleShorten)
	s.mux.HandleFunc("GET /{id}", s.handleRedirect)
	s.mux.HandleFunc("GET /api/v1/links/{id}", s.handleMetadata)

	s.httpServer = &http.Server{Addr: ":" + port, Handler: s.mux}
	return s, nil
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

// Runes the server and loops on listenning until something kills it
// it needs a shutdown button/command for safer shutting down (saving db, etc)
func (s *Server) Run() {
	log.Printf("[Server] Listening on :%s\n", s.port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("Server error: %w", err)
	}
	log.Println("Server shutting down without any problem :)")
}
