package main

import (
	"bytes"
	"testing"
)

func TestRun_ValidFlags(t *testing.T) {
	buf := new(bytes.Buffer)
	err := run([]string{"-base", "http://localhost:8080", "-addr", "8080"}, buf, false)
	if err != nil {
		t.Fatalf("expected run to succeed, got %v", err)
	}
}

func TestRun_InvalidFlags(t *testing.T) {
	buf := new(bytes.Buffer)
	err := run([]string{"-unknown-flag"}, buf, false)
	if err == nil {
		t.Fatalf("expected error for unknown flag, got nil")
	}
}

func TestRun_InvalidPort(t *testing.T) {
	buf := new(bytes.Buffer)
	// Passing invalid port that triggers api.NewServer error
	err := run([]string{"-addr", "not-a-port"}, buf, false)
	if err == nil {
		t.Fatalf("expected error for invalid port, got nil")
	}
}
