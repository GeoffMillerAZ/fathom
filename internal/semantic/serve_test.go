package semantic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func seedServed(t *testing.T, st store.Store, id, rule string, scope []string, status core.ClaimStatus) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: LayerObligations, LayerVersion: "v1", Kind: KindObligation,
		Subject: "docs/g.md#s-" + id, Rule: rule, Scope: scope,
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
		PromptVersion: PromptVersion, ReproKey: id, InputDigest: "d",
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: "docs/g.md#L1-L2", TargetHash: "h"}}
	for _, g := range scope {
		links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: g})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestObligationsFor_ScopedOrderedFlagged(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	seedServed(t, st, "c-broad", "Sort all outputs.", []string{"**"}, core.ClaimFresh)
	seedServed(t, st, "c-narrow", "Inject clocks in query logic.", []string{"internal/query/**"}, core.ClaimFresh)
	seedServed(t, st, "c-stale", "Old rule that drifted.", []string{"internal/query/**"}, core.ClaimStale)
	seedServed(t, st, "c-other", "Rule for the mcp package.", []string{"internal/mcp/**"}, core.ClaimFresh)
	seedServed(t, st, "c-dead", "Refuted rule.", []string{"**"}, core.ClaimRefuted)

	got, err := ObligationsFor(ctx, st, "svc", "internal/query/engine.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != 3 {
		t.Fatalf("got %d rules, want 3 (narrow, stale-narrow, broad; refuted+mcp excluded): %+v", len(got.Rules), got.Rules)
	}
	// Narrow scope beats broad; ties break on rule text.
	if got.Rules[0].Rule != "Inject clocks in query logic." || got.Rules[1].Rule != "Old rule that drifted." {
		t.Errorf("order wrong: %+v", got.Rules)
	}
	if got.Rules[2].Rule != "Sort all outputs." {
		t.Errorf("broad rule not last: %+v", got.Rules)
	}
	// Drifted rule is served FLAGGED, never dropped.
	if got.Rules[1].Status != string(core.ClaimStale) {
		t.Errorf("stale rule status = %q, want flagged stale", got.Rules[1].Status)
	}
	if got.Rules[0].Status != "" {
		t.Errorf("fresh rule must omit status, got %q", got.Rules[0].Status)
	}
	if got.Rules[0].Src == "" || got.Rules[0].Tier != string(core.TierDocClaimed) {
		t.Errorf("citation/tier missing: %+v", got.Rules[0])
	}
	if got.Watermark != "manifest:aaa" {
		t.Errorf("watermark = %q", got.Watermark)
	}
}

func TestObligationsFor_ZeroOutputOnClean(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedServed(t, st, "c-mcp", "Rule for the mcp package.", []string{"internal/mcp/**"}, core.ClaimFresh)

	got, err := ObligationsFor(ctx, st, "svc", "cmd/fathom/main.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != 0 {
		t.Fatalf("clean file got %d rules", len(got.Rules))
	}
	// The JSON a hook would emit must carry no rules key at all.
	b, _ := json.Marshal(got)
	if string(b) != `{"service":"svc","file":"cmd/fathom/main.go"}` {
		t.Errorf("clean payload = %s", b)
	}
}

func TestObligationsFor_CapsAtMax(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	rules := []string{"A.", "B.", "C.", "D.", "E.", "F.", "G.", "H."}
	for i, r := range rules {
		seedServed(t, st, "c-"+r, r, []string{"**"}, core.ClaimFresh)
		_ = i
	}
	got, err := ObligationsFor(ctx, st, "svc", "any.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != MaxObligations {
		t.Errorf("got %d rules, want cap %d", len(got.Rules), MaxObligations)
	}
}
