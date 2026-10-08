package store

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GormLink represents the SQLite database table schema.
// GORM creates a table named `gorm_links` (or `links` if customized).
type GormLink struct {
	Code      string    `gorm:"primaryKey;size:10"`
	Url       string    `gorm:"not null;index"`
	CreatedAt time.Time `gorm:"not null"`
}

// GormDatabase implements the store.Store interface using an underlying SQL database.
type GormDatabase struct {
	db *gorm.DB
}

// NewGormStore opens the SQLite database at dsn, configures it, and migrates the schema.
func NewGormStore(dsn string) (*GormDatabase, error) {
	if dsn == "" {
		dsn = "links.db"
	}

	// Open connection with silent logging to keep benchmark and test outputs clean.
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure the underlying database connection pool for SQLite concurrency.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// SQLite file locking works best when concurrent writes do not cause lock thrashing.
	sqlDB.SetMaxOpenConns(1)

	// Automatically create the table if it doesn't exist already.
	if err := db.AutoMigrate(&GormLink{}); err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	return &GormDatabase{db: db}, nil
}

// Read queries a link by its short code.
func (g *GormDatabase) Read(code string) (LinkRecord, error) {
	var row GormLink
	// Equivalent to: SELECT * FROM gorm_links WHERE code = ? LIMIT 1
	result := g.db.First(&row, "code = ?", code)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return LinkRecord{}, ErrNotFound
		}
		return LinkRecord{}, fmt.Errorf("gorm read error: %w", result.Error)
	}

	return LinkRecord{
		Code:      row.Code,
		Url:       row.Url,
		CreatedAt: row.CreatedAt,
	}, nil
}

// Write persists a link record to disk.
func (g *GormDatabase) Write(record LinkRecord) error {
	row := GormLink{
		Code:      record.Code,
		Url:       record.Url,
		CreatedAt: record.CreatedAt,
	}

	// Equivalent to: INSERT INTO gorm_links (code, url, created_at) VALUES (...)
	result := g.db.Create(&row)
	if result.Error != nil {
		return fmt.Errorf("gorm write error: %w", result.Error)
	}
	return nil
}

// Close closes the underlying SQL database handle.
// Useful during tests to flush and release SQLite locks.
func (g *GormDatabase) Close() error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
