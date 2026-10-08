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

	if err := flags.Parse(args); err != nil {
		return err
	}

	srv, err := api.NewServer(*base, *addr)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// For testing setup/flag parsing without blocking:
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
