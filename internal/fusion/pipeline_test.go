// Package fusion_test exercises the source fusion pipeline using the
// FauxConnector fixture data and an in-process MemoryStore. No network,
// no disk writes beyond the store, no secrets required.
package fusion_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/query"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_WithGitHead verifies that WithGitHead injects a fake commit hash
// that ends up in ServiceMeta after Ingest.
func TestIngest_WithGitHead(t *testing.T) {
	const fakeCommit = "deadbeef1234567890abcdef"
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	p.WithGitHead(func(_ string) string { return fakeCommit })

	ctx := context.Background()
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest error: %v", err)
	}

	meta, found, err := st.GetServiceMeta(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("GetServiceMeta: %v", err)
	}
	if !found {
		t.Fatal("ServiceMeta not found after Ingest")
	}
	if meta.IndexedCommit != fakeCommit {
		t.Errorf("IndexedCommit = %q, want %q", meta.IndexedCommit, fakeCommit)
	}
	if meta.ServiceID != paymentSvc.ID {
		t.Errorf("ServiceID = %q, want %q", meta.ServiceID, paymentSvc.ID)
	}
	if meta.IndexedAt == 0 {
		t.Error("IndexedAt is zero, want non-zero timestamp")
	}
}

// TestIngest_WithGitHead_Empty verifies that when the git function returns ""
// the pipeline records the connector's content-addressed watermark instead
// (ADR-017) — best-effort, no error either way.
func TestIngest_WithGitHead_Empty(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	p.WithGitHead(func(_ string) string { return "" })

	ctx := context.Background()
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest error: %v", err)
	}

	meta, found, err := st.GetServiceMeta(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("GetServiceMeta: %v", err)
	}
	if !found {
		t.Fatal("ServiceMeta not found after Ingest")
	}
	// With no git HEAD the pipeline falls back to the connector's
	// content-addressed watermark (ADR-017: freshness is domain-neutral).
	if !strings.HasPrefix(meta.IndexedCommit, "manifest:") {
		t.Errorf("IndexedCommit = %q, want a manifest: watermark", meta.IndexedCommit)
	}
}

// paymentSvc is the ServiceRef used throughout all tests.
var paymentSvc = core.ServiceRef{
	ID:   "payment-service",
	Path: "payment-service",
}

// defaultCfg returns a Config suitable for deterministic tests.
func defaultCfg() fusion.Config {
	return fusion.Config{
		Concurrency: 4,
		MaxTokens:   0, // disabled
		MaxCostUSD:  0, // disabled
		Lazy:        false,
	}
}

// TestIngest_Basic verifies that ingesting the fauxtown payment-service through
// FauxConnector + MemoryStore stores observations and returns accurate counts.
func TestIngest_Basic(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())

	ctx := context.Background()
	report, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest error: %v", err)
	}

	// payment-service has 2 .go files; both must be discovered.
	if report.Discovered != 2 {
		t.Errorf("Discovered = %d, want 2", report.Discovered)
	}
	// First ingest: nothing is skipped (no stored fingerprints yet).
	if report.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0 on first ingest", report.Skipped)
	}
	// payment.go yields 5 symbols; errors.go yields 0 (unexported vars only).
	const wantSymbols = 5
	if report.Symbols != wantSymbols {
		t.Errorf("Symbols = %d, want %d", report.Symbols, wantSymbols)
	}
	// Extracted should equal Discovered minus Skipped.
	if report.Extracted != report.Discovered-report.Skipped {
		t.Errorf("Extracted = %d, want Discovered-Skipped = %d",
			report.Extracted, report.Discovered-report.Skipped)
	}

	// Observations must be persisted in the store.
	obs, err := st.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	if len(obs) != wantSymbols {
		t.Errorf("stored %d observations, want %d", len(obs), wantSymbols)
	}
	// Every observation must reference the correct service.
	for i, o := range obs {
		if o.ServiceID != paymentSvc.ID {
			t.Errorf("obs[%d].ServiceID = %q, want %q", i, o.ServiceID, paymentSvc.ID)
		}
		if o.EvidenceTier != core.TierVerified {
			t.Errorf("obs[%d].EvidenceTier = %q, want %q", i, o.EvidenceTier, core.TierVerified)
		}
		if o.ContentHash == "" {
			t.Errorf("obs[%d].ContentHash is empty", i)
		}
		if o.IngestedAt.IsZero() {
			t.Errorf("obs[%d].IngestedAt is zero", i)
		}
	}
}

// TestIngest_Deterministic verifies that running Ingest twice on an unchanged
// service produces the same observations in the same order.
func TestIngest_Deterministic(t *testing.T) {
	conn := connector.NewFauxConnector()
	ctx := context.Background()

	// First run.
	st1 := store.NewMemoryStore()
	p1 := fusion.New(conn, st1, defaultCfg())
	if _, err := p1.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	obs1, err := st1.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("first QueryObservations: %v", err)
	}

	// Second run — fresh store, same connector.
	st2 := store.NewMemoryStore()
	p2 := fusion.New(conn, st2, defaultCfg())
	if _, err := p2.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	obs2, err := st2.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("second QueryObservations: %v", err)
	}

	if len(obs1) != len(obs2) {
		t.Fatalf("run 1 stored %d obs, run 2 stored %d obs", len(obs1), len(obs2))
	}
	for i := range obs1 {
		if obs1[i].Symbol != obs2[i].Symbol {
			t.Errorf("obs[%d].Symbol: run1=%q run2=%q", i, obs1[i].Symbol, obs2[i].Symbol)
		}
		if obs1[i].ContentHash != obs2[i].ContentHash {
			t.Errorf("obs[%d].ContentHash: run1=%q run2=%q", i, obs1[i].ContentHash, obs2[i].ContentHash)
		}
		if obs1[i].Claim != obs2[i].Claim {
			t.Errorf("obs[%d].Claim: run1=%q run2=%q", i, obs1[i].Claim, obs2[i].Claim)
		}
	}
}

// TestIngest_HashGate verifies that re-ingesting an unchanged service skips all
// files and produces no new observations.
func TestIngest_HashGate(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	// First ingest populates fingerprints.
	r1, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	if r1.Skipped != 0 {
		t.Fatalf("first ingest: Skipped=%d, want 0", r1.Skipped)
	}

	// Second ingest — source files unchanged, fingerprints are current.
	r2, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if r2.Skipped != r2.Discovered {
		t.Errorf("second ingest: Skipped=%d, Discovered=%d; want Skipped==Discovered",
			r2.Skipped, r2.Discovered)
	}
	if r2.Extracted != 0 {
		t.Errorf("second ingest: Extracted=%d, want 0 (all hash-gated)", r2.Extracted)
	}
	if r2.Symbols != 0 {
		t.Errorf("second ingest: Symbols=%d, want 0 (no new extractions)", r2.Symbols)
	}

	// The observation count in the store must remain the same as after the first ingest.
	obs, err := st.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	if len(obs) != r1.Symbols {
		t.Errorf("store has %d obs after 2nd ingest, want %d (same as after 1st)", len(obs), r1.Symbols)
	}
}

// TestIngest_ChangeDetection verifies that corrupting a stored fingerprint causes
// the affected file to be re-extracted and its stale observations to be purged and
// replaced.
func TestIngest_ChangeDetection(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	// Initial ingest.
	r1, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("initial Ingest: %v", err)
	}

	// Corrupt the fingerprint of payment.go to simulate a changed file.
	// The path stored by FauxConnector is relative to the fauxtown root.
	// Discover returns paths like "payment-service/payment.go".
	refs, err := conn.Discover(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	var paymentRef core.SourceRef
	for _, ref := range refs {
		if ref.Path == "payment-service/payment.go" {
			paymentRef = ref
			break
		}
	}
	if paymentRef.Path == "" {
		t.Fatal("payment-service/payment.go not found in Discover output")
	}

	// Write a bogus fingerprint to force re-extraction.
	if err := st.SetFingerprint(ctx, paymentSvc.ID, paymentRef.Path, "deadbeef"); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}

	// Re-ingest: only payment.go fingerprint is wrong; errors.go is unchanged.
	r2, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("re-Ingest: %v", err)
	}

	// errors.go fingerprint is still correct → skipped.
	// payment.go fingerprint is wrong → re-extracted.
	if r2.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (only errors.go)", r2.Skipped)
	}
	if r2.Extracted != 1 {
		t.Errorf("Extracted = %d, want 1 (only payment.go)", r2.Extracted)
	}

	// After re-ingest, total observations should be the same as after initial ingest
	// (stale obs purged, new obs written).
	obs, err := st.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	if len(obs) != r1.Symbols {
		t.Errorf("store has %d obs after change-detected re-ingest, want %d",
			len(obs), r1.Symbols)
	}
}

// TestIngest_Lazy verifies that Lazy mode queues files during Ingest without
// extracting them, and that ExtractPending subsequently extracts and stores
// observations.
func TestIngest_Lazy(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	cfg := defaultCfg()
	cfg.Lazy = true
	p := fusion.New(conn, st, cfg)
	ctx := context.Background()

	// Ingest in lazy mode: should discover files but NOT extract.
	report, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest(lazy): %v", err)
	}
	if report.Discovered == 0 {
		t.Fatal("Discovered 0 files in lazy mode — fixture issue")
	}
	// No symbols should have been extracted yet.
	if report.Symbols != 0 {
		t.Errorf("lazy Ingest: Symbols=%d, want 0 (extraction deferred)", report.Symbols)
	}

	// Store should be empty before ExtractPending.
	obs, err := st.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations before ExtractPending: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("store has %d obs before ExtractPending, want 0", len(obs))
	}

	// Fire the deferred extraction.
	if err := p.ExtractPending(ctx, paymentSvc.ID); err != nil {
		t.Fatalf("ExtractPending: %v", err)
	}

	// Now observations should be present.
	obs, err = st.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations after ExtractPending: %v", err)
	}
	const wantSymbols = 5
	if len(obs) != wantSymbols {
		t.Errorf("store has %d obs after ExtractPending, want %d", len(obs), wantSymbols)
	}
}

// TestIngest_ObservationFields spot-checks that each observation is populated
// with the fields required by the design specification.
func TestIngest_ObservationFields(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	obs, err := st.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}

	for _, o := range obs {
		if o.ServiceID == "" {
			t.Errorf("ServiceID is empty for obs symbol=%q", o.Symbol)
		}
		if o.SourceRef == "" {
			t.Errorf("SourceRef is empty for obs symbol=%q", o.Symbol)
		}
		if o.SourceKind == "" {
			t.Errorf("SourceKind is empty for obs symbol=%q", o.Symbol)
		}
		if o.EvidenceTier != core.TierVerified {
			t.Errorf("EvidenceTier=%q for symbol=%q, want %q",
				o.EvidenceTier, o.Symbol, core.TierVerified)
		}
		if o.Symbol == "" {
			t.Error("Symbol is empty")
		}
		if o.ContentHash == "" {
			t.Errorf("ContentHash is empty for symbol=%q", o.Symbol)
		}
		if o.IngestedAt.IsZero() {
			t.Errorf("IngestedAt is zero for symbol=%q", o.Symbol)
		}
	}
}

// TestIngest_LineNumberRoundTrip verifies that line numbers extracted from Go
// source survive the full extractor → pipeline → store → query path.
// payment.go in the fauxtown fixture defines Charge at line 26 and
// validateCharge at line 34. Both must appear with non-zero Line in the query
// result returned by query.Engine.FindSymbol.
func TestIngest_LineNumberRoundTrip(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	engine := query.New(st)

	// Verify Charge (exported function) carries its line number.
	chargeNodes, err := engine.FindSymbol(ctx, paymentSvc.ID, "Charge")
	if err != nil {
		t.Fatalf("FindSymbol Charge: %v", err)
	}
	if len(chargeNodes) == 0 {
		t.Fatal("FindSymbol returned no nodes for Charge")
	}
	for _, n := range chargeNodes {
		if n.Name == "Charge" && n.Line == 0 {
			t.Errorf("Charge.Line = 0, want non-zero (extractor sets it from AST position)")
		}
	}

	// Verify validateCharge (unexported) also carries its line number.
	vcNodes, err := engine.FindSymbol(ctx, paymentSvc.ID, "validateCharge")
	if err != nil {
		t.Fatalf("FindSymbol validateCharge: %v", err)
	}
	if len(vcNodes) == 0 {
		t.Fatal("FindSymbol returned no nodes for validateCharge")
	}
	for _, n := range vcNodes {
		if n.Name == "validateCharge" && n.Line == 0 {
			t.Errorf("validateCharge.Line = 0, want non-zero")
		}
	}
}

// TestIngest_SelfHealsLegacyAbsoluteAnchors verifies the ADR-017 migration
// guard: a store carrying pre-ADR-017 absolute-path anchors is purged
// wholesale on the first ingest that discovers repo-relative refs, so no
// symbol exists under both anchor styles.
func TestIngest_SelfHealsLegacyAbsoluteAnchors(t *testing.T) {
	ctx := context.Background()
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()

	// Seed the legacy state: one observation + fingerprint keyed by an
	// absolute machine-local path.
	legacy := core.Observation{
		ServiceID:    paymentSvc.ID,
		SourceRef:    "/machines/old-host/payment-service/payment.go",
		SourceKind:   core.KindGoFunc,
		EvidenceTier: core.TierVerified,
		Symbol:       "Charge",
		Claim:        "func Charge()",
		ContentHash:  "stale",
	}
	if err := st.PutObservations(ctx, []core.Observation{legacy}); err != nil {
		t.Fatalf("seed observations: %v", err)
	}
	if err := st.SetFingerprint(ctx, paymentSvc.ID, legacy.SourceRef, "stale"); err != nil {
		t.Fatalf("seed fingerprint: %v", err)
	}

	p := fusion.New(conn, st, defaultCfg())
	p.WithGitHead(func(_ string) string { return "" })
	report, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if !report.PurgedLegacyAnchors {
		t.Error("report.PurgedLegacyAnchors = false, want true")
	}

	obs, err := st.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	for _, o := range obs {
		if strings.HasPrefix(o.SourceRef, "/") {
			t.Errorf("absolute anchor survived self-heal: %q", o.SourceRef)
		}
	}
	// The symbol exists exactly once (relative anchor), not duplicated.
	n := 0
	for _, o := range obs {
		if o.Symbol == "Charge" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Charge observed %d times after self-heal, want exactly 1", n)
	}

	// A second ingest must not purge again (steady state).
	report2, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if report2.PurgedLegacyAnchors {
		t.Error("second ingest purged again; self-heal must be one-shot")
	}
}
