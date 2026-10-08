package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AliMozdian/url-shortener/internal/shortener"
	"github.com/AliMozdian/url-shortener/internal/store"
)

type Server struct {
	base       string
	port       string
	mux        *http.ServeMux
	httpServer *http.Server
	shortener  *shortener.Shortener
}

// creates a new Server (my struct for handling APIs)
func NewServer(base, port, dbMode, dsn string) (*Server, error) {
	port = strings.TrimPrefix(port, ":")
	portAsInt, err := strconv.Atoi(port)
	if err != nil || portAsInt < 0 || portAsInt > 65535 {
		return nil, fmt.Errorf("port must be an int, and between 0 and 65535!, not %q", port)
	}

	var db store.Store
	switch strings.ToLower(dbMode) {
	case "sqlite", "gorm", "persistent":
		gormStore, err := store.NewGormStore(dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize sqlite store: %w", err)
		}
		db = gormStore
	case "in-memory", "ram":
		db = store.NewRam()
	case "fake", "test":
		db = store.NewFakeStore()
	default:
		return nil, fmt.Errorf("unkown value for db flag (arg)! expected gorm/sqlite, ram or fake, but got %q", dbMode)
	}

	s := &Server{base: base, port: port}
	s.shortener = shortener.New(db)

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/api/shorten", s.handleShorten)
	s.mux.HandleFunc("GET /{id}", s.handleRedirect)
	s.mux.HandleFunc("GET /api/v1/links/{id}", s.handleMetadata)

	s.httpServer = &http.Server{
		Addr:         ":" + port,
		Handler:      s.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return s, nil
}

// just like NewServer but instead of dbMode it receives a shortner
// useful for tests having access to the shortner and its db
func NewServerWithShortner(base, port string, sh *shortener.Shortener) (*Server, error) {
	port = strings.TrimPrefix(port, ":")
	portAsInt, err := strconv.Atoi(port)
	if err != nil || portAsInt < 0 || portAsInt > 65535 {
		return nil, fmt.Errorf("port must be an int, and between 0 and 65535!, not %q", port)
	}
	if sh == nil {
		return nil, fmt.Errorf("shortner of a server cannot be nil!")
	}
	s := &Server{base: base, port: port}
	s.shortener = sh

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/api/shorten", s.handleShorten)
	s.mux.HandleFunc("GET /{id}", s.handleRedirect)
	s.mux.HandleFunc("GET /api/v1/links/{id}", s.handleMetadata)

	s.httpServer = &http.Server{Addr: ":" + port, Handler: s.mux}
	return s, nil
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
