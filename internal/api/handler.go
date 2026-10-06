package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"url-shortener/internal/shortener"
)

// POST /api/shorten - create a new short URL
// GET /{id} - get the original URL by short ID

// request & response body structs
type shortenReqBody struct {
	Url string `json:"url"`
}

type shortenRspBody struct {
	Code     string `json:"code"`
	ShortUrl string `json:"short_url"`
}

type Server struct {
	port       string
	mux        *http.ServeMux
	httpServer *http.Server
	shortener  *shortener.Shortener
}

// creates a new Server (my struct for handling APIs)
func NewServer(port string) (*Server, error) {
	// error handling for port (checkInt, check not empty)
	portAsInt, err := strconv.Atoi(port)
	if err != nil || portAsInt < 0 || portAsInt > 65535 {
		return nil, errors.New("port must be an int, and between 0 and 65535!")
	}

	s := &Server{port: port}
	s.shortener = shortener.New()

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/api/shorten", s.handleShorten)
	s.mux.HandleFunc("GET /{id}", s.handleRedirect)

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

	id, err := s.shortener.Shorten(newReqBody.Url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	shortURL := fmt.Sprintf("http://%s/%s", r.Host, id)
	fmt.Println("set original url:", newReqBody.Url, "to short-form of:", shortURL)
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
	originalURL, found := s.shortener.Redirect(id)
	if !found {
		http.Error(w, "ID not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
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
