package docindex

// ingest.go — IngestDocUnits (KR-8, docs/design/knowledge-retrieval.md §4:
// "Units are written at ingest when a doc file's content hash changes
// (hash-gated, P19); a deleted file's units are removed in the same
// transaction") and the per-service byte budget it applies (KR-9, §8).
// Wires docCorpusFiles (KR-3) and SplitDocUnits (KR-2) into
// store.PutDocUnits (KR-1), and enforceDocBudget over the whole corpus.
// internal/fusion's ingest is the production caller (docindexpass.go).
//
// SQLite-only, like the store half it drives (docindex.go's doc comment):
// takes *store.SQLiteStore directly rather than the generic store.Store
// interface, since PutDocUnits/DeleteDocUnitsForPath/DocIndexedPaths only
// exist on that concrete type.
//
// The gate is doc_units itself. A file is unchanged when the units its
// fresh split would store (the ones inside the budget) equal the units
// already stored for its path — same ids, content hashes, line ranges,
// heading paths, kinds and staleness flag. No second record of "what I
// saw last time" exists, so the gate cannot drift from the index it guards
// (a removed service, a purge, a budget change all read as "changed" the
// moment the stored units differ). An earlier draft kept a per-file
// fingerprint in the store's shared fingerprints table under a "docindex:"
// prefix; store.ListSourceRefs unions that table with observations and the
// pipeline's orphan purge deletes every stored ref the connector did not
// discover, so the gate was erased (and counted as a deleted file) by the
// very next code ingest.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// MaxDocFileBytes is the largest corpus file the pass reads (the same 2 MiB
// per-file bound dependency docs carry, design system-graph §8.3): a
// generated or pasted-in Markdown file past it is named and not indexed,
// never read whole into memory.
const MaxDocFileBytes = 2 * 1024 * 1024

// IngestOption tunes one IngestDocUnits call.
type IngestOption func(*ingestConfig)

type ingestConfig struct {
	budgetBytes int
	keep        func(path string) bool
}

// WithBudgetBytes sets the per-service byte budget (KR-9); n <= 0 keeps
// DefaultDocBudgetBytes.
func WithBudgetBytes(n int) IngestOption {
	return func(c *ingestConfig) { c.budgetBytes = n }
}

// WithCorpusFilter restricts the corpus to the repo-relative, slash-form
// paths keep accepts. The production caller passes the connector's own
// Discover set, so the doc index never holds a document the code index was
// told to leave out: .fathomignore patterns, a service's scoped exclusions
// (the purge chokepoint's ignore.d/<service>), default-ignored cache
// directories. A previously indexed document that stops passing the filter
// leaves the index like a deleted one.
func WithCorpusFilter(keep func(path string) bool) IngestOption {
	return func(c *ingestConfig) { c.keep = keep }
}

// DocFileFailure names one corpus file this pass could not index (I38: an
// unreadable input is never a healthy zero). An unreadable file's
// previously stored units are left exactly as they were; an over-bound file
// (MaxDocFileBytes) is not indexed and any units it held when smaller are
// removed.
type DocFileFailure struct {
	Path string
	Err  string
}

// IngestDocUnitsResult summarizes one IngestDocUnits call — what it did,
// not what it found (that lives in the store afterward, queryable via
// QueryDocUnits/DocUnitsForPath).
type IngestDocUnitsResult struct {
	FilesScanned   int // every corpus file docCorpusFiles found
	FilesChanged   int // re-split and re-written: its stored units differed from what now fits
	FilesUnchanged int // stored units already equal the file's; nothing written
	FilesDeleted   int // previously indexed, no longer in the corpus; units removed
	UnitsWritten   int // total units written across every changed file

	// KR-9: what the per-service byte budget cut, in the order it cut it
	// (live documents before stale_by_location ones, then path and heading
	// path). Named, never a silent trim; empty when everything fit.
	UnitsElided int
	ElidedBytes int
	Elided      []ElidedDocUnit

	// Failures names every corpus file that could not be read, sorted by
	// path. Its stored units stand; the rest of the corpus was indexed.
	Failures []DocFileFailure
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// unitID builds a stable id for a unit within path: sha256 of the path and
// its heading path, so the same section keeps the same id across ingests
// (doc_links, KR-6, anchors to it) as long as the heading path itself does
// not change.
func unitID(path, headingPath string, kind string) string {
	return sha256Hex([]byte(path + "\x00" + headingPath + "\x00" + kind))
}

// buildDocUnits splits one file into the store's unit rows. Two sections
// with the same heading path and kind (a changelog's repeated "### Added",
// an ADR's repeated bold label) would share a unit id and fail doc_units'
// primary key, so the second and later occurrence, in document order, is
// keyed with its ordinal: deterministic, and stable while the sections stay
// in the same order.
func buildDocUnits(cf CorpusFile, raw []byte, now time.Time) []store.DocUnit {
	units := SplitDocUnits(cf.Kind, string(raw))
	seen := make(map[string]int, len(units))
	out := make([]store.DocUnit, 0, len(units))
	for _, u := range units {
		key := u.HeadingPath + "\x00" + u.Kind
		n := seen[key]
		seen[key]++
		idPath := u.HeadingPath
		if n > 0 {
			idPath += "\x00#" + strconv.Itoa(n+1)
		}
		out = append(out, store.DocUnit{
			UnitID:          unitID(cf.Path, idPath, u.Kind),
			Path:            cf.Path,
			LineStart:       u.LineStart,
			LineEnd:         u.LineEnd,
			HeadingPath:     u.HeadingPath,
			Kind:            u.Kind,
			ContentHash:     sha256Hex([]byte(u.Body)),
			IndexedAt:       now.UnixNano(),
			StaleByLocation: cf.StaleByLocation,
			Body:            u.Body,
		})
	}
	return out
}

// docUnitStub is u without its Body but with the size the budget charges
// for it, so the planning pass over the whole corpus holds metadata only.
func docUnitStub(u store.DocUnit) store.DocUnit {
	u.SizeBytes = len(u.HeadingPath) + len(u.Body)
	u.Body = ""
	return u
}

// readDocUnits reads and splits one corpus file.
func readDocUnits(root string, cf CorpusFile, now time.Time) ([]store.DocUnit, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cf.Path)))
	if err != nil {
		return nil, err
	}
	return buildDocUnits(cf, raw, now), nil
}

// sameDocUnits reports whether a and b hold the same units: everything a
// search or a link reads back, IndexedAt excluded (it is the run's clock).
func sameDocUnits(a, b []store.DocUnit) bool {
	if len(a) != len(b) {
		return false
	}
	sorted := func(in []store.DocUnit) []store.DocUnit {
		out := make([]store.DocUnit, len(in))
		copy(out, in)
		sort.Slice(out, func(i, j int) bool {
			if out[i].LineStart != out[j].LineStart {
				return out[i].LineStart < out[j].LineStart
			}
			return out[i].UnitID < out[j].UnitID
		})
		return out
	}
	as, bs := sorted(a), sorted(b)
	for i := range as {
		x, y := as[i], bs[i]
		if x.UnitID != y.UnitID || x.ContentHash != y.ContentHash || x.LineStart != y.LineStart ||
			x.LineEnd != y.LineEnd || x.HeadingPath != y.HeadingPath || x.Kind != y.Kind ||
			x.StaleByLocation != y.StaleByLocation {
			return false
		}
	}
	return true
}

// IngestDocUnits walks root's corpus (docCorpusFiles), splits every file,
// applies the per-service byte budget (DefaultDocBudgetBytes) across the
// whole corpus, and writes, per file, exactly the files whose stored units
// differ from what now fits — never a whole-service replace (§8's cost
// table: "a docs-only change re-indexes only changed units"). It removes a
// previously indexed file's units when the corpus no longer contains it.
//
// now stamps every written unit's IndexedAt; the caller supplies it
// (typically time.Now()) rather than this function calling the clock
// itself, so a test can pin it.
//
// Two passes over the corpus keep memory bounded to the changed files: pass
// one splits every file and keeps body-less stubs (ids, hashes, sizes) to
// plan the budget over the whole service; pass two, per file, compares the
// stubs inside the budget with the stored units and re-reads only the files
// that differ to write them.
func IngestDocUnits(ctx context.Context, st *store.SQLiteStore, serviceID, root string, now time.Time, opts ...IngestOption) (IngestDocUnitsResult, error) {
	var cfg ingestConfig
	for _, o := range opts {
		o(&cfg)
	}
	var res IngestDocUnitsResult

	walked, err := docCorpusFiles(root)
	if err != nil {
		return res, fmt.Errorf("IngestDocUnits: corpus walk: %w", err)
	}
	corpus := walked[:0:0]
	for _, cf := range walked {
		if cfg.keep == nil || cfg.keep(cf.Path) {
			corpus = append(corpus, cf)
		}
	}
	res.FilesScanned = len(corpus)

	type filePlan struct {
		cf    CorpusFile
		stubs []store.DocUnit
	}
	present := make(map[string]bool, len(corpus))
	plans := make([]filePlan, 0, len(corpus))
	var candidates []store.DocUnit
	for _, cf := range corpus {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(cf.Path))); err == nil && fi.Size() > MaxDocFileBytes {
			res.Failures = append(res.Failures, DocFileFailure{Path: cf.Path,
				Err: fmt.Sprintf("%d bytes is over the %d-byte per-file bound; not indexed", fi.Size(), MaxDocFileBytes)})
			continue // not present: units it held when smaller are removed
		}
		present[cf.Path] = true
		units, err := readDocUnits(root, cf, now)
		if err != nil {
			res.Failures = append(res.Failures, DocFileFailure{Path: cf.Path, Err: err.Error()})
			continue
		}
		stubs := make([]store.DocUnit, len(units))
		for i, u := range units {
			stubs[i] = docUnitStub(u)
		}
		plans = append(plans, filePlan{cf: cf, stubs: stubs})
		candidates = append(candidates, stubs...)
	}

	budget := enforceDocBudget(candidates, cfg.budgetBytes)
	kept := make(map[string]bool, len(budget.Kept))
	for _, u := range budget.Kept {
		kept[u.UnitID] = true
	}
	res.Elided, res.UnitsElided, res.ElidedBytes = budget.Elided, len(budget.Elided), budget.ElidedBytes

	fits := func(units []store.DocUnit) []store.DocUnit {
		var out []store.DocUnit
		for _, u := range units {
			if kept[u.UnitID] {
				out = append(out, u)
			}
		}
		return out
	}
	for _, p := range plans {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		want := fits(p.stubs)
		stored, err := st.DocUnitsForPath(ctx, serviceID, p.cf.Path)
		if err != nil {
			return res, fmt.Errorf("IngestDocUnits: stored units %s: %w", p.cf.Path, err)
		}
		if sameDocUnits(want, stored) {
			res.FilesUnchanged++
			continue
		}
		units, err := readDocUnits(root, p.cf, now)
		if err != nil {
			res.Failures = append(res.Failures, DocFileFailure{Path: p.cf.Path, Err: err.Error()})
			continue
		}
		write := fits(units)
		stubsOfWrite := make([]store.DocUnit, len(write))
		for i, u := range write {
			stubsOfWrite[i] = docUnitStub(u)
		}
		if !sameDocUnits(stubsOfWrite, want) {
			// The bytes moved between the planning read and this one; the
			// budget was planned over the earlier text. Leave the stored
			// units alone; the next ingest plans over the new bytes.
			res.Failures = append(res.Failures, DocFileFailure{Path: p.cf.Path, Err: "changed while it was being indexed; re-run the ingest"})
			continue
		}
		if err := st.PutDocUnits(ctx, serviceID, p.cf.Path, write); err != nil {
			return res, fmt.Errorf("IngestDocUnits: PutDocUnits %s: %w", p.cf.Path, err)
		}
		res.FilesChanged++
		res.UnitsWritten += len(write)
	}

	indexed, err := st.DocIndexedPaths(ctx, serviceID)
	if err != nil {
		return res, fmt.Errorf("IngestDocUnits: DocIndexedPaths: %w", err)
	}
	for _, path := range indexed {
		if present[path] {
			continue // in the corpus: read, or named in Failures with its units kept
		}
		if err := st.DeleteDocUnitsForPath(ctx, serviceID, path); err != nil {
			return res, fmt.Errorf("IngestDocUnits: DeleteDocUnitsForPath %s: %w", path, err)
		}
		res.FilesDeleted++
	}

	return res, nil
}
