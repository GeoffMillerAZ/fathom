package docindex

// depdocs_ingest.go — IngestDepDocs, the write half of KR-11
// (docs/design/knowledge-retrieval.md §7): a pinned package's doc units, read
// from the toolchain cache through a DepCacheLocator, land in the existing
// docs_fts/doc_units tables under the package's own scope — the canonical
// `pkg:<ecosystem>/<name>@<version>` key is the service id — so a doc unit is
// version-exact by construction and no table, cache or index file is added
// (KR-X3). It is what the dependency plane's indexer (system-graph SG-37)
// calls once per package version; nothing here runs on a request path.
//
// Like IngestDocUnits it is hash-gated per file (a version's cache does not
// change, so a repeat call re-writes nothing), removes a vanished file's
// units, and applies the per-package byte budget of KR-9 (enforceDocBudget)
// with the cut named — recorded beside the units (one fingerprint row) so a
// search of the package can say it is partial (I38).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	// depFingerprintPrefix namespaces this file's per-path content-hash gate
	// in the store's (serviceID, sourceRef) fingerprint table, the same
	// convention docFingerprintPrefix uses for a repository's docs.
	depFingerprintPrefix = "depdocs:"
	// depReportRef is the fingerprint row that marks a package version as
	// indexed (even with no doc units) and carries what indexing cut.
	depReportRef = "depdocs:report"

	// DefaultDepDocBudgetBytes caps the FTS bytes one package version may
	// hold; a package over it keeps a priority-ordered prefix and names the
	// rest as elided. It sits well under the per-service default because a
	// dependency's docs are read in bulk and rarely all wanted.
	DefaultDepDocBudgetBytes = 4 * 1024 * 1024
)

// IngestDepDocsResult summarizes one IngestDepDocs call.
type IngestDepDocsResult struct {
	Key            string // the package scope the units were written under
	FilesScanned   int    // cache files that produced units
	FilesChanged   int    // re-written (new or changed)
	FilesUnchanged int    // hash matched; skipped
	FilesDeleted   int    // previously indexed, gone now; units removed
	UnitsWritten   int
	// Elided names every unit the byte budget cut, by path and heading.
	Elided []ElidedDocUnit
	// Report is what the cache read did (bounds hit, files skipped).
	Report DepDocsReport
}

// depIndexReport is the JSON kept in the depReportRef fingerprint.
type depIndexReport struct {
	Elided  int    `json:"elided"`
	Skipped int    `json:"skipped"`
	Partial string `json:"partial,omitempty"`
}

// IngestDepDocs reads ref's docs from the cache loc points at and writes
// them under ref.Key(). It returns ErrNotCached (nothing written, nothing
// fetched) when the cache does not hold the pinned version.
func IngestDepDocs(ctx context.Context, st *store.SQLiteStore, loc DepCacheLocator, ref PkgRef, now time.Time) (IngestDepDocsResult, error) {
	return ingestDepDocs(ctx, st, loc, ref, now, defaultDepLimits, DefaultDepDocBudgetBytes)
}

func ingestDepDocs(ctx context.Context, st *store.SQLiteStore, loc DepCacheLocator, ref PkgRef, now time.Time, lim depLimits, budgetBytes int) (IngestDepDocsResult, error) {
	res := IngestDepDocsResult{Key: ref.Key()}
	files, rep, err := depDocUnitsLimited(ctx, loc, ref, lim)
	if err != nil {
		return res, err
	}
	res.Report = rep
	res.FilesScanned = len(files)

	// One budget over the whole package, in KR-9's priority order.
	var candidates []store.DocUnit
	for _, f := range files {
		candidates = append(candidates, f.Units...)
	}
	fit := enforceDocBudget(candidates, budgetBytes)
	res.Elided = fit.Elided
	kept := make(map[string][]store.DocUnit, len(files))
	for _, u := range fit.Kept {
		kept[u.Path] = append(kept[u.Path], u)
	}

	key := ref.Key()
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		units := kept[f.Path]
		if len(units) == 0 {
			continue // every unit of this file was elided: as if absent
		}
		seen[f.Path] = true
		// Restore file order (the budget reorders by heading, not by line).
		ordered := make([]store.DocUnit, 0, len(units))
		byID := make(map[string]store.DocUnit, len(units))
		for _, u := range units {
			byID[u.UnitID] = u
		}
		for _, u := range f.Units {
			if k, ok := byID[u.UnitID]; ok {
				ordered = append(ordered, k)
			}
		}
		fp := depFileFingerprint(ordered)
		stored, found, err := st.Fingerprint(ctx, key, depFingerprintPrefix+f.Path)
		if err != nil {
			return res, fmt.Errorf("IngestDepDocs: fingerprint %s: %w", f.Path, err)
		}
		if found && stored == fp {
			res.FilesUnchanged++
			continue
		}
		for i := range ordered {
			ordered[i].IndexedAt = now.UnixNano()
		}
		if err := st.PutDocUnits(ctx, key, f.Path, ordered); err != nil {
			return res, fmt.Errorf("IngestDepDocs: PutDocUnits %s: %w", f.Path, err)
		}
		if err := st.SetFingerprint(ctx, key, depFingerprintPrefix+f.Path, fp); err != nil {
			return res, fmt.Errorf("IngestDepDocs: SetFingerprint %s: %w", f.Path, err)
		}
		res.FilesChanged++
		res.UnitsWritten += len(ordered)
	}

	indexed, err := st.DocIndexedPaths(ctx, key)
	if err != nil {
		return res, fmt.Errorf("IngestDepDocs: DocIndexedPaths: %w", err)
	}
	for _, p := range indexed {
		if seen[p] {
			continue
		}
		if err := st.DeleteDocUnitsForPath(ctx, key, p); err != nil {
			return res, fmt.Errorf("IngestDepDocs: DeleteDocUnitsForPath %s: %w", p, err)
		}
		_ = st.DeleteFingerprint(ctx, key, depFingerprintPrefix+p)
		res.FilesDeleted++
	}

	raw, err := json.Marshal(depIndexReport{Elided: len(fit.Elided), Skipped: len(rep.Skipped), Partial: rep.Partial})
	if err != nil {
		return res, fmt.Errorf("IngestDepDocs: report: %w", err)
	}
	if err := st.SetFingerprint(ctx, key, depReportRef, string(raw)); err != nil {
		return res, fmt.Errorf("IngestDepDocs: SetFingerprint report: %w", err)
	}
	return res, nil
}

// depFileFingerprint hashes what would be stored for one file (heading,
// kind, position and content of each unit), so a changed budget or reader
// re-writes the file even when the source bytes are the same.
func depFileFingerprint(units []store.DocUnit) string {
	h := sha256.New()
	for _, u := range units {
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00%d\x00%s\x00", u.HeadingPath, u.Kind, u.LineStart, u.LineEnd, u.ContentHash)
	}
	return hex.EncodeToString(h.Sum(nil))
}
