package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// storeExtensions lists the recognized on-disk store file extensions, in the
// order BackendForPath checks them. ".sqlite" and ".db" both open
// SQLiteStore (".db" is accepted so a bare, unadorned store filename still
// dispatches, matching the historical on-disk test fixtures that predate
// this backend split). ".duckdb" is still RECOGNIZED here — so a leftover
// pre-SS-8 store file still participates in same-stem collision detection
// instead of silently vanishing from ResolveStoreFiles — but BackendForPath
// refuses to ever OPEN one: the DuckDB backend itself was deleted in SS-8
// (docs/plans/store-sqlite.md; ADR-020/ADR-025), so a lone ".duckdb" file is
// a loud error, never a silent fallback.
var storeExtensions = []string{".duckdb", ".sqlite", ".db"}

// storeExtensionRank orders extensions by preference when one service stem
// has multiple store files — the shape a pre-SS-8 `migrate-store` run (now
// removed) left an operator in mid-migration, or any other stale leftover
// takes. Lower wins. The SQLite backend is preferred unconditionally: post
// SS-8 ".duckdb" can never be opened at all, so preferring it over a
// perfectly good ".sqlite" sibling would turn a harmless leftover file into
// a hard failure.
var storeExtensionRank = map[string]int{".sqlite": 0, ".db": 1, ".duckdb": 2}

// ServiceIDFromStoreFilename returns the service ID (filename stem) for a
// store file name, and whether name is a recognized store file at all.
// Extension matching is case-insensitive (mirroring BackendForPath's
// ToLower on the extension); the stem keeps its original case. Used by
// PlatformStore.load, gen-platform-manifest, and platform-ingest so all
// three treat ".duckdb"/".sqlite"/".db" identically instead of each
// hardcoding its own suffix check (ADR-020).
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

// errDuckDBRemoved is returned (wrapped with the offending path) whenever a
// caller tries to actually OPEN a ".duckdb" store. The DuckDB backend — and
// CGO along with it — was deleted outright in SS-8 once the fleet finished
// migrating to SQLite (docs/plans/store-sqlite.md; ADR-020/ADR-025). This is
// deliberately loud: a silent fallback here would mean silently serving (or
// silently failing to serve) whatever stale pre-migration data the file
// still holds.
func errDuckDBRemoved(path string) error {
	return fmt.Errorf("store: %q is a DuckDB store; DuckDB stores are no longer supported (removed in SS-8) — re-open it with a pre-SS-8 build of fathom to read it, or migrate-store it to .sqlite with that build first", path)
}

// BackendForPath opens the Store backend appropriate for path's extension:
// ".sqlite" or ".db" opens a SQLiteStore; ".duckdb" is recognized (so
// same-stem collision detection still sees it) but always refused with
// errDuckDBRemoved rather than opened. An empty path opens an in-memory
// SQLiteStore — pre-SS-8 this was an in-memory DuckDB database; the
// ModeFaux/session-mode callers that pass "" for an ephemeral store see no
// behavioral difference (both are process-local and discarded on Close), so
// this is not treated as a breaking change to the "" convention itself. An
// unrecognized extension is a typed error naming the supported option, never
// a silent fallback.
func BackendForPath(path string) (Store, error) {
	if path == "" {
		return NewSQLiteStore("")
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".duckdb":
		return nil, errDuckDBRemoved(path)
	case ".sqlite", ".db":
		return NewSQLiteStore(path)
	default:
		return nil, fmt.Errorf("store: unrecognized store extension %q for %q (expected .sqlite or .db)", ext, path)
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
