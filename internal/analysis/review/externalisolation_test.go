package review_test

// externalisolation_test.go — plan FL-12 (vision 30 §3.3 item 2, ADR-040
// decision 3): native review reconciliation (StaleReviewClaims) must never
// touch a claim the findings-loop external importer wrote
// (PromptVersion "external-import/v1") — the two are distinct
// reconciliation domains, discriminated by PromptVersion. StaleReviewClaims
// already filters `cl.PromptVersion != ClaimsPromptVersion` (review's own
// "review-verb/v1"), which structurally excludes an external-import claim
// without any code change; this test proves it with BOTH populations
// seeded into one store, as the row's own annotation asks for.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestStaleReviewClaims_IgnoresExternalImportClaims(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	const svc = "svc-fl12"
	const changedFile = "api/handler.go"

	// Native review-minted claim on the changed file: eligible for
	// reconciliation once its issue no longer reproduces.
	native := core.SemanticClaim{
		ClaimID: "native-arch-claim", ServiceID: svc,
		Layer: review.ClaimsLayer, PromptVersion: review.ClaimsPromptVersion,
		Subject: review.DimArch + "#api.Handler", Scope: []string{changedFile},
		Status: core.ClaimFresh, Tier: core.TierVerified, TierProfile: "engineering",
	}
	// External-import claim on the SAME file, from the findings-loop
	// importer — must never be touched by review's own reconcile, however
	// its scope or subject happen to overlap.
	external := core.SemanticClaim{
		ClaimID: "ext-claim-1", ServiceID: svc,
		Layer: "findings", PromptVersion: "external-import/v1",
		Subject: "ext:reviewkit.system-review/acme/widget/api/rule-1", Scope: []string{changedFile},
		Status: core.ClaimFresh, Tier: core.TierPreScan, TierProfile: "engineering",
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{native, external}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	existing, err := st.ClaimsForService(ctx, svc, "")
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(existing) != 2 {
		t.Fatalf("seeded %d claims, want 2", len(existing))
	}

	// A review run over the same changed file whose arch dimension is
	// CHECKED and CLEAN (no violation emitted this run) — the condition
	// under which StaleReviewClaims supersedes a fixed native finding.
	v := review.Run(review.Input{
		Service:      svc,
		ChangedFiles: []string{changedFile},
		Nodes:        fixtureNodes(),
		ArchChecked:  true,
	})

	stale := review.StaleReviewClaims(v, existing)
	if len(stale) != 1 || stale[0] != native.ClaimID {
		t.Fatalf("StaleReviewClaims = %v, want exactly [%q] (the native claim only — the external-import claim must never be superseded by review's own reconcile)", stale, native.ClaimID)
	}
}
