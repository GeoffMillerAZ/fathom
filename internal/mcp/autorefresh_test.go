package mcp

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// refreshFake is a QueryInterface whose ServiceMeta reports a fixed indexed
// commit and which counts IngestService calls.
type refreshFake struct {
	path, indexed string
	ingests       int
}

func (f *refreshFake) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return f.path, f.indexed, true, nil
}
func (f *refreshFake) IngestService(_ context.Context, _ string) error { f.ingests++; return nil }
func (f *refreshFake) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *refreshFake) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (f *refreshFake) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *refreshFake) FindSymbol(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *refreshFake) Services(_ context.Context) ([]string, error) { return nil, nil }

func (f *refreshFake) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}

func (f *refreshFake) ListCommits(_ context.Context, _ string) ([]CommitInfo, error) { return nil, nil }

func (f *refreshFake) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) { return nil, nil }

func newAR(fake QueryInterface, head string) *autoRefreshBackend {
	return &autoRefreshBackend{
		QueryInterface: fake,
		headFn:         func(string) string { return head },
		seen:           make(map[string]string),
		inflight:       make(map[string]*inlineIngest),
		failures:       make(map[string]refreshFailure),
	}
}

// TestAutoRefresh_ReingestsOnDriftOnce: HEAD drifted from the indexed commit, so
// the first read re-ingests; subsequent reads are debounced (at most one ingest
// per (service, commit)).
func TestAutoRefresh_ReingestsOnDriftOnce(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "oldsha"}
	ar := newAR(fake, "newsha")
	ctx := context.Background()
	ar.ListAllSymbols(ctx, "svc")
	ar.FindSymbol(ctx, "svc", "X")
	ar.QuerySymbols(ctx, core.Query{ServiceID: "svc"})
	ar.Sources(ctx, "svc")
	ar.TypedEdges(ctx, "svc")
	if fake.ingests != 1 {
		t.Fatalf("ingests = %d, want 1 (debounced)", fake.ingests)
	}
}

// TestAutoRefresh_TypedEdges_TriggersEnsure confirms TypedEdges is one of
// the intercepted per-service reads (ADR-021 §3): a fresh wrapper (no prior
// debounce) must refresh before answering, exactly like the other reads.
func TestAutoRefresh_TypedEdges_TriggersEnsure(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "oldsha"}
	ar := newAR(fake, "newsha")
	if _, err := ar.TypedEdges(context.Background(), "svc"); err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if fake.ingests != 1 {
		t.Fatalf("ingests = %d, want 1 (TypedEdges must trigger ensure)", fake.ingests)
	}
}

// TestAutoRefresh_AutoRefreshTotal_CountsDebouncedRefreshesOnce pins O108's
// /metrics gauge (fathom_autorefresh_total, cmd/fathom/servehttp.go): the
// counter increments once per debounced drift, matching the ingest count
// TestAutoRefresh_ReingestsOnDriftOnce already pins — never once per
// intercepted read.
func TestAutoRefresh_AutoRefreshTotal_CountsDebouncedRefreshesOnce(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "oldsha"}
	ar := newAR(fake, "newsha")
	ctx := context.Background()
	ar.ListAllSymbols(ctx, "svc")
	ar.FindSymbol(ctx, "svc", "X")
	ar.QuerySymbols(ctx, core.Query{ServiceID: "svc"})
	if got := ar.AutoRefreshTotal(); got != 1 {
		t.Fatalf("AutoRefreshTotal() = %d, want 1 (debounced)", got)
	}

	// A second, later drift bumps it again.
	ar.headFn = func(string) string { return "newersha" }
	ar.ListAllSymbols(ctx, "svc")
	if got := ar.AutoRefreshTotal(); got != 2 {
		t.Fatalf("AutoRefreshTotal() after a second drift = %d, want 2", got)
	}
}

// TestAutoRefresh_NoReingestWhenFresh: HEAD == indexed commit → no ingest.
func TestAutoRefresh_NoReingestWhenFresh(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "same"}
	ar := newAR(fake, "same")
	ar.ListAllSymbols(context.Background(), "svc")
	if fake.ingests != 0 {
		t.Fatalf("ingests = %d, want 0 (fresh)", fake.ingests)
	}
}

// TestAutoRefresh_NoReingestWhenHeadUnknown: git HEAD unavailable → leave the
// index as-is rather than risk a failed query.
func TestAutoRefresh_NoReingestWhenHeadUnknown(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "old"}
	ar := newAR(fake, "")
	ar.ListAllSymbols(context.Background(), "svc")
	if fake.ingests != 0 {
		t.Fatalf("ingests = %d, want 0 (head unknown)", fake.ingests)
	}
}

// capabilityFake adds the four VB-9/D2/O54 obligations-adjacent optional
// capabilities wave N15 item 5 forwards (ObligationsLayer,
// RootObligationsLayer, ObligationsLayerCoverage, ClaimFeedback) to
// refreshFake, recording each call's own arguments.
type capabilityFake struct {
	refreshFake
	gotLayerService, gotLayerFile string
	gotLayerFilter                semantic.ObligationLayerFilter
	gotRootLayerFilter            semantic.ObligationLayerFilter
	coverage                      semantic.LayerCoverage
	coverageFound                 bool
	gotFeedbackClaimID            string
}

func (f *capabilityFake) ObligationsLayer(_ context.Context, service, file string, _ int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	f.gotLayerService, f.gotLayerFile, f.gotLayerFilter = service, file, layer
	return semantic.Obligations{Service: service, File: file}, nil
}

func (f *capabilityFake) RootObligationsLayer(_ context.Context, service string, _ int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	f.gotRootLayerFilter = layer
	return semantic.Obligations{Service: service}, nil
}

func (f *capabilityFake) ObligationsLayerCoverage(_ context.Context, _ string) (semantic.LayerCoverage, bool, error) {
	return f.coverage, f.coverageFound, nil
}

func (f *capabilityFake) ClaimFeedback(_ context.Context, _, claimID, _, _, _ string) error {
	f.gotFeedbackClaimID = claimID
	return nil
}

// TestAutoRefresh_ForwardsObligationsLayerCapabilities is wave N15 item
// 5's own proof: internal/mcp/autorefresh.go forwarded RawFallbackObligations
// (ADR-037's wave O1) but not ObligationsLayerCoverage, ObligationsLayer,
// RootObligationsLayer, or ClaimFeedback — so the operator daemon (serve-http's
// own default: auto-refresh ON) never emitted layer_coverage at all, even
// after O54 deliverable 2 shipped, because wrapping the client in
// autoRefreshBackend silently stripped the capability before any request
// reached it (the exact dogfood O7 forwarding class every OTHER optional
// capability in this file already documents). A wrapped backend that DOES
// implement all four must expose every one of them through the wrapper.
func TestAutoRefresh_ForwardsObligationsLayerCapabilities(t *testing.T) {
	fake := &capabilityFake{
		refreshFake:   refreshFake{path: "/repo", indexed: "same"},
		coverage:      semantic.LayerCoverage{SectionsTotal: 5, SectionsCompiled: 3, BudgetStopped: true},
		coverageFound: true,
	}
	ar := newAR(fake, "same")
	ctx := context.Background()

	if _, err := ar.ObligationsLayer(ctx, "svc", "a.go", 6, semantic.ObligationLayerRules); err != nil {
		t.Fatalf("ObligationsLayer: %v", err)
	}
	if fake.gotLayerService != "svc" || fake.gotLayerFile != "a.go" || fake.gotLayerFilter != semantic.ObligationLayerRules {
		t.Errorf("ObligationsLayer not forwarded through the wrapper: service=%q file=%q layer=%q", fake.gotLayerService, fake.gotLayerFile, fake.gotLayerFilter)
	}

	if _, err := ar.RootObligationsLayer(ctx, "svc", 6, semantic.ObligationLayerClaims); err != nil {
		t.Fatalf("RootObligationsLayer: %v", err)
	}
	if fake.gotRootLayerFilter != semantic.ObligationLayerClaims {
		t.Errorf("RootObligationsLayer not forwarded through the wrapper: got layer %q", fake.gotRootLayerFilter)
	}

	cov, found, err := ar.ObligationsLayerCoverage(ctx, "svc")
	if err != nil {
		t.Fatalf("ObligationsLayerCoverage: %v", err)
	}
	if !found {
		t.Fatal("ObligationsLayerCoverage not forwarded through the wrapper: found=false")
	}
	if cov.SectionsTotal != 5 || cov.SectionsCompiled != 3 || !cov.BudgetStopped {
		t.Errorf("ObligationsLayerCoverage payload lost through the wrapper: got %+v", cov)
	}

	if err := ar.ClaimFeedback(ctx, "svc", "claim-1", "refute", "note", "author"); err != nil {
		t.Fatalf("ClaimFeedback: %v", err)
	}
	if fake.gotFeedbackClaimID != "claim-1" {
		t.Errorf("ClaimFeedback not forwarded through the wrapper: got claim id %q", fake.gotFeedbackClaimID)
	}
}

// TestAutoRefresh_ObligationsLayerCapabilitiesDegradeGracefully proves the
// OTHER half: a backend that does NOT implement one of these four optional
// capabilities degrades honestly through the wrapper (an error, or
// found=false) rather than panicking on a failed type assertion.
func TestAutoRefresh_ObligationsLayerCapabilitiesDegradeGracefully(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "same"}
	ar := newAR(fake, "same")
	ctx := context.Background()

	if _, err := ar.ObligationsLayer(ctx, "svc", "a.go", 6, semantic.ObligationLayerAll); err == nil {
		t.Error("expected an error when the backend does not implement obligationLayerBackend")
	}
	if _, err := ar.RootObligationsLayer(ctx, "svc", 6, semantic.ObligationLayerAll); err == nil {
		t.Error("expected an error when the backend does not implement rootObligationsLayerBackend")
	}
	if _, found, err := ar.ObligationsLayerCoverage(ctx, "svc"); err != nil || found {
		t.Errorf("expected found=false, err=nil for a backend with no coverage capability, got found=%v err=%v", found, err)
	}
	if err := ar.ClaimFeedback(ctx, "svc", "c1", "refute", "n", "a"); err == nil {
		t.Error("expected an error when the backend does not implement claimFeedbackBackend")
	}
}
