package store

import "sync"

type Store interface {
	Read(short string) (long string, exists bool)
	Write(short, long string) error
}

type RamDatabase struct {
	mu      sync.RWMutex
	storage map[string]string
}

func NewRam() *RamDatabase {
	return &RamDatabase{mu: sync.RWMutex{}, storage: make(map[string]string)}
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
