package store

import (
	"fmt"
	"path/filepath"
	"strings"
)

// storeExtensions lists the recognized on-disk store file extensions, in the
// order BackendForPath checks them. ".duckdb" opens DuckStore; ".sqlite" and
// ".db" both open SQLiteStore (".db" is accepted so a bare, unadorned store
// filename still dispatches, matching the historical on-disk test fixtures
// that predate this backend split).
var storeExtensions = []string{".duckdb", ".sqlite", ".db"}

// ServiceIDFromStoreFilename returns the service ID (filename stem) for a
// store file name, and whether name is a recognized store file at all. Used
// by PlatformStore.load, gen-platform-manifest, and platform-ingest so all
// three treat ".duckdb"/".sqlite"/".db" identically instead of each
// hardcoding its own ".duckdb"-only suffix check (ADR-020).
func ServiceIDFromStoreFilename(name string) (string, bool) {
	for _, ext := range storeExtensions {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext), true
		}
	}
	return "", false
}

// BackendForPath opens the Store backend appropriate for path's extension:
// ".duckdb" opens a DuckStore, ".sqlite" or ".db" opens a SQLiteStore. An
// empty path opens an in-memory DuckStore — the longstanding behavior for
// ModeFaux/session-mode callers that pass "" for an ephemeral store, kept
// unchanged rather than silently reinterpreted under the SQLite-default flip
// (ADR-020's "defaults flip" governs on-disk CLI/init defaults, not the
// existing in-memory convention). An unrecognized extension is a typed
// error naming both options, never a silent fallback.
func BackendForPath(path string) (Store, error) {
	if path == "" {
		return OpenDuckDB("")
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".duckdb":
		return OpenDuckDB(path)
	case ".sqlite", ".db":
		return NewSQLiteStore(path)
	default:
		return nil, fmt.Errorf("store: unrecognized store extension %q for %q (expected .duckdb or .sqlite/.db)", ext, path)
	}
}

// Open is the general-purpose store constructor used throughout fathom
// (pkg/client, cmd/fathom). It dispatches via BackendForPath — see that
// function's doc for the exact extension rules, including the in-memory
// "" case.
func Open(path string) (Store, error) {
	s, err := BackendForPath(path)
	if err != nil {
		return nil, fmt.Errorf("store.Open: %w", err)
	}
	return s, nil
}
