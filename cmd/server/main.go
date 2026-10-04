package main

import (
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
	storage map[string]string // in-memory storage for now
	Store
}

func (db *RamDatabase) Read(id string) (origin string, exists bool) {
	db.mu.RLock()
	originalURL, exists := db.storage[id]
	db.mu.RUnlock()
	return originalURL, exists
}

func (db *RamDatabase) Save(id, origin string) error {
	db.mu.Lock()
	db.storage[id] = origin // later change
	db.mu.Unlock()
	return nil // nothing for now!
}

type MapAlgorithm interface {
	Shorten(origin string) (id string)
}

// for now, includes URL generator
type Server struct {
	db *RamDatabase // later: Store
	MapAlgorithm
}

// needs massive refactore
func NewServer() *Server {
	db := &RamDatabase{mu: sync.RWMutex{}, storage: make(map[string]string)}
	return &Server{db: db}
}

func (s *Server) Shorten(originalURL string) string {
	// generate a short ID (for simplicity, use the length of the storage)
	id := fmt.Sprintf("%d", len(s.db.storage)+1) // later we can use a better ID generation method (and safer!)
	// problem with current id is that it is not idempotent! the same long-url doesn't map to the same short-url
	s.db.Write(id, originalURL) // error ignored for now
	return id
}

func (s *Server) Redirect(id string) (string, bool) {
	return s.db.Read(id) // originalURL, found/exists
}

func (s *Server) handleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	originalURL := r.FormValue("url")
	if originalURL == "" {
		http.Error(w, "Missing URL", http.StatusBadRequest)
		return
	}

	id := s.Shorten(originalURL)
	shortURL := fmt.Sprintf("http://%s/%s/id=%s", r.Host, id)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
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
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal("Server error: %w", err)
	}
}
