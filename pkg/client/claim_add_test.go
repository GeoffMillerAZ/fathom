package client_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestAddClaim_MintsSMEStatedTierCappedClaim(t *testing.T) {
	conn := connector.NewFauxConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })

	res, err := c.AddClaim(context.Background(), client.AddClaimRequest{
		Service: "svc", Scope: []string{"internal/foo/**"}, Rule: "Do not log secrets in plaintext.",
	})
	if err != nil {
		t.Fatalf("AddClaim: %v", err)
	}
	if res.Claim.Tier != core.TierSMEStated {
		t.Errorf("Tier = %q, want sme-stated", res.Claim.Tier)
	}
	if res.Claim.PromptVersion != client.HumanAuthoredPromptVersion {
		t.Errorf("PromptVersion = %q, want %q", res.Claim.PromptVersion, client.HumanAuthoredPromptVersion)
	}
	if res.Claim.Layer != "findings" || res.Claim.Kind != "finding" {
		t.Errorf("Layer/Kind = %q/%q, want findings/finding", res.Claim.Layer, res.Claim.Kind)
	}

	claims, err := ms.ClaimsForService(context.Background(), "svc", "findings")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v / %d", err, len(claims))
	}
}

func TestAddClaim_RequiredFields(t *testing.T) {
	conn := connector.NewFauxConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()

	if _, err := c.AddClaim(ctx, client.AddClaimRequest{Scope: []string{"a.go"}, Rule: "x"}); err == nil {
		t.Error("expected an error with no Service")
	}
	if _, err := c.AddClaim(ctx, client.AddClaimRequest{Service: "svc", Rule: "x"}); err == nil {
		t.Error("expected an error with no Scope")
	}
	if _, err := c.AddClaim(ctx, client.AddClaimRequest{Service: "svc", Scope: []string{"a.go"}}); err == nil {
		t.Error("expected an error with no Rule")
	}
}

func TestAddClaim_MalformedCiteRef(t *testing.T) {
	conn := connector.NewFauxConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })

	_, err := c.AddClaim(context.Background(), client.AddClaimRequest{
		Service: "svc", Scope: []string{"a.go"}, Rule: "x", CiteRef: "not-a-citeref",
	})
	if err == nil || !strings.Contains(err.Error(), "malformed --cite") {
		t.Errorf("err = %v, want a malformed --cite refusal", err)
	}
}

// TestAddClaim_OwnReconciliationDomain_NativeReviewNeverTouchesIt is SU-17's
// own-domain proof: `review.StaleReviewClaims` — the mechanism native
// `fathom review --mode persist` uses to supersede claims a re-run no
// longer emits — is scoped to claims whose Layer+PromptVersion match its
// OWN bridge ("findings"/"review-verb/v1", claims.go's ClaimsLayer/
// ClaimsPromptVersion). A human-authored claim (PromptVersion
// "human-authored/v1") sharing the exact same layer, scope and changed-file
// set as a review run that emitted NOTHING for that file must still never
// be superseded by it.
func TestAddClaim_OwnReconciliationDomain_NativeReviewNeverTouchesIt(t *testing.T) {
	conn := connector.NewFauxConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })

	res, err := c.AddClaim(context.Background(), client.AddClaimRequest{
		Service: "svc", Scope: []string{"internal/foo/bar.go"}, Rule: "Always check the return value.",
	})
	if err != nil {
		t.Fatalf("AddClaim: %v", err)
	}

	// A native review run over the SAME file that emitted ZERO arch_conformance
	// findings this time — the exact shape that would supersede a
	// review-minted claim scoped to that file (StaleReviewClaims' own
	// doc comment), if the claim were in its domain.
	v := review.Verdict{
		Service:      "svc",
		ChangedFiles: []string{"internal/foo/bar.go"},
		Dimensions: []review.Dimension{
			{Name: review.DimArch, Checked: true, Findings: nil},
		},
	}
	stale := review.StaleReviewClaims(v, []core.SemanticClaim{res.Claim})
	for _, id := range stale {
		if id == res.ClaimID {
			t.Fatalf("native review's StaleReviewClaims superseded a human-authored claim (PromptVersion %q) — the domains must stay separate", res.Claim.PromptVersion)
		}
	}
}
