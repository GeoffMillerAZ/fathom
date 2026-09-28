package docindex

// ingest.go — IngestDocUnits (KR-8, docs/design/knowledge-retrieval.md §4:
// "Units are written at ingest when a doc file's content hash changes
// (hash-gated, P19); a deleted file's units are removed in the same
// transaction"). Wires docCorpusFiles (KR-3) and SplitDocUnits (KR-2) into
// store.PutDocUnits (KR-1).
//
// SQLite-only, like the store half it drives (docindex.go's doc comment):
// takes *store.SQLiteStore directly rather than the generic store.Store
// interface, since PutDocUnits/DeleteDocUnitsForPath/DocIndexedPaths only
// exist on that concrete type.
//
// Per-file hash-gating reuses the store's existing generic
// Fingerprint/SetFingerprint (keyed by service+sourceRef), under a
// "docindex:"+path sourceRef namespace — the same "distinguish a second
// fingerprint use by prefixing sourceRef" convention internal/semantic/
// eval.go already uses ("eval-fixture:"+probe) — so this never collides
// with, or is clobbered by, the plain-path fingerprint the fusion pipeline
// keeps for its own (unrelated) skip-if-unchanged check on the same files.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// docFingerprintPrefix namespaces this package's per-file content-hash gate
// within the store's shared (serviceID, sourceRef) Fingerprint table.
const docFingerprintPrefix = "docindex:"

// IngestDocUnitsResult summarizes one IngestDocUnits call — what it did,
// not what it found (that lives in the store afterward, queryable via
// QueryDocUnits/DocUnitsForPath).
type IngestDocUnitsResult struct {
	FilesScanned   int // every corpus file docCorpusFiles found
	FilesChanged   int // re-split and re-written (new file or changed content hash)
	FilesUnchanged int // content hash matched the stored fingerprint; skipped
	FilesDeleted   int // previously indexed, no longer in the corpus; units removed
	UnitsWritten   int // total units written across every changed file
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

// IngestDocUnits walks root's corpus (docCorpusFiles), splits and writes
// every file whose content changed since the last call (SetFingerprint
// under a "docindex:"+path key), and removes a previously indexed file's
// units when the corpus no longer contains it — all per-file, in doc_units'
// own (service_id, path) granularity (§4), never a whole-service replace
// (§8's cost table: "a docs-only change re-indexes only changed units").
//
// now stamps every written unit's IndexedAt; the caller supplies it
// (typically time.Now()) rather than this function calling the clock
// itself, so a test can pin it.
func IngestDocUnits(ctx context.Context, st *store.SQLiteStore, serviceID, root string, now time.Time) (IngestDocUnitsResult, error) {
	var res IngestDocUnitsResult

	corpus, err := docCorpusFiles(root)
	if err != nil {
		return res, fmt.Errorf("IngestDocUnits: corpus walk: %w", err)
	}
	res.FilesScanned = len(corpus)

	seen := make(map[string]bool, len(corpus))
	for _, cf := range corpus {
		seen[cf.Path] = true
		fpKey := docFingerprintPrefix + cf.Path

		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cf.Path)))
		if err != nil {
			return res, fmt.Errorf("IngestDocUnits: read %s: %w", cf.Path, err)
		}
		contentHash := sha256Hex(raw)

		stored, found, err := st.Fingerprint(ctx, serviceID, fpKey)
		if err != nil {
			return res, fmt.Errorf("IngestDocUnits: fingerprint %s: %w", cf.Path, err)
		}
		if found && stored == contentHash {
			res.FilesUnchanged++
			continue
		}

		units := SplitDocUnits(cf.Kind, string(raw))
		storeUnits := make([]store.DocUnit, 0, len(units))
		for _, u := range units {
			storeUnits = append(storeUnits, store.DocUnit{
				UnitID:          unitID(cf.Path, u.HeadingPath, u.Kind),
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
		if err := st.PutDocUnits(ctx, serviceID, cf.Path, storeUnits); err != nil {
			return res, fmt.Errorf("IngestDocUnits: PutDocUnits %s: %w", cf.Path, err)
		}
		if err := st.SetFingerprint(ctx, serviceID, fpKey, contentHash); err != nil {
			return res, fmt.Errorf("IngestDocUnits: SetFingerprint %s: %w", cf.Path, err)
		}
		res.FilesChanged++
		res.UnitsWritten += len(storeUnits)
	}

	indexed, err := st.DocIndexedPaths(ctx, serviceID)
	if err != nil {
		return res, fmt.Errorf("IngestDocUnits: DocIndexedPaths: %w", err)
	}
	for _, path := range indexed {
		if seen[path] {
			continue
		}
		if err := st.DeleteDocUnitsForPath(ctx, serviceID, path); err != nil {
			return res, fmt.Errorf("IngestDocUnits: DeleteDocUnitsForPath %s: %w", path, err)
		}
		_ = st.DeleteFingerprint(ctx, serviceID, docFingerprintPrefix+path)
		res.FilesDeleted++
	}

	return res, nil
}
