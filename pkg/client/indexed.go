package client

import (
	"context"
	"fmt"
	"strings"
)

// IsIndexed reports whether service has been indexed: a service_meta row
// with a non-zero indexed_at AND at least one symbol (RT-80,
// docs/design/responsible-tenant.md §4.4). The symbol half keeps the
// meaning the list_services probe it replaces had (symbol_count > 0):
// verify's exit-2 fail-open and init's "no symbols in the index yet" line
// rely on it, and an empty-tree ingest or a purge leaves a meta row with
// nothing to verify against. The count comes from ServiceSymbolAggregates —
// one aggregate query per service per watermark, cached and
// single-flighted — never a symbol or observation load. An exact id is
// tried first; a case-insensitive match against the stored service ids
// follows only on a miss. resolved is the stored id that answered ("" when
// none did).
func (c *Client) IsIndexed(ctx context.Context, service string) (indexed bool, resolved string, err error) {
	meta, found, err := c.st.GetServiceMeta(ctx, service)
	if err != nil {
		return false, "", fmt.Errorf("client.IsIndexed: %w", err)
	}
	if !found {
		ids, lerr := c.st.ListServiceIDs(ctx)
		if lerr != nil {
			return false, "", fmt.Errorf("client.IsIndexed: %w", lerr)
		}
		for _, id := range ids {
			if !strings.EqualFold(id, service) {
				continue
			}
			meta, found, err = c.st.GetServiceMeta(ctx, id)
			if err != nil {
				return false, "", fmt.Errorf("client.IsIndexed: %w", err)
			}
			if found {
				service = id
				break
			}
		}
		if !found {
			return false, "", nil
		}
	}
	if meta.IndexedAt <= 0 {
		return false, service, nil
	}
	agg, err := c.ServiceSymbolAggregates(ctx, service)
	if err != nil {
		return false, "", fmt.Errorf("client.IsIndexed: %w", err)
	}
	return agg.Count > 0, service, nil
}
