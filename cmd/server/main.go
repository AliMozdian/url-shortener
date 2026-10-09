package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/AliMozdian/url-shortener/internal/api"
)

// url shortener service with http server and in-memory storage

func run(args []string, stdout io.Writer, actuallyRun bool) error {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(stdout)

	addr := flags.String("addr", "8080", "HTTP listen address")
	base := flags.String("base", "http://localhost:8080", "Base URL for short links")
	db := flags.String("db", "ram", "Database mode for link storage")
	dsn := flags.String("dsn", "links.db", "SQLite DSN database file path")
	cachCap := flags.Int("cache-cap", 0, "Cache Capacity for persistent DBs "+
		"(-1 for no cache, 0 for default, +value for actual capacity)")

	if err := flags.Parse(args); err != nil {
		return err
	}

	srv, err := api.NewServer(*base, *addr, *db, *dsn, *cachCap)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if actuallyRun {
		srv.Run()
	}
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, true); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
