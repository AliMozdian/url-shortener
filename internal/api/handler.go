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
// GET /l?id={id} - get the original URL by short ID

const REDIRECT_PATH = "l" // link/	--> needs to be removed and use GET/{code} instead

// request & response body structs

type ShortenReqBody struct {
	Url string `json:"url"`
}

type ShortenRspBody struct {
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
	s.mux.HandleFunc("/api/shorten", s.HandleShorten)
	s.mux.HandleFunc("/l", s.HandleRedirect)

	s.httpServer = &http.Server{Addr: ":" + port, Handler: s.mux}
	return s, nil
}

func (s *Server) HandleShorten(w http.ResponseWriter, r *http.Request) {
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

	id := s.shortener.Shorten(newReqBody.Url)
	shortURL := fmt.Sprintf("http://%s/%s?id=%s", r.Host, REDIRECT_PATH, id)
	fmt.Println("set original url:", newReqBody.Url, "to short-form of:", shortURL)
	newRspBody := &ShortenRspBody{Code: id, ShortUrl: shortURL}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newRspBody)
}

func (s *Server) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
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
