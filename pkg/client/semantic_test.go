package client_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func newTestClient(t *testing.T, dir string) *client.Client {
	t.Helper()
	st := store.NewMemoryStore()
	conn := connector.NewLocalConnector()
	return client.New(conn, st)
}

func fp(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ─── corpus with CLAUDE.md + one Go file ───────────────────────────────────

func TestSemanticEndToEnd(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Governing document.
	claudeMD := []byte(`# Rules

- Run tests before committing.
- Sort all exported slice results.
`)
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}

	// Source file that will be indexed.
	goFile := []byte("package main\nfunc Foo() {}\n")
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), goFile, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()

	// Ingest so the store has service_meta and observations.
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	// Dry-run compile (no LLM needed).
	run, err := c.CompileObligations(ctx, "svc", true)
	if err != nil {
		t.Fatalf("CompileObligations dry-run: %v", err)
	}
	if run.DryRun != true {
		t.Errorf("DryRun flag not set")
	}
	if run.SectionsTotal == 0 {
		t.Errorf("dry-run: expected >0 sections, got %d", run.SectionsTotal)
	}
	t.Logf("dry-run: sections=%d tokensIn=%d", run.SectionsTotal, run.TokensIn)

	// Live compile with mock LLM.
	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["**"],"cite_start":3,"cite_end":3},{"rule":"Sort all exported slice results.","scope":["pkg/**"],"cite_start":4,"cite_end":4}]}`
	mock := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 80, OutputTokens: 30},
		},
	})
	c.WithLLM(mock, "mock-model", 0.001, 0.005)

	run2, err := c.CompileObligations(ctx, "svc", false)
	if err != nil {
		t.Fatalf("CompileObligations live: %v", err)
	}
	if run2.ClaimsEmitted < 1 {
		t.Errorf("expected ≥1 claims emitted, got %d (rejected=%d)", run2.ClaimsEmitted, run2.ClaimsRejected)
	}
	t.Logf("live: emitted=%d rejected=%d tokens=%d/%d cost=%.6f",
		run2.ClaimsEmitted, run2.ClaimsRejected, run2.TokensIn, run2.TokensOut, run2.CostUSD)

	// Obligations returns rules for pkg/foo.go.
	obs, err := c.Obligations(ctx, "svc", "pkg/foo.go", 0)
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if len(obs.Rules) == 0 {
		t.Errorf("expected obligations for pkg/foo.go, got none")
	}
	t.Logf("obligations for pkg/foo.go: %d rules", len(obs.Rules))

	// ObligationsIndexStale: file not changed → false.
	stale := c.ObligationsIndexStale(ctx, "svc", "CLAUDE.md")
	// After ingest CLAUDE.md is in the store; content same on disk → not stale.
	// (May be false even if not ingested since we can't fingerprint what we don't have.)
	t.Logf("ObligationsIndexStale(CLAUDE.md) = %v (may be false if not fingerprinted)", stale)

	// SweepClaims: should run without error.
	sr, err := c.SweepClaims(ctx, "svc")
	if err != nil {
		t.Fatalf("SweepClaims: %v", err)
	}
	t.Logf("sweep: checked=%d fresh=%d stale=%d contradicted=%d",
		sr.ClaimsChecked, sr.Fresh, sr.Stale, sr.Contradicted)
}

// TestDryRunWithoutLLM: no LLM configured, dry_run → ok; !dry_run → error.
func TestDryRunWithoutLLM(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# R\n\n- Do X.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	if _, err := c.CompileObligations(ctx, "svc", true); err != nil {
		t.Errorf("dry-run without LLM should succeed: %v", err)
	}
	if _, err := c.CompileObligations(ctx, "svc", false); err == nil {
		t.Errorf("live compile without LLM should return error")
	}
}

// TestObligationsIndexStale: edit file on disk, stale should flip.
func TestObligationsIndexStale(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fpath := filepath.Join(dir, "CLAUDE.md")
	initial := []byte("# R\n\n- Keep tests green.\n")
	if err := os.WriteFile(fpath, initial, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	// Not stale immediately after ingest.
	if c.ObligationsIndexStale(ctx, "svc", "CLAUDE.md") {
		t.Error("expected not stale right after ingest")
	}

	// Edit the file.
	if err := os.WriteFile(fpath, []byte("# R\n\n- Keep tests green.\n- New rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !c.ObligationsIndexStale(ctx, "svc", "CLAUDE.md") {
		t.Error("expected stale after editing CLAUDE.md on disk without re-ingest")
	}
}

// TestObligationsLayerCoverage_LiveCorpusReflectsGrowth is O54 deliverable
// 2's second review round, end to end through pkg/client: a compile_runs
// row's own sections_total goes stale the moment a NEW governing doc is
// added to disk — Client.ObligationsLayerCoverage must read the LIVE
// corpus (the same way a real compile resolves it), not the possibly
// stale recorded numbers. Since RT-143 the corpus PATHS are memoized per
// store watermark (a names-only walk shared by every call between ingests),
// so a NEW guide is noticed at the next ingest — which is all it takes; no
// recompile. An edit to a guide already known is noticed on the next call
// (TestLayerCoverageFromSourcePathsSingleFlightNeverReadsContents).
func TestObligationsLayerCoverage_LiveCorpusReflectsGrowth(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n\n- Run tests before committing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), []byte("package main\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	c.WithLLM(mock, "mock-model", 0, 0)
	run, err := c.CompileObligations(ctx, "svc", false)
	if err != nil || run.SectionsCompiled != 1 {
		t.Fatalf("compile: run=%+v err=%v", run, err)
	}

	cov, found, err := c.ObligationsLayerCoverage(ctx, "svc")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if cov.SectionsTotal != 1 || cov.SectionsCompiled != 1 || cov.CorpusGrew != 0 {
		t.Fatalf("cov = %+v, want fully covered, no growth yet", cov)
	}

	// Add a nested guide to disk AFTER the compile — deliberately no
	// recompile; the next ingest moves the watermark.
	if err := os.MkdirAll(filepath.Join(dir, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "CLAUDE.md"), []byte("# internal\n\n- Register every extractor.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("re-Ingest:", err)
	}

	cov2, found, err := c.ObligationsLayerCoverage(ctx, "svc")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if cov2.SectionsTotal != 2 {
		t.Fatalf("cov2.SectionsTotal = %d, want 2 (the live corpus, not the stale recorded 1)", cov2.SectionsTotal)
	}
	if cov2.SectionsCompiled != 1 {
		t.Fatalf("cov2.SectionsCompiled = %d, want 1 (only the original CLAUDE.md)", cov2.SectionsCompiled)
	}
	if cov2.CorpusGrew != 1 {
		t.Fatalf("cov2.CorpusGrew = %d, want 1 (internal/CLAUDE.md has never been seen by any run)", cov2.CorpusGrew)
	}
	if want := "internal/CLAUDE.md"; len(cov2.UncompiledScopes) != 1 || cov2.UncompiledScopes[0] != want {
		t.Fatalf("cov2.UncompiledScopes = %v, want [%q]", cov2.UncompiledScopes, want)
	}
}

// TestSweepClaims_AnchorDeadRecordsContradictedEvent is D2-2 (docs/plans/
// expert-depth.md) plus P1-9's vocabulary fix (docs/plans/
// pulled-tools-excellence.md): SweepClaims used to flip an anchor-dead
// claim's status to contradicted silently. It must now ALSO leave a
// ClaimEvent{Verdict: "contradicted", Author:"sweep"} — the audit trail
// durability needs, using the SAME mechanism a human `fathom claim refute` /
// the obligations tool's feedback action (D2-1) uses for its own "refuted"
// verdict, but a distinct verdict string: "refuted" names a deliberate,
// terminal human judgment (never served again); "contradicted" names this
// anchor-dead, machine-detected state, which stays served with the status
// label attached. The two must never share a verdict string. Seeds the
// claim directly against the store (kept in scope alongside the Client,
// mirroring internal/semantic/sweep_test.go's seedClaim helper) rather than
// through a live LLM compile — this is a targeted test of SweepClaims'
// behavior, not an end-to-end compile test (TestSemanticEndToEnd already
// covers that path).
func TestSweepClaims_AnchorDeadRecordsContradictedEvent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st := store.NewMemoryStore()
	conn := connector.NewLocalConnector()
	c := client.New(conn, st)
	defer c.Close()

	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: dir}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	docPath := filepath.Join(dir, "CLAUDE.md")
	doc := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(docPath, doc, 0o644); err != nil {
		t.Fatal(err)
	}
	hash, ok := core.HashLines(doc, 3, 3)
	if !ok {
		t.Fatal("seed citation does not resolve")
	}
	claim := core.SemanticClaim{
		ClaimID: "cl-anchor-dead", ServiceID: "svc",
		Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "CLAUDE.md#s1", Rule: "Run tests before committing.",
		Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
		Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
	}
	links := []core.ClaimLink{{ClaimID: claim.ClaimID, Rel: core.RelCites, TargetRef: core.CiteRef("CLAUDE.md", 3, 3), TargetHash: hash}}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	// Anchor goes dead: the cited doc is deleted from disk.
	if err := os.Remove(docPath); err != nil {
		t.Fatal(err)
	}

	rep, err := c.SweepClaims(ctx, "svc")
	if err != nil {
		t.Fatalf("SweepClaims: %v", err)
	}
	if rep.Contradicted != 1 || rep.Flipped != 1 {
		t.Fatalf("sweep report = %+v, want contradicted=1 flipped=1", rep)
	}

	claims, err := st.ClaimsForService(ctx, "svc", "")
	if err != nil || len(claims) != 1 || claims[0].Status != core.ClaimContradicted {
		t.Fatalf("claim status after sweep: err=%v claims=%+v, want 1 claim contradicted", err, claims)
	}

	events, err := st.ClaimEvents(ctx, claim.ClaimID)
	if err != nil {
		t.Fatalf("ClaimEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 contradicted event, got %d: %+v", len(events), events)
	}
	if events[0].Verdict != "contradicted" || events[0].Author != "sweep" {
		t.Errorf("event = %+v, want verdict=contradicted author=sweep", events[0])
	}
}

// TestSweepClaims_ReanchorsOnUpstreamLineShift is T5-1/O13 (docs/dogfood-
// observations.md, docs/plans/trust-and-tokenomics.md) exercised through the
// PUBLIC pkg/client wrapper (the plan row's anchor: {symbol: SweepClaims,
// package: pkg/client}) rather than internal/semantic.Sweep directly —
// internal/semantic/sweep_test.go covers the algorithm exhaustively
// (heal/stale/ambiguous/legacy); this proves the fix reaches real callers
// through the same on-disk fetch path CompileLayer/SweepClaims use in
// production (os.ReadFile against service_meta.Path), not just an injected
// fetch func. An upstream insertion shifts the cited rule down two lines —
// its own text unchanged — and the sweep must re-anchor the citation in
// place with an audit event instead of flipping the claim stale.
func TestSweepClaims_ReanchorsOnUpstreamLineShift(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st := store.NewMemoryStore()
	conn := connector.NewLocalConnector()
	c := client.New(conn, st)
	defer c.Close()

	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: dir}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	docPath := filepath.Join(dir, "CLAUDE.md")
	original := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(docPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	hash, ok := core.HashLines(original, 3, 3)
	if !ok {
		t.Fatal("seed citation does not resolve")
	}
	text, ok := core.LinesText(original, 3, 3)
	if !ok {
		t.Fatal("seed citation text does not resolve")
	}
	claim := core.SemanticClaim{
		ClaimID: "cl-shift", ServiceID: "svc",
		Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "CLAUDE.md#s1", Rule: "Run tests before committing.",
		Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
		Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
	}
	links := []core.ClaimLink{{
		ClaimID: claim.ClaimID, Rel: core.RelCites,
		TargetRef: core.CiteRef("CLAUDE.md", 3, 3), TargetHash: hash, AnchorText: text,
	}}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	// Upstream insertion: two prose lines land above the rule on disk — the
	// rule's own text is unchanged, just shifted down.
	shifted := []byte("# Rules\n\nSome new prose inserted above the bullets.\nMore prose.\n\n- Run tests before committing.\n")
	if err := os.WriteFile(docPath, shifted, 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := c.SweepClaims(ctx, "svc")
	if err != nil {
		t.Fatalf("SweepClaims: %v", err)
	}
	if rep.ReAnchored != 1 || rep.Stale != 0 || rep.Fresh != 1 {
		t.Fatalf("sweep report = %+v, want reanchored=1 stale=0 fresh=1", rep)
	}

	claims, err := st.ClaimsForService(ctx, "svc", "")
	if err != nil || len(claims) != 1 || claims[0].Status != core.ClaimFresh {
		t.Fatalf("claim status after sweep: err=%v claims=%+v, want 1 claim fresh (healed)", err, claims)
	}

	gotLinks, err := st.ClaimLinks(ctx, "svc", []string{claim.ClaimID})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	if len(gotLinks) != 1 || gotLinks[0].TargetRef != core.CiteRef("CLAUDE.md", 6, 6) {
		t.Fatalf("citation not re-anchored: got %+v, want CLAUDE.md#L6-L6", gotLinks)
	}

	events, err := st.ClaimEvents(ctx, claim.ClaimID)
	if err != nil {
		t.Fatalf("ClaimEvents: %v", err)
	}
	if len(events) != 1 || events[0].Verdict != "re-anchored" || events[0].Author != "sweep" {
		t.Fatalf("events = %+v, want 1 re-anchored/sweep event", events)
	}
}

// seedFeedbackClaim writes one minimal fresh claim directly against st, for
// ClaimFeedback tests that don't need a real citation/scope.
func seedFeedbackClaim(t *testing.T, st store.Store, service, claimID string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: claimID, ServiceID: service,
		Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "CLAUDE.md#s1", Rule: "Run tests before committing.",
		Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
		Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}
}

// TestClaimFeedback_Refute is D2-1 (docs/plans/expert-depth.md): the
// pkg/client write path behind the obligations tool's feedback action.
// refute must append a ClaimEvent{Verdict:"refuted"} AND flip the claim's
// status to core.ClaimRefuted — mirroring `fathom claim refute`
// (cmd/fathom/claim.go) exactly.
func TestClaimFeedback_Refute(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	seedFeedbackClaim(t, st, "svc", "cl-1")

	if err := c.ClaimFeedback(ctx, "svc", "cl-1", "refute", "already fixed upstream", "alice"); err != nil {
		t.Fatalf("ClaimFeedback: %v", err)
	}

	claims, err := st.ClaimsForService(ctx, "svc", "")
	if err != nil || len(claims) != 1 || claims[0].Status != core.ClaimRefuted {
		t.Fatalf("claims after refute: err=%v claims=%+v, want 1 claim refuted", err, claims)
	}
	events, err := st.ClaimEvents(ctx, "cl-1")
	if err != nil || len(events) != 1 {
		t.Fatalf("ClaimEvents: err=%v n=%d", err, len(events))
	}
	if events[0].Verdict != "refuted" || events[0].Author != "alice" || events[0].Note != "already fixed upstream" {
		t.Errorf("event = %+v", events[0])
	}
}

// TestClaimFeedback_CorroborateNeverPromotesStatus mirrors
// cmd/fathom/claim.go's doctrine: corroboration is provenance, never
// evidence-laundering — status and tier must be untouched.
func TestClaimFeedback_CorroborateNeverPromotesStatus(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	seedFeedbackClaim(t, st, "svc", "cl-2")

	if err := c.ClaimFeedback(ctx, "svc", "cl-2", "corroborate", "confirmed in review", ""); err != nil {
		t.Fatalf("ClaimFeedback: %v", err)
	}

	claims, err := st.ClaimsForService(ctx, "svc", "")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: err=%v n=%d", err, len(claims))
	}
	if claims[0].Status != core.ClaimFresh {
		t.Errorf("status = %q, want unchanged (fresh) — corroborate must never flip status", claims[0].Status)
	}
	if claims[0].Tier != core.TierDocClaimed {
		t.Errorf("tier = %q, want unchanged", claims[0].Tier)
	}
	events, err := st.ClaimEvents(ctx, "cl-2")
	if err != nil || len(events) != 1 || events[0].Verdict != "corroborated" {
		t.Errorf("events = %+v, err=%v", events, err)
	}
}

func TestClaimFeedback_Errors(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	seedFeedbackClaim(t, st, "svc", "cl-3")

	if err := c.ClaimFeedback(ctx, "svc", "cl-3", "delete", "x", ""); err == nil {
		t.Error("unknown action: expected error")
	}
	if err := c.ClaimFeedback(ctx, "svc", "no-such-claim", "refute", "x", ""); err == nil {
		t.Error("unknown claim: expected error")
	}
}

// TestClient_AppendClaimEvent_RoundTrips (Q-5, docs/plans/expert-suite-q.md).
func TestClient_AppendClaimEvent_RoundTrips(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	ev := core.ClaimEvent{
		ClaimID: "cl-4", Verdict: "superseded", Author: "review",
		Note: "finding fixed — no longer emitted by review run",
		At:   time.Unix(1700000000, 0).UTC(),
	}
	if err := c.AppendClaimEvent(ctx, ev); err != nil {
		t.Fatalf("AppendClaimEvent: %v", err)
	}
	events, err := st.ClaimEvents(ctx, "cl-4")
	if err != nil || len(events) != 1 {
		t.Fatalf("ClaimEvents: err=%v n=%d", err, len(events))
	}
	if events[0].Verdict != "superseded" || events[0].Author != "review" || events[0].Note != ev.Note {
		t.Errorf("event = %+v, want %+v", events[0], ev)
	}
}

// ─── wave O1 (ADR-037): RawFallbackObligations ─────────────────────────────
//
// The product defect the benches found: `obligations` served NOTHING
// USEFUL when the compiled layer's coverage for the edited path's own
// scope was incomplete. vs-pilot-5's second reading: the obligations tool
// never served the predicted rule in 0 of 4 trials built to test it, the
// compiled-rule layer having hit budget_stopped before the nested guide
// that mattered. guards-pilot-6's second reading: a brand-new file's
// PostToolUse hook returned only "index is behind your working tree" and
// nothing else.

// TestClient_RawFallbackObligations_CorpusGrowthServesNestedGuideRaw is
// test (b): a nested guide added to the repo AFTER a full compile (the
// exact TestObligationsLayerCoverage_LiveCorpusReflectsGrowth shape) never
// gets a compiled claim of its own until the NEXT compile. RawFallback must
// serve it raw, and the ALREADY-COMPILED root rule for the same file must
// not be touched — the two are independent calls at this layer; ordering
// them in one payload is internal/mcp's job (see
// TestObligationsTool_RawFallback_NestedRankedAheadOfCompiledRoot).
func TestClient_RawFallbackObligations_CorpusGrowthServesNestedGuideRaw(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n\n- Run tests before committing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), []byte("package main\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	c.WithLLM(mock, "mock-model", 0, 0)
	run, err := c.CompileObligations(ctx, "svc", false)
	if err != nil || run.SectionsCompiled != 1 {
		t.Fatalf("compile: run=%+v err=%v", run, err)
	}

	// The COMPILED root rule already answers a file under pkg/ — untouched
	// by anything below.
	rootObl, err := c.Obligations(ctx, "svc", "pkg/foo.go", 0)
	if err != nil || len(rootObl.Rules) != 1 {
		t.Fatalf("Obligations(pkg/foo.go): obl=%+v err=%v", rootObl, err)
	}

	// A NESTED guide lands on disk after the compile — no re-ingest, no
	// recompile. internal/extractor/goextract.go's own governing guide has
	// never been turned into a claim at all.
	if err := os.MkdirAll(filepath.Join(dir, "internal", "extractor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "CLAUDE.md"), []byte("# internal\n\n- Register every extractor in fusion.New.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, why, ok, err := c.RawFallbackObligations(ctx, "svc", "internal/extractor/goextract.go", 2)
	if err != nil {
		t.Fatalf("RawFallbackObligations: %v", err)
	}
	if !ok || len(rows) == 0 {
		t.Fatalf("ok=%v rows=%+v, want a raw row for the never-compiled nested guide", ok, rows)
	}
	if rows[0].Tier != "raw" {
		t.Errorf("rows[0].Tier = %q, want raw", rows[0].Tier)
	}
	if rows[0].Src != "internal/CLAUDE.md#L1-L4" {
		t.Errorf("rows[0].Src = %q, want the nested guide's own citation", rows[0].Src)
	}
	if why == "" || !strings.Contains(why, "uncompiled") {
		t.Errorf("why = %q, want it to name the uncompiled status", why)
	}

	// The root rule for pkg/foo.go must still be exactly what it was —
	// RawFallbackObligations never touches the compiled path.
	rootObl2, err := c.Obligations(ctx, "svc", "pkg/foo.go", 0)
	if err != nil || len(rootObl2.Rules) != 1 || rootObl2.Rules[0].Rule != rootObl.Rules[0].Rule {
		t.Fatalf("Obligations(pkg/foo.go) changed: before=%+v after=%+v err=%v", rootObl, rootObl2, err)
	}
}

// TestClient_RawFallbackObligations_BehaviorTokenFromFileContent (wave O3,
// item 2) is design (2)'s end-to-end client-level proof: a root-wide
// convention (the workspace/ row — names no file, package, or symbol of
// its own) loses a narrow raw-fallback window to a lexically louder but
// unrelated decoy UNTIL the edited file's own real source (which
// RawFallbackObligations now reads from the already-fetched Sources list,
// alongside the pre-existing symbol-name lookup) is scanned for a
// behaviour signal (os.WriteFile) and the resulting "workspace" token
// folded into the same extraTokens path a resolved symbol name uses.
func TestClient_RawFallbackObligations_BehaviorTokenFromFileContent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(
		"# Rules\n\n## Non-negotiables — the workspace/ convention\n\nAll build artifacts "+
			"belong under the gitignored workspace/ directory, never elsewhere in the tree. "+
			"Run go tools from cmd as usual.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ADR.md"), []byte(
		"## ADR-050: cmd tool\n\nThe cmd package wraps go binaries.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "emit.go"), []byte(
		"package cmd\n\nimport \"os\"\n\nfunc emit(dir string) error {\n"+
			"\treturn os.WriteFile(dir, nil, 0o644)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	// Deliberately no compile at all — the "never compiled" raw path.

	rows, _, ok, err := c.RawFallbackObligations(ctx, "svc", "cmd/emit.go", 1)
	if err != nil {
		t.Fatalf("RawFallbackObligations: %v", err)
	}
	if !ok || len(rows) != 1 {
		t.Fatalf("ok=%v rows=%+v, want exactly one raw row", ok, rows)
	}
	if rows[0].Rule != "Non-negotiables — the workspace/ convention" {
		t.Errorf("rows[0].Rule = %q, want the workspace/ convention row — cmd/emit.go's own os.WriteFile call should have promoted it over the ADR-050 decoy: rows=%+v", rows[0].Rule, rows)
	}
}

// TestClient_RawFallbackObligations_BudgetStoppedNamesStatus is test (a)'s
// sibling at the client level: a RECORDED budget-stopped compile run (the
// store's own bookkeeping — no real LLM trip needed to prove the status
// label) leaves a nested guide's section uncompiled. RawFallback must name
// "budget_stopped" in why, distinct from the "uncompiled"/CorpusGrew and
// "no compile_runs row" statuses the sibling tests below name.
func TestClient_RawFallbackObligations_BudgetStoppedNamesStatus(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "internal", "mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "CLAUDE.md"), []byte("# internal\n\n- Sort all outputs.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "mcp", "tools.go"), []byte("package mcp\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	// A budget-stopped run recorded directly against the store — mirrors
	// TestObligationsLayerCoverage_ReadsLatestRun's own pattern. Zero
	// claims exist for the layer, so the live corpus scan (LiveLayerCoverage)
	// correctly reports internal/CLAUDE.md's section as never compiled,
	// while cov.BudgetStopped carries this run's OWN recorded flag through.
	if err := st.RecordCompileRun(ctx, core.CompileRun{
		RunID: "r1", ServiceID: "svc", Layer: "obligations",
		RanAt: time.Unix(1, 0).UTC(), SectionsTotal: 1, SectionsCompiled: 0,
		BudgetStopped: true,
	}); err != nil {
		t.Fatalf("RecordCompileRun: %v", err)
	}

	rows, why, ok, err := c.RawFallbackObligations(ctx, "svc", "internal/mcp/tools.go", 2)
	if err != nil {
		t.Fatalf("RawFallbackObligations: %v", err)
	}
	if !ok || len(rows) == 0 {
		t.Fatalf("ok=%v rows=%+v, want a raw row for the budget-stopped nested guide", ok, rows)
	}
	if !strings.Contains(why, "budget_stopped") {
		t.Errorf("why = %q, want it to name budget_stopped", why)
	}
	if rows[0].Src != "internal/CLAUDE.md#L1-L4" {
		t.Errorf("rows[0].Src = %q, want internal/CLAUDE.md's own citation", rows[0].Src)
	}
}

// TestClient_RawFallbackObligations_CompleteLayerReturnsFalse is test (c):
// once the layer's compile genuinely covers file's own governing guide,
// RawFallback must add nothing — the existing compiled answer stands
// unchanged.
func TestClient_RawFallbackObligations_CompleteLayerReturnsFalse(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n\n- Run tests before committing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), []byte("package main\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	c.WithLLM(mock, "mock-model", 0, 0)
	if run, err := c.CompileObligations(ctx, "svc", false); err != nil || run.SectionsCompiled != 1 {
		t.Fatalf("compile: run=%+v err=%v", run, err)
	}

	rows, why, ok, err := c.RawFallbackObligations(ctx, "svc", "pkg/foo.go", 2)
	if err != nil {
		t.Fatalf("RawFallbackObligations: %v", err)
	}
	if ok || len(rows) != 0 || why != "" {
		t.Fatalf("ok=%v rows=%+v why=%q, want no raw fallback for a fully-compiled layer", ok, rows, why)
	}
}

// TestClient_RawFallbackObligations_NeverCompiledAtAllStillServesRaw is
// guards-pilot-6's own defect, reproduced directly: a brand-new service
// with a governing guide but NO compile_runs row and NO claims at all —
// "index is behind your working tree" must not be the only thing served.
func TestClient_RawFallbackObligations_NeverCompiledAtAllStillServesRaw(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "use-cases"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "CLAUDE.md"), []byte("# docs\n\n- Every use-case doc needs a Maintained line.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "use-cases", "new-thing.md"), []byte("# New thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	// Deliberately NO CompileObligations call at all — the layer has never
	// been compiled, and no claim of any kind exists for it.

	rows, why, ok, err := c.RawFallbackObligations(ctx, "svc", "docs/use-cases/new-thing.md", 2)
	if err != nil {
		t.Fatalf("RawFallbackObligations: %v", err)
	}
	if !ok || len(rows) == 0 {
		t.Fatalf("ok=%v rows=%+v, want the nearest guide served raw even though nothing was ever compiled", ok, rows)
	}
	if !strings.Contains(why, "no compile_runs row") {
		t.Errorf("why = %q, want it to name the never-compiled status", why)
	}
	if rows[0].Src != "docs/CLAUDE.md#L1-L4" {
		t.Errorf("rows[0].Src = %q, want docs/CLAUDE.md's own citation", rows[0].Src)
	}
}

// TestClient_RawFallbackObligations_ClaimsWithoutCompileRunNeverGuesses
// protects internal/indexbench's L1 payload-replay fixture (SeedFixtureClaims
// writes claims directly via PutClaims — no compile_runs row is ever
// recorded for that store): a layer with real claims but no run record must
// NOT be treated as a coverage gap, or the L1 golden's byte count would move
// for a reason that has nothing to do with a genuine defect.
func TestClient_RawFallbackObligations_ClaimsWithoutCompileRunNeverGuesses(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n\n- Sort outputs.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	conn := connector.NewLocalConnector()
	c := client.New(conn, st)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	// Seed a claim directly against the store — no CompileObligations call,
	// so no compile_runs row exists, exactly like SeedFixtureClaims.
	claim := core.SemanticClaim{
		ClaimID: "svc:fx-sorted", ServiceID: "svc",
		Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "seeded#fx-sorted", Rule: "Sort outputs.",
		Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
		Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	rows, why, ok, err := c.RawFallbackObligations(ctx, "svc", "pkg/foo.go", 2)
	if err != nil {
		t.Fatalf("RawFallbackObligations: %v", err)
	}
	if ok || len(rows) != 0 || why != "" {
		t.Fatalf("ok=%v rows=%+v why=%q, want no raw fallback when claims exist without a compile_runs row", ok, rows, why)
	}
}

// TestClient_ClaimEventsSince_RoundTrips (docs/design/findings-as-
// evidence.md §6 items 3/7).
func TestClient_ClaimEventsSince_RoundTrips(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	in := core.ClaimEvent{ClaimID: "cl-5", Verdict: "superseded", At: time.Unix(1700000100, 0).UTC(), ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "abc"}
	if err := c.AppendClaimEvent(ctx, in); err != nil {
		t.Fatalf("AppendClaimEvent: %v", err)
	}
	got, err := c.ClaimEventsSince(ctx, time.Unix(1700000000, 0).UTC(), time.Unix(1700000200, 0).UTC())
	if err != nil {
		t.Fatalf("ClaimEventsSince: %v", err)
	}
	if len(got) != 1 || got[0].ClaimID != "cl-5" || got[0].ResolutionKind != core.ResolutionCodeChange {
		t.Fatalf("got %+v, want 1 event for cl-5 with ResolutionKind=%q", got, core.ResolutionCodeChange)
	}
}
