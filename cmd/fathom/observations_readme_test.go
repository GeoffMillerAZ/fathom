package main

import (
	"os"
	"testing"
)

// This repository's own .fathom/observations/README.md is the header
// `fathom observe` writes into a fresh per-entry directory (O179, O291):
// pinned byte for byte so the two never drift.
func TestObservationsReadmeEqualsHeader(t *testing.T) {
	got, err := os.ReadFile("../../.fathom/observations/README.md")
	if err != nil {
		t.Fatalf("read the repo's observations README: %v", err)
	}
	if string(got) != observationsHeader {
		t.Fatalf("README.md differs from observationsHeader:\n--- got\n%s\n--- want\n%s", got, observationsHeader)
	}
}
