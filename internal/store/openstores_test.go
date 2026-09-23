package store

import (
	"path/filepath"
	"testing"
)

// TestOpenStores_TracksOpenAndClose pins O108's /metrics gauge
// (fathom_open_stores, cmd/fathom/servehttp.go): NewSQLiteStore increments
// the process-lifetime openStoreCount, Close decrements it. A daemon that
// opens one store at startup and never re-opens one should hold this
// constant — this test is the unit-level receipt that the counter itself
// moves correctly, not just that some doctor row exists to read it.
func TestOpenStores_TracksOpenAndClose(t *testing.T) {
	before := OpenStores()

	s1, err := NewSQLiteStore("")
	if err != nil {
		t.Fatalf("NewSQLiteStore #1: %v", err)
	}
	if got := OpenStores(); got != before+1 {
		t.Fatalf("OpenStores after 1 open = %d, want %d", got, before+1)
	}

	path := filepath.Join(t.TempDir(), "openstores.sqlite")
	s2, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore #2: %v", err)
	}
	if got := OpenStores(); got != before+2 {
		t.Fatalf("OpenStores after 2 opens = %d, want %d", got, before+2)
	}

	if err := s1.Close(); err != nil {
		t.Fatalf("Close #1: %v", err)
	}
	if got := OpenStores(); got != before+1 {
		t.Fatalf("OpenStores after 1 close = %d, want %d", got, before+1)
	}

	if err := s2.Close(); err != nil {
		t.Fatalf("Close #2: %v", err)
	}
	if got := OpenStores(); got != before {
		t.Fatalf("OpenStores after both closed = %d, want %d (back to baseline)", got, before)
	}
}
