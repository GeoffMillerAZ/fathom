package semantic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestObligationsAsOf_ServesWhatWasValidThen (SU-19, ADR-041 decision 7): a
// claim later superseded still answers "what governed this file at asOf"
// when asOf falls INSIDE its [ValidFrom, ValidTo) window, even though its
// CURRENT status (superseded) would exclude it from live serving entirely.
// It reads through ObligationsAsOfOpts, the function the product runs
// (pkg/client's ObligationsAsOf, behind the obligations tool's as_of), and
// pins ObligationsAsOf to the same payload.
func TestObligationsAsOf_ServesWhatWasValidThen(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	mintedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	supersededAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	claim := core.SemanticClaim{
		ClaimID: "c-old", ServiceID: "svc",
		Layer: LayerObligations, LayerVersion: "v1", Kind: KindObligation,
		Subject: "docs/g.md#s-old", Rule: "Old rule.", Scope: []string{"**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh,
		PromptVersion: obligationsPromptVersion(), ReproKey: "c-old", InputDigest: "d",
		CompiledAtWatermark: "manifest:aaa", CompiledAt: mintedAt,
		ValidFrom: mintedAt,
	}
	links := []core.ClaimLink{
		{ClaimID: "c-old", Rel: core.RelCites, TargetRef: "docs/g.md#L1-L2", TargetHash: "h"},
		{ClaimID: "c-old", Rel: core.RelScopes, TargetRef: "**"},
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Retire it — live serving must never see it again.
	if err := st.SetClaimStatus(ctx, "svc", "c-old", core.ClaimSuperseded); err != nil {
		t.Fatalf("SetClaimStatus: %v", err)
	}
	if err := st.SetClaimValidTo(ctx, "svc", "c-old", supersededAt); err != nil {
		t.Fatalf("SetClaimValidTo: %v", err)
	}

	live, err := ObligationsFor(ctx, st, "svc", "any/file.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(live.Rules) != 0 {
		t.Fatalf("live ObligationsFor must not serve a superseded claim, got %+v", live.Rules)
	}

	inWindow := mintedAt.Add(30 * 24 * time.Hour)
	asOfGot, err := ObligationsAsOfOpts(ctx, st, "svc", "any/file.go", 0, inWindow, ObligationsOptions{})
	if err != nil {
		t.Fatalf("ObligationsAsOfOpts: %v", err)
	}
	if len(asOfGot.Rules) != 1 || asOfGot.Rules[0].Rule != "Old rule." {
		t.Fatalf("ObligationsAsOfOpts(in-window) = %+v, want the superseded-now claim served", asOfGot.Rules)
	}
	if asOfGot.AsOf != inWindow.UTC().Format(time.RFC3339) {
		t.Errorf("AsOf = %q, want %q", asOfGot.AsOf, inWindow.UTC().Format(time.RFC3339))
	}
	plain, err := ObligationsAsOf(ctx, st, "svc", "any/file.go", 0, inWindow)
	if err != nil {
		t.Fatalf("ObligationsAsOf: %v", err)
	}
	optsJSON, _ := json.Marshal(asOfGot)
	plainJSON, _ := json.Marshal(plain)
	if string(optsJSON) != string(plainJSON) {
		t.Errorf("ObligationsAsOfOpts with zero options = %s, ObligationsAsOf = %s; want byte-identical", optsJSON, plainJSON)
	}

	afterWindow := supersededAt.Add(time.Hour)
	asOfAfter, err := ObligationsAsOfOpts(ctx, st, "svc", "any/file.go", 0, afterWindow, ObligationsOptions{})
	if err != nil {
		t.Fatalf("ObligationsAsOfOpts: %v", err)
	}
	if len(asOfAfter.Rules) != 0 {
		t.Fatalf("ObligationsAsOfOpts(after supersede) = %+v, want empty", asOfAfter.Rules)
	}

	beforeMint := mintedAt.Add(-time.Hour)
	asOfBefore, err := ObligationsAsOfOpts(ctx, st, "svc", "any/file.go", 0, beforeMint, ObligationsOptions{})
	if err != nil {
		t.Fatalf("ObligationsAsOfOpts: %v", err)
	}
	if len(asOfBefore.Rules) != 0 {
		t.Fatalf("ObligationsAsOfOpts(before mint) = %+v, want empty", asOfBefore.Rules)
	}
}

// TestObligationsFor_DefaultByteIdenticalWithBiTemporalFieldsUnset (SU-19):
// a claim written with ValidFrom/ValidTo/AssertedBy all unset (the shape
// every pre-SU-19 claim in the store has, and the shape every writer this
// wave has not yet touched still produces) serves a payload with no "as_of"
// key and no "asserted_by" key anywhere — the live path's JSON shape is
// unchanged from before these fields existed.
func TestObligationsFor_DefaultByteIdenticalWithBiTemporalFieldsUnset(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedServed(t, st, "c1", "Rule one.", []string{"**"}, core.ClaimFresh)

	got, err := ObligationsFor(ctx, st, "svc", "any/file.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if got.AsOf != "" {
		t.Errorf("AsOf = %q, want empty for a live call", got.AsOf)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"as_of"`) {
		t.Errorf("live payload must never carry an as_of key: %s", raw)
	}
	if strings.Contains(string(raw), `"asserted_by"`) {
		t.Errorf("payload must omit asserted_by when the underlying claim never set it: %s", raw)
	}
}

// TestSemanticClaim_ValidAt pins core.SemanticClaim.ValidAt's own contract
// directly (unconstrained zero bounds, half-open upper bound).
func TestSemanticClaim_ValidAt(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	unconstrained := core.SemanticClaim{}
	if !unconstrained.ValidAt(t0) {
		t.Error("a claim with both bounds zero must be valid at any instant")
	}

	open := core.SemanticClaim{ValidFrom: t0}
	if open.ValidAt(t0.Add(-time.Second)) {
		t.Error("must not be valid before ValidFrom")
	}
	if !open.ValidAt(t0) || !open.ValidAt(t1) {
		t.Error("must be valid at and after ValidFrom with no ValidTo")
	}

	closed := core.SemanticClaim{ValidFrom: t0, ValidTo: t1}
	if !closed.ValidAt(t0) {
		t.Error("must be valid at ValidFrom (inclusive lower bound)")
	}
	if closed.ValidAt(t1) {
		t.Error("must not be valid AT ValidTo (exclusive upper bound)")
	}
	if !closed.ValidAt(t1.Add(-time.Second)) {
		t.Error("must be valid just before ValidTo")
	}
}
