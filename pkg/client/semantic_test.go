package client_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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
