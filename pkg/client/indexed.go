package client

import (
	"context"
	"fmt"
	"strings"
)

// IsIndexed reports whether service has been indexed: a service_meta row
// with a non-zero indexed_at (RT-80, docs/design/responsible-tenant.md
// §4.4). It is the yes/no probe verify, the stop and edit hooks, plans and
// the console ask — one meta row read, never a symbol or observation load.
// An exact id is tried first; a case-insensitive match against the stored
// service ids follows only on a miss (the list_services probe it replaces
// matched case-insensitively). resolved is the stored id that answered
// ("" when none did).
func (c *Client) IsIndexed(ctx context.Context, service string) (indexed bool, resolved string, err error) {
	meta, found, err := c.st.GetServiceMeta(ctx, service)
	if err != nil {
		return false, "", fmt.Errorf("client.IsIndexed: %w", err)
	}
	if found {
		return meta.IndexedAt > 0, service, nil
	}
	ids, err := c.st.ListServiceIDs(ctx)
	if err != nil {
		return false, "", fmt.Errorf("client.IsIndexed: %w", err)
	}
	for _, id := range ids {
		if !strings.EqualFold(id, service) {
			continue
		}
		meta, found, err := c.st.GetServiceMeta(ctx, id)
		if err != nil {
			return false, "", fmt.Errorf("client.IsIndexed: %w", err)
		}
		if found {
			return meta.IndexedAt > 0, id, nil
		}
	}
	return false, "", nil
}
