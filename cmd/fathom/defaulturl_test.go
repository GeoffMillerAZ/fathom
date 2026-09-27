package main

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestDefaultURLUnroutableUnderTest (RT-17, §10.4): with FATHOM_HTTP unset,
// a test binary's default daemon URL is unroutable — never the operator's
// localhost:7575 — and an explicit FATHOM_HTTP still wins.
func TestDefaultURLUnroutableUnderTest(t *testing.T) {
	t.Setenv("FATHOM_HTTP", "")
	if got := defaultFathomURL(); got != testisolate.UnroutableURL {
		t.Fatalf("defaultFathomURL() = %q under test, want %q", got, testisolate.UnroutableURL)
	}
	t.Setenv("FATHOM_HTTP", "http://127.0.0.1:7851")
	if got := defaultFathomURL(); got != "http://127.0.0.1:7851" {
		t.Fatalf("defaultFathomURL() = %q, want the explicit FATHOM_HTTP", got)
	}
}
