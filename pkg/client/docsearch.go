package client

// docsearch.go — Client.SearchDocs (KR-5, docs/design/knowledge-retrieval.md
// §5): the pkg/client half of `search kind: docs`'s optional backend
// capability (internal/mcp's docsSearchBackend), following the same
// straight-delegate-to-a-package-function-against-c.st shape
// Obligations/ReviewObligations (pkg/client/semantic.go) already use.

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/docindex"
)

// SearchDocs answers one `search kind: docs` query: docindex.RankDocs
// against this client's own store. Returns docindex.ErrDocIndexUnavailable
// when the underlying store is not a *store.SQLiteStore (e.g. a
// MemoryStore-backed session) — the caller (internal/mcp's search tool)
// turns that into a clear refusal rather than reporting zero hits, which
// would be indistinguishable from a real "nothing matched" (I38).
func (c *Client) SearchDocs(ctx context.Context, service, query string, concernSymbols []string, limit int) ([]docindex.RankedDoc, error) {
	return docindex.RankDocs(ctx, c.st, service, query, concernSymbols, limit)
}

// SearchPkgDocs answers one `search kind: docs` query scoped to a `pkg:` key
// (KR-12, docs/design/knowledge-retrieval.md §7): docindex.SearchPkgDocs
// against this client's own store — the pkg/client half of internal/mcp's
// pkgDocsSearchBackend. A malformed key is docindex.ErrBadPkgKey and a
// non-SQLite store docindex.ErrDocIndexUnavailable; the search tool turns
// each into its own refusal.
func (c *Client) SearchPkgDocs(ctx context.Context, pkgKey, query string, limit int) (docindex.PkgDocsResult, error) {
	return docindex.SearchPkgDocs(ctx, c.st, pkgKey, query, limit)
}
