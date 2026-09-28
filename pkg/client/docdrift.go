package client

// docdrift.go — Client.DocLinkDrift (KR-7, docs/design/knowledge-retrieval.md
// §6): the pkg/client half of staleness_report's doc-link-drift capability
// (internal/mcp's docDriftBackend), following docsearch.go's own
// straight-delegate-to-a-package-function-against-c.st shape.

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/docindex"
)

// DocLinkDrift answers which of this client's service's open doc_links
// (KR-6) have drifted: docindex.StalenessDocDrift against this client's
// own store. Returns docindex.ErrDocIndexUnavailable when the underlying
// store is not a *store.SQLiteStore (e.g. a MemoryStore-backed session) —
// the caller (internal/mcp's staleness_report handler) treats that as "no
// doc-link-drift signal available", never as "nothing drifted" (I38).
func (c *Client) DocLinkDrift(ctx context.Context, service string) ([]docindex.DriftedDocLink, error) {
	return docindex.StalenessDocDrift(ctx, c.st, service)
}
