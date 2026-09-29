package fusion

// docindexpass.go — the production caller of docindex.IngestDocUnits (KR-8,
// docs/design/knowledge-retrieval.md §4) and, through it, of the per-service
// byte budget (KR-9, §8). Until this pass existed nothing called the doc
// ingest, so `search kind: docs` read an empty index.
//
// It runs at the same ingest as the code passes, over the same discovered
// set: the corpus is the Markdown Connector.Discover returned, so a document
// the code index was told to leave out (.fathomignore, the scoped
// exclusions purge writes, default-ignored caches) is never indexed for
// search either. The pass is additive like the typed-edge and doc-xref
// passes: whatever goes wrong is named in Report.DocIndexSkipped and never
// fails or unwinds the code ingest.

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Why the doc-unit pass did not run or did not finish (Report.DocIndexSkipped).
const (
	docIndexSkipNoRoot  = "no local corpus root (the service's path is not a directory on this machine, as for a remote or fixture service)"
	docIndexSkipNoIndex = "the store has no doc index (docs_fts is SQLite-only)"
)

// indexDocs runs the doc-unit pass for svc over refs (this ingest's
// discovered set) and records its outcome on report.
func (p *Pipeline) indexDocs(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, report *Report) {
	root := absCorpusPath(svc.Path)
	if root == "" {
		report.DocIndexSkipped = docIndexSkipNoRoot
		return
	}
	sq, ok := p.st.(*store.SQLiteStore)
	if !ok {
		report.DocIndexSkipped = docIndexSkipNoIndex
		return
	}
	// The connector walks a root that is itself a symlink through its
	// target (O39); the doc corpus walk does not follow one.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		report.DocIndexSkipped = docIndexSkipNoRoot
		return
	}
	discovered := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		discovered[r.Path] = struct{}{}
	}
	res, err := docindex.IngestDocUnits(ctx, sq, svc.ID, root, time.Now(),
		docindex.WithCorpusFilter(func(path string) bool {
			_, ok := discovered[path]
			return ok
		}))
	report.DocIndex = res
	if err != nil {
		report.DocIndexSkipped = "doc index not finished: " + err.Error()
	}
}
