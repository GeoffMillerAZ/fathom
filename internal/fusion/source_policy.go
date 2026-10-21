package fusion

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Only the concrete LocalConnector has a complete discovery-policy receipt.
// Wrappers and other connectors may alter Discover semantics, so they ingest
// normally but cannot publish a sealed reference projection.
func sourcePolicyDigest(ctx context.Context, c connector.Connector, svc core.ServiceRef) string {
	provider, ok := c.(*connector.LocalConnector)
	if !ok {
		return ""
	}
	digest, err := provider.DiscoveryPolicyDigest(ctx, svc)
	if err != nil {
		return ""
	}
	return digest
}

func sourceMembershipStable(ctx context.Context, c connector.Connector, svc core.ServiceRef, initial []core.SourceRef) bool {
	now, err := c.Discover(ctx, svc)
	if err != nil || len(now) != len(initial) {
		return false
	}
	for i := range initial {
		if now[i] != initial[i] {
			return false
		}
	}
	return true
}
