package connector_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Opt-in, read-only probe for the production corpus. It does not ingest or
// modify files. The digest's error reports a policy-identity refusal.
func TestDiscoveryPolicyProductionProbe(t *testing.T) {
	root := os.Getenv("FATHOM_POLICY_PROBE_ROOT")
	if root == "" {
		t.Skip("set FATHOM_POLICY_PROBE_ROOT to enable")
	}
	ctx := context.Background()
	svc := core.ServiceRef{ID: "policy-probe", Path: root}
	c := connector.NewLocalConnector()
	start := time.Now()
	refs, err := c.Discover(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("discovered=%d discover_elapsed=%s exclusions=%+v", len(refs), time.Since(start), c.Exclusions(svc.ID))
	start = time.Now()
	digest, err := c.DiscoveryPolicyDigest(ctx, svc)
	t.Logf("policy_digest=%q policy_elapsed=%s policy_error=%v", digest, time.Since(start), err)
	if err != nil || digest == "" {
		t.Fatalf("production policy identity unavailable: %v", err)
	}
}
