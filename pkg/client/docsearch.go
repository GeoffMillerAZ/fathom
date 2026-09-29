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

// RankedDoc is one `search kind: docs` hit: the unit, its line range and
// heading path, whether its location marks it stale, a snippet, and the
// fused score. It is an alias of the internal result type so a caller
// outside this module can name what SearchDocs returns.
type RankedDoc = docindex.RankedDoc

// ErrDocIndexUnavailable is what SearchDocs and DocLinkDrift return when
// the client's store carries no doc index (a store that is not SQLite
// backed). It is the same value the internal package returns, so errors.Is
// works against either name.
var ErrDocIndexUnavailable = docindex.ErrDocIndexUnavailable

// SearchDocs answers one `search kind: docs` query: docindex.RankDocs
// against this client's own store. Returns docindex.ErrDocIndexUnavailable
// when the underlying store is not a *store.SQLiteStore (e.g. a
// MemoryStore-backed session) — the caller (internal/mcp's search tool)
// turns that into a clear refusal rather than reporting zero hits, which
// would be indistinguishable from a real "nothing matched" (I38).
func (c *Client) SearchDocs(ctx context.Context, service, query string, concernSymbols []string, limit int) ([]RankedDoc, error) {
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
