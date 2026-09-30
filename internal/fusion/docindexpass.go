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
//
// The link pass (KR-6) rides the same call: each (re)indexed unit's spans
// are linked to the code this ingest indexed (docindex.LinkUnits with a
// store-backed resolver) and synced into doc_links, and Report.DocIndex.
// LinksWritten counts the links those units hold.

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
	// KR-6: every unit this ingest (re)indexes is linked right after it is
	// written — its backticked symbols, paths and contract keys resolved
	// exactly against the index this ingest just wrote, with the target's
	// hash at link time — and a removed unit's links close. An unchanged
	// unit is not re-linked, so its link-time hashes stay what KR-7's
	// drift compares against.
	now := time.Now()
	resolver := docindex.NewStoreResolver(ctx, sq, svc.ID)
	links := 0
	res, err := docindex.IngestDocUnits(ctx, sq, svc.ID, root, now,
		docindex.WithCorpusFilter(func(path string) bool {
			_, ok := discovered[path]
			return ok
		}),
		docindex.WithOnWrite(func(ctx context.Context, _ string, written []store.DocUnit, removed []string) error {
			n, lerr := docindex.LinkUnits(ctx, sq, svc.ID, written, removed, resolver, now.UnixNano())
			links += n
			return lerr
		}))
	res.LinksWritten = links
	report.DocIndex = res
	if err != nil {
		report.DocIndexSkipped = "doc index not finished: " + err.Error()
	}
}
