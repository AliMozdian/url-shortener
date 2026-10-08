package store

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("record not found")
)

type LinkRecord struct {
	Code      string    `json:"code"`
	Url       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type Store interface {
	Read(short string) (LinkRecord, error)
	Write(record LinkRecord) error
}

type RamDatabase struct {
	mu      sync.RWMutex
	storage map[string]LinkRecord
}

func NewRam() *RamDatabase {
	return &RamDatabase{mu: sync.RWMutex{}, storage: make(map[string]LinkRecord)}
}

func (db *RamDatabase) Read(code string) (LinkRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rec, exists := db.storage[code]
	if !exists {
		return LinkRecord{}, ErrNotFound
	}
	return rec, nil
}

func (db *RamDatabase) Write(record LinkRecord) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.storage[record.Code] = record // later change
	return nil                       // nothing for now!
}
