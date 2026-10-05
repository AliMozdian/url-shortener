package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// url shortener service with http server and in-memory storage
// POST /api/shorten - create a new short URL
// GET /l?id={id} - get the original URL by short ID

const REDIRECT_PATH = "l" // link/

type Store interface {
	Read(short string) (long string, exists bool)
	Write(short, long string) error
}

type RamDatabase struct {
	mu      sync.RWMutex
	storage map[string]string
}

func (db *RamDatabase) Read(id string) (origin string, exists bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	originalURL, exists := db.storage[id]
	return originalURL, exists
}

func (db *RamDatabase) Write(id, origin string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.storage[id] = origin // later change
	return nil              // nothing for now!
}

// request & response body structs

type ShortenReqBody struct {
	Url string `json:"url"`
}

type ShortenRspBody struct {
	Code     string `json:"code"`
	ShortUrl string `json:"short_url"`
}

type Server struct {
	db Store
}

// needs massive refactore
func NewServer() *Server {
	db := &RamDatabase{mu: sync.RWMutex{}, storage: make(map[string]string)}
	return &Server{db: db}
}

func (s *Server) Shorten(originalURL string) string {
	// generate a short ID (simple counter for Phase1)
	id := fmt.Sprintf("%d", 1)      // later we can use a better ID generation method (and safer!)
	_ = s.db.Write(id, originalURL) // error ignored for now
	return id
}

func (s *Server) Redirect(id string) (string, bool) {
	return s.db.Read(id) // originalURL, found/exists
}

func (s *Server) handleShorten(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var newReqBody ShortenReqBody
	err := json.NewDecoder(r.Body).Decode(&newReqBody)
	if err != nil {
		http.Error(w, `{"error": "Invalid JSON format"}`, http.StatusBadRequest)
		return
	}

	if newReqBody.Url == "" {
		http.Error(w, "Missing URL", http.StatusBadRequest)
		return
	}

	id := s.Shorten(newReqBody.Url)
	shortURL := fmt.Sprintf("http://%s/%s?id=%s", r.Host, REDIRECT_PATH, id)
	fmt.Println("set original url:", newReqBody.Url, "to short-form of:", shortURL)
	newRspBody := &ShortenRspBody{Code: id, ShortUrl: shortURL}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newRspBody)
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	originalURL, found := s.Redirect(id)
	if !found {
		http.Error(w, "ID not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
}

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	flag.Parse()
	srv := NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/shorten", srv.handleShorten)
	mux.HandleFunc("/l", srv.handleRedirect)

	server := &http.Server{Addr: ":" + *port, Handler: mux}

	log.Printf("[Server] Listening on :%s\n", *port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("Server error: %w", err)
	}
}
