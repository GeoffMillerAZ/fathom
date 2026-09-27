package custodian

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestDefaultDaemonURLUnroutableUnderTest (RT-18, §10.4): the custodian's
// daemon default has the same backstop as cmd/fathom's.
func TestDefaultDaemonURLUnroutableUnderTest(t *testing.T) {
	t.Setenv("FATHOM_HTTP", "")
	if got := DefaultDaemonURL(); got != testisolate.UnroutableURL {
		t.Fatalf("DefaultDaemonURL() = %q under test, want %q", got, testisolate.UnroutableURL)
	}
	t.Setenv("FATHOM_HTTP", "http://127.0.0.1:7852")
	if got := DefaultDaemonURL(); got != "http://127.0.0.1:7852" {
		t.Fatalf("DefaultDaemonURL() = %q, want the explicit FATHOM_HTTP", got)
	}
}
