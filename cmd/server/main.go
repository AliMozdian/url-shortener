package main

import (
	"flag"
	"fmt"

	"url-shortener/internal/api"
)

// url shortener service with http server and in-memory storage

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	base := flag.String("base", "http://localhost:8080", "Base URL for short links")
	flag.Parse()

	srv, err := api.NewServer(*base, *port)
	if err != nil {
		newErr := fmt.Errorf("Error while creating new api.server: %w", err)
		fmt.Println(newErr.Error())
		return
	}

	srv.Run()
}
