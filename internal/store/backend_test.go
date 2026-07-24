package store

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestServiceIDFromStoreFilename(t *testing.T) {
	cases := []struct {
		name     string
		wantStem string
		wantOK   bool
	}{
		{"svc.duckdb", "svc", true},
		{"svc.sqlite", "svc", true},
		{"svc.db", "svc", true},
		// Extension matching is case-insensitive, mirroring BackendForPath's
		// strings.ToLower on the extension; the stem keeps its original case.
		{"SVC.SQLITE", "SVC", true},
		{"Svc.DuckDB", "Svc", true},
		{"readme.txt", "", false},
		{"noext", "", false},
	}
	for _, tc := range cases {
		stem, ok := ServiceIDFromStoreFilename(tc.name)
		if stem != tc.wantStem || ok != tc.wantOK {
			t.Errorf("ServiceIDFromStoreFilename(%q) = (%q, %v), want (%q, %v)",
				tc.name, stem, ok, tc.wantStem, tc.wantOK)
		}
	}
}

// TestResolveStoreFiles is the F2 regression: a directory holding BOTH
// svc.duckdb and svc.sqlite for the same stem — the exact state migrate-store
// leaves an operator in mid-migration — must resolve to the .sqlite file (the
// migration target), never silently serve the stale .duckdb, and must report
// the collision so callers can warn loudly. Both input orders are exercised
// because os.ReadDir's name-sort always yields .duckdb before .sqlite, which
// is precisely how the original first-name-wins bug hid.
func TestResolveStoreFiles(t *testing.T) {
	cases := []struct {
		name           string
		names          []string
		want           map[string]string
		wantCollisions []StoreCollision
	}{
		{
			name:  "no collisions",
			names: []string{"alpha.duckdb", "beta.sqlite"},
			want:  map[string]string{"alpha": "alpha.duckdb", "beta": "beta.sqlite"},
		},
		{
			name:  "duckdb listed first (ReadDir order) — sqlite still wins",
			names: []string{"svc.duckdb", "svc.sqlite"},
			want:  map[string]string{"svc": "svc.sqlite"},
			wantCollisions: []StoreCollision{
				{Stem: "svc", Winner: "svc.sqlite", Losers: []string{"svc.duckdb"}},
			},
		},
		{
			name:  "sqlite listed first — same outcome (order-independent)",
			names: []string{"svc.sqlite", "svc.duckdb"},
			want:  map[string]string{"svc": "svc.sqlite"},
			wantCollisions: []StoreCollision{
				{Stem: "svc", Winner: "svc.sqlite", Losers: []string{"svc.duckdb"}},
			},
		},
		{
			name:  "three-way: .sqlite beats .db beats .duckdb",
			names: []string{"svc.db", "svc.duckdb", "svc.sqlite"},
			want:  map[string]string{"svc": "svc.sqlite"},
			wantCollisions: []StoreCollision{
				{Stem: "svc", Winner: "svc.sqlite", Losers: []string{"svc.db", "svc.duckdb"}},
			},
		},
		{
			name:  "db beats duckdb when no .sqlite",
			names: []string{"svc.duckdb", "svc.db"},
			want:  map[string]string{"svc": "svc.db"},
			wantCollisions: []StoreCollision{
				{Stem: "svc", Winner: "svc.db", Losers: []string{"svc.duckdb"}},
			},
		},
		{
			name:  "non-store files ignored",
			names: []string{"svc.sqlite", "README.txt", "svc.sqlite-wal"},
			want:  map[string]string{"svc": "svc.sqlite"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, collisions := ResolveStoreFiles(tc.names)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("winners = %v, want %v", got, tc.want)
			}
			if len(tc.wantCollisions) == 0 {
				if len(collisions) != 0 {
					t.Errorf("collisions = %+v, want none", collisions)
				}
				return
			}
			if !reflect.DeepEqual(collisions, tc.wantCollisions) {
				t.Errorf("collisions = %+v, want %+v", collisions, tc.wantCollisions)
			}
		})
	}
}

// TestBackendForPath_DuckDBLoudlyRefused is the SS-8 regression: DuckDB was
// deleted outright, so BackendForPath must never silently open (or silently
// fall back away from) a ".duckdb" path — it must return a clear, typed
// error naming the file and pointing at a pre-SS-8 build, every time.
func TestBackendForPath_DuckDBLoudlyRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "svc.duckdb")
	s, err := BackendForPath(path)
	if err == nil {
		if s != nil {
			_ = s.Close()
		}
		t.Fatal("BackendForPath(.duckdb) succeeded, want a loud error")
	}
	if s != nil {
		t.Errorf("BackendForPath(.duckdb) returned a non-nil Store alongside an error: %v", s)
	}
	if !strings.Contains(err.Error(), "DuckDB") || !strings.Contains(err.Error(), path) {
		t.Errorf("error = %q, want it to name DuckDB and the offending path %q", err.Error(), path)
	}
}

// TestBackendForPath_EmptyPathIsSQLite proves the in-memory "" convention
// now opens SQLiteStore, not DuckStore (SS-8 removed the latter entirely).
func TestBackendForPath_EmptyPathIsSQLite(t *testing.T) {
	s, err := BackendForPath("")
	if err != nil {
		t.Fatalf("BackendForPath(\"\"): %v", err)
	}
	defer s.Close()
	if _, ok := s.(*SQLiteStore); !ok {
		t.Errorf("BackendForPath(\"\") = %T, want *SQLiteStore", s)
	}
}
