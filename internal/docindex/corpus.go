package docindex

// corpus.go — the doc corpus (KR-3, docs/design/knowledge-retrieval.md §3).
// docCorpusFiles enumerates the Markdown files the knowledge-retrieval
// ingest (KR-8) reads and classifies each by DocKind so SplitDocUnits (KR-2)
// applies the right rule. It runs at INGEST time only (called from the same
// place file discovery already walks the whole repository for the fusion
// pipeline), never from a query-serving path — the tenancy lint's
// "no whole-tree reads on request paths" rule governs request handlers, not
// ingest, which has always walked the full tree (ADR-005).
//
// Memories and symbol doc comments (§3's last two corpus rows) are NOT
// files this walk can find — the ingest layer sources those from the
// store's memory rows and from already-extracted observations respectively.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/fsfault"
)

// CorpusFile is one Markdown source docCorpusFiles found under a service
// root: its repo-relative, slash-form path, which DocKind rule to split it
// with, and whether §3's stale_by_location flag applies (docs/plans/archive/
// and any tmp/ path — indexed, ranked below live docs, never dropped).
type CorpusFile struct {
	Path            string
	Kind            DocKind
	StaleByLocation bool
}

// corpusExtraSkipDirs are directory names docCorpusFiles always prunes on
// top of connector.IsSkippedDir's ingest-time set: "workspace" is this
// repo's own gitignored build-artifact convention (docs/usage.md) and is
// never part of the indexed corpus, the same stance search_tool.go's own
// searchExtraSkipDirs takes for the same reason. "third_party" is §3's
// "vendored paths" in its conventional spelling (the connector's own
// "vendor" covers Go's): 622 of this repository's 998 documents are a
// vendored kit's, and indexed they outnumber and outrank the service's own
// words. Unlike search's list, this one does NOT add "tmp" or "archive" —
// §3 requires those to be indexed with the stale_by_location flag, not
// excluded.
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var corpusExtraSkipDirs = map[string]bool{"workspace": true, "third_party": true}

func corpusSkipDir(name string) bool {
	return connector.IsSkippedDir(name) || corpusExtraSkipDirs[name]
}

// classifyDocPath returns the DocKind and stale_by_location flag for a
// repo-relative, slash-form Markdown path, per §3's table.
func classifyDocPath(rel string) (DocKind, bool) {
	stale := false
	for _, seg := range strings.Split(rel, "/") {
		if seg == "archive" || seg == "tmp" {
			stale = true
			break
		}
	}
	switch {
	case rel == "ADR.md":
		return DocKindADR, stale
	case strings.HasPrefix(rel, "docs/intent/"):
		return DocKindIntent, stale
	case strings.HasPrefix(rel, "docs/plans/"):
		return DocKindPlan, stale
	default:
		return DocKindOther, stale
	}
}

// docCorpusFiles walks root and returns every Markdown file §3's corpus
// table names, sorted by path (I2/determinism: a directory walk order is
// not itself a promise, so the caller never sees an unsorted result). A
// symlink is not followed (matches connector's own walk convention); a
// directory this process cannot read is reported as an error, never
// silently skipped (I38: an unreadable input is never a healthy zero).
func docCorpusFiles(root string) ([]CorpusFile, error) {
	var out []CorpusFile
	// fsfault hands a test's denied directory to the err branch below as
	// Walk hands a real refusal — whoever runs the test; root lists a
	// mode-000 directory (CS-8).
	err := filepath.Walk(root, fsfault.WalkFunc(func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if rel != "." && corpusSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if !strings.HasSuffix(rel, ".md") {
			return nil
		}
		kind, stale := classifyDocPath(rel)
		out = append(out, CorpusFile{Path: rel, Kind: kind, StaleByLocation: stale})
		return nil
	}))
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
