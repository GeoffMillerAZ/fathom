package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// storeExtensions lists the recognized on-disk store file extensions, in the
// order BackendForPath checks them. ".duckdb" opens DuckStore; ".sqlite" and
// ".db" both open SQLiteStore (".db" is accepted so a bare, unadorned store
// filename still dispatches, matching the historical on-disk test fixtures
// that predate this backend split).
var storeExtensions = []string{".duckdb", ".sqlite", ".db"}

// storeExtensionRank orders extensions by preference when one service stem
// has multiple store files — the mid-migration state migrate-store leaves
// (svc.duckdb beside svc.sqlite). Lower wins. The SQLite backend is
// preferred because it is the migration TARGET: serving the .duckdb file in
// that state means serving stale, pre-migration data silently.
var storeExtensionRank = map[string]int{".sqlite": 0, ".db": 1, ".duckdb": 2}

// ServiceIDFromStoreFilename returns the service ID (filename stem) for a
// store file name, and whether name is a recognized store file at all.
// Extension matching is case-insensitive (mirroring BackendForPath's
// ToLower on the extension); the stem keeps its original case. Used by
// PlatformStore.load, gen-platform-manifest, and platform-ingest so all
// three treat ".duckdb"/".sqlite"/".db" identically instead of each
// hardcoding its own ".duckdb"-only suffix check (ADR-020).
func ServiceIDFromStoreFilename(name string) (string, bool) {
	lower := strings.ToLower(name)
	for _, ext := range storeExtensions {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)], true
		}
	}
	return "", false
}

// StoreCollision records one same-stem conflict found by ResolveStoreFiles:
// a service whose stem appears under multiple store files. Winner is the
// file being served; Losers (sorted) are the shadowed files the operator
// should remove once the migration is confirmed.
type StoreCollision struct {
	Stem   string
	Winner string
	Losers []string
}

// ResolveStoreFiles groups store filenames by service stem and resolves
// same-stem duplicates by backend preference (.sqlite > .db > .duckdb; ties
// broken by name for determinism). It returns stem → winning filename plus
// the sorted list of collisions so callers (PlatformStore.load,
// gen-platform-manifest) can warn loudly instead of silently serving —
// or double-listing — a stale store. Input order never affects the outcome;
// os.ReadDir's name-sort put svc.duckdb before svc.sqlite, which is exactly
// how the original first-name-wins bug served pre-migration data.
func ResolveStoreFiles(names []string) (map[string]string, []StoreCollision) {
	byStem := make(map[string][]string)
	for _, name := range names {
		stem, ok := ServiceIDFromStoreFilename(name)
		if !ok {
			continue
		}
		byStem[stem] = append(byStem[stem], name)
	}

	winners := make(map[string]string, len(byStem))
	var collisions []StoreCollision
	for stem, candidates := range byStem {
		sort.Slice(candidates, func(i, j int) bool {
			ri := extensionRank(candidates[i])
			rj := extensionRank(candidates[j])
			if ri != rj {
				return ri < rj
			}
			return candidates[i] < candidates[j]
		})
		winners[stem] = candidates[0]
		if len(candidates) > 1 {
			losers := make([]string, len(candidates)-1)
			copy(losers, candidates[1:])
			sort.Strings(losers)
			collisions = append(collisions, StoreCollision{Stem: stem, Winner: candidates[0], Losers: losers})
		}
	}
	sort.Slice(collisions, func(i, j int) bool { return collisions[i].Stem < collisions[j].Stem })
	return winners, collisions
}

// extensionRank returns the preference rank for a store filename's
// extension (lower wins); unranked extensions sort last.
func extensionRank(name string) int {
	if r, ok := storeExtensionRank[strings.ToLower(filepath.Ext(name))]; ok {
		return r
	}
	return len(storeExtensionRank)
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
