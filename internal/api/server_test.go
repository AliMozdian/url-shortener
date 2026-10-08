package api

import (
	"testing"

	"github.com/AliMozdian/url-shortener/internal/shortener"
	"github.com/AliMozdian/url-shortener/internal/store"
)

func TestNewServerValidation(t *testing.T) {
	validBase := "http://localhost:8080" // later check and test invalid base

	validPorts := []string{"123", "60222", "0000"}
	for _, vp := range validPorts {
		if _, err := NewServer(validBase, vp, "ram"); err != nil {
			t.Errorf("port %q must be accepted as it is an int, >=0 and <=65535", vp)
		}
	}

	noIntPort := "8a8b"
	if _, err := NewServer(validBase, noIntPort, "ram"); err == nil {
		t.Errorf("port %q must not be accepted as it's not an int", noIntPort)
	}

	negativePort := "-123"
	if _, err := NewServer(validBase, negativePort, "ram"); err == nil {
		t.Errorf("port %q must not be accepted as it's must be non-negative", negativePort)
	}

	moreThanMaxPort := "65536"
	if _, err := NewServer(validBase, moreThanMaxPort, "ram"); err == nil {
		t.Errorf("port %q must not be accepted as it's higher than max valid port 65535", moreThanMaxPort)
	}
}

func TestNewServerWithShortener(t *testing.T) {
	validBase := "http://localhost:8080" // later check and test invalid base

	validPorts := []string{"123", "60222", "0000"}
	shFakeDb := shortener.New(store.NewFakeStore())
	for _, vp := range validPorts {
		if _, err := NewServerWithShortner(validBase, vp, shFakeDb); err != nil {
			t.Errorf("port %q must be accepted as it is an int, >=0 and <=65535", vp)
		}
	}

	if _, err := NewServerWithShortner("http://localhost:8080", "8080", nil); err == nil {
		t.Errorf("server with shortener=nil must return error not")
	}

	if _, err := NewServerWithShortner("http://localhost:8080", "abcd", nil); err == nil {
		t.Errorf("port %q must not be accepted as it's not an int", "abcd")
	}
}

func TestCreateFakeStoreServer(t *testing.T) {
	_, err := NewServer("http://localhost:8080", "8080", "fake")
	if err != nil {
		t.Fatalf("failed to create new server with dbMode=fake: %v", err)
	}
	// nothing more for now :) I just wanted more coverage :)
}
