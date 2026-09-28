package client

// docdrift.go — Client.DocLinkDrift (KR-7, docs/design/knowledge-retrieval.md
// §6): the pkg/client half of staleness_report's doc-link-drift capability
// (internal/mcp's docDriftBackend), following docsearch.go's own
// straight-delegate-to-a-package-function-against-c.st shape.

import (
	"context"
	"time"

	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/store"
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

// DocLinksAsOf answers which doc links held at at (GL-16/GL-17,
// store.SQLiteStore.DocLinksAsOf): staleness_report's dated doc-link
// section. Like DocLinkDrift it returns docindex.ErrDocIndexUnavailable
// when the store is not a *store.SQLiteStore, which the caller treats as
// "no doc-link signal", never as "no links held" (I38). The store stamps
// link windows in UnixNano, the ingest convention.
func (c *Client) DocLinksAsOf(ctx context.Context, service string, at time.Time) ([]store.DocLinkView, error) {
	sq, ok := c.st.(*store.SQLiteStore)
	if !ok {
		return nil, docindex.ErrDocIndexUnavailable
	}
	return sq.DocLinksAsOf(ctx, service, at.UnixNano())
}
