package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// reviewFixtureSrc is a tiny Go file whose func A has an in-body comment
// referencing a spec ID ("[US-777]") that no symbol's own doc comment (and
// therefore no node's Signature — the extractor's per-symbol doc/signature
// field) ever repeats. spec_coverage's codeRefs is built ONLY from node
// Name+Signature, so this spec ID is genuinely "referenced in the diff" (the
// raw-source scan sees it) yet "uncovered" (no symbol claims it) — a
// reliable, extractor-quirk-proof way to force exactly one deterministic
// review finding through a REAL ingest, without depending on precise Go
// call-edge resolution the way an arch_conformance violation would.
const reviewFixtureSrc = `package pkg

func A() {
	// satisfies [US-777] but nothing implements it yet
}
`

func TestReview_RequiredFlags(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runReview([]string{"--since", "origin/main"}, fakeGitRunner(nil), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --service is missing")
	}

	out.Reset()
	errBuf.Reset()
	code = runReview([]string{"--service", "svc"}, fakeGitRunner(nil), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --since is missing")
	}
}

func TestReview_NoChangedFiles(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{"--service", "svc", "--since", "origin/main"},
		fakeGitRunner(nil), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "nothing to review") {
		t.Errorf("output = %q", out.String())
	}
}

// TestReview_OneCallWorkedExample ingests a real fixture and runs `fathom
// review` end to end, then asserts the JSON verdict shape review_diff and
// the CLI share (internal/analysis/review.Verdict.Payload): four
// dimensions, a summary, and a spec_coverage finding for the uncovered
// [US-777] reference.
func TestReview_OneCallWorkedExample(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{"--service", "review-svc", "--since", "HEAD~1", "--store", storePath, "--json"},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if payload["service"] != "review-svc" {
		t.Errorf("service = %v", payload["service"])
	}
	dims, ok := payload["dimensions"].([]any)
	if !ok || len(dims) != 4 {
		t.Fatalf("expected 4 dimensions, got %v", payload["dimensions"])
	}
	var specDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "spec_coverage" {
			specDim = d
		}
	}
	if specDim == nil {
		t.Fatal("missing spec_coverage dimension")
	}
	findings, _ := specDim["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected 1 spec_coverage finding, got %d: %v", len(findings), findings)
	}
	f := findings[0].(map[string]any)
	if !strings.Contains(f["message"].(string), "US-777") {
		t.Errorf("finding message = %q, want a mention of US-777", f["message"])
	}
	if f["claim_id"] == nil || f["claim_id"] == "" {
		t.Error("expected a non-empty claim_id (findings persisted)")
	}
	if payload["findings_persisted"] != true {
		t.Errorf("findings_persisted = %v, want true", payload["findings_persisted"])
	}

	t.Logf("one-call worked example output:\n%s", out.String())
}

// TestReview_FindingsBridge_RefutationDurability is C2-1's "prove it with a
// test" scenario end to end through a real store: emit finding → refute via
// `fathom claim refute` → re-run review → the finding carries refuted
// status and is excluded from its dimension's active count.
func TestReview_FindingsBridge_RefutationDurability(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	runOnce := func() map[string]any {
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{"--service", "review-svc", "--since", "HEAD~1", "--store", storePath, "--json"},
			fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
		)
		if code != 0 {
			t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
		}
		return payload
	}
	specDimOf := func(payload map[string]any) map[string]any {
		for _, raw := range payload["dimensions"].([]any) {
			d := raw.(map[string]any)
			if d["name"] == "spec_coverage" {
				return d
			}
		}
		t.Fatal("missing spec_coverage dimension")
		return nil
	}

	// Run 1: emit + persist the finding.
	p1 := specDimOf(runOnce())
	f1 := p1["findings"].([]any)[0].(map[string]any)
	claimID, _ := f1["claim_id"].(string)
	if claimID == "" {
		t.Fatal("expected a non-empty claim_id")
	}
	if _, hasStatus := f1["status"]; hasStatus {
		t.Errorf("fresh finding should carry no status, got %v", f1["status"])
	}
	if p1["count"].(float64) != 1 {
		t.Errorf("count = %v, want 1", p1["count"])
	}

	// Refute it via the CLI's own refute verb.
	var refuteOut, refuteErr bytes.Buffer
	if code := run([]string{
		"claim", "refute",
		"--store", storePath, "--service", "review-svc", "--claim", claimID,
		"--note", "false positive: covered by design doc", "--author", "test",
	}, &refuteOut, &refuteErr); code != 0 {
		t.Fatalf("claim refute exit = %d, stderr: %s", code, refuteErr.String())
	}

	// Run 2: the identical finding must re-derive the SAME ClaimID and now
	// carry status "refuted", excluded from count but still listed.
	p2 := specDimOf(runOnce())
	findings2 := p2["findings"].([]any)
	if len(findings2) != 1 {
		t.Fatalf("refuted finding should remain listed, got %d findings", len(findings2))
	}
	f2 := findings2[0].(map[string]any)
	if f2["claim_id"] != claimID {
		t.Fatalf("ClaimID changed across re-review: %v vs %v", claimID, f2["claim_id"])
	}
	if f2["status"] != "refuted" {
		t.Errorf("status = %v, want %q", f2["status"], "refuted")
	}
	if p2["count"].(float64) != 0 {
		t.Errorf("count = %v, want 0 (the only finding is refuted)", p2["count"])
	}
}

// ─── critic-round regression tests (defects 1-4, docs/plans/review-verb.md) ──

// reviewIngestFixture writes the [US-777] fixture, ingests it, and returns
// (root, storePath).
func reviewIngestFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}
	return root, storePath
}

// runReviewJSON runs a persisting review over pkg/a.go and returns the payload.
func runReviewJSON(t *testing.T, storePath string, extraArgs ...string) map[string]any {
	t.Helper()
	args := append([]string{
		"--service", "review-svc", "--since", "HEAD~1", "--store", storePath, "--json",
	}, extraArgs...)
	var out, errBuf bytes.Buffer
	code := runReview(args, fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf)
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	return payload
}

// reviewSvcClaims returns the current findings-layer claims for review-svc.
func reviewSvcClaims(t *testing.T, storePath string) []core.SemanticClaim {
	t.Helper()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	claims, err := st.ClaimsForService(context.Background(), "review-svc", "findings")
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	return claims
}

// TestReview_SweepZeroFlips is defect 1's regression test: run SweepClaims
// (semantic.Sweep with a real disk fetch — exactly what the daemon runs
// after every auto-refresh) over freshly persisted review claims and assert
// ZERO flips: no false stale, no false contradicted, no lying
// "anchor no longer resolves" events. Before the fix, persisted cites links
// carried no TargetHash and bare-path refs, so the first sweep flipped
// every claim and appended a contradicted event apiece.
func TestReview_SweepZeroFlips(t *testing.T) {
	root, storePath := reviewIngestFixture(t)
	runReviewJSON(t, storePath)

	claimsBefore := reviewSvcClaims(t, storePath)
	if len(claimsBefore) == 0 {
		t.Fatal("expected persisted review claims before the sweep")
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	fetch := func(path string) ([]byte, bool) {
		data, rerr := os.ReadFile(filepath.Join(root, path))
		return data, rerr == nil
	}
	rep, err := semantic.Sweep(context.Background(), st, "review-svc", fetch, nil)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if rep.Flipped != 0 || rep.Stale != 0 || rep.Contradicted != 0 {
		t.Fatalf("sweep over fresh review claims must not flip anything: %+v", rep)
	}
	for _, cl := range reviewSvcClaims(t, storePath) {
		if cl.Status != core.ClaimFresh {
			t.Errorf("claim %s status after sweep = %q, want fresh", cl.ClaimID, cl.Status)
		}
		events, eerr := st.ClaimEvents(context.Background(), cl.ClaimID)
		if eerr != nil {
			t.Fatalf("ClaimEvents: %v", eerr)
		}
		if len(events) != 0 {
			t.Errorf("claim %s has %d unexpected event(s): %+v", cl.ClaimID, len(events), events)
		}
	}
}

// TestReview_Idempotent_DoubleRun is defect 2's regression test: an
// identical persisting re-run yields the identical claim set — no
// obligations wrappers, no impact noise, no growth. Before the fix, an
// identical re-run grew the store (57→82 measured) by re-minting the
// obligations dimension's SERVED claims as new wrapper claims.
func TestReview_Idempotent_DoubleRun(t *testing.T) {
	_, storePath := reviewIngestFixture(t)

	runReviewJSON(t, storePath)
	after1 := reviewSvcClaims(t, storePath)
	runReviewJSON(t, storePath)
	after2 := reviewSvcClaims(t, storePath)

	if len(after1) != len(after2) {
		t.Fatalf("claim count changed across identical re-runs: %d → %d", len(after1), len(after2))
	}
	for _, cl := range after2 {
		if cl.PromptVersion != "review-verb/v1" {
			t.Errorf("claim %s PromptVersion = %q — only review-minted claims may exist here", cl.ClaimID, cl.PromptVersion)
		}
		if !strings.HasPrefix(cl.Subject, "arch_conformance#") && !strings.HasPrefix(cl.Subject, "spec_coverage#") {
			t.Errorf("claim %s subject = %q — only the deterministic-oracle dimensions may persist (never obligations wrappers or impact noise)", cl.ClaimID, cl.Subject)
		}
	}
}

// TestReview_ObligationsForUnpolluted is defect 2's serving-side regression
// test: a review run over a file governed by a real compiled obligation
// leaves ObligationsFor's served payload for that file UNCHANGED — review
// noise must never displace real obligations from the edit-hook's
// MaxObligations budget. The fixture review emits zero persistable findings
// (no arch rules → unchecked; the seeded file has no spec refs).
func TestReview_ObligationsForUnpolluted(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package pkg\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	// Seed one REAL obligations-layer claim governing pkg/**.
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	seeded := core.SemanticClaim{
		ClaimID:   core.ClaimID("obligations", "obligation", "CLAUDE.md#conventions", "seed"),
		ServiceID: "review-svc", Layer: "obligations", LayerVersion: "v1",
		Kind: "obligation", Subject: "CLAUDE.md#conventions",
		Rule: "Sort outputs.", Scope: []string{"pkg/**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh,
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{seeded}, nil); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}
	before, err := semantic.ObligationsFor(context.Background(), st, "review-svc", "pkg/a.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor before: %v", err)
	}
	st.Close()

	runReviewJSON(t, storePath)

	st2, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("re-open store: %v", err)
	}
	defer st2.Close()
	after, err := semantic.ObligationsFor(context.Background(), st2, "review-svc", "pkg/a.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor after: %v", err)
	}
	if len(before.Rules) != 1 {
		t.Fatalf("seed setup broken: expected 1 served rule before review, got %d", len(before.Rules))
	}
	if len(after.Rules) != len(before.Rules) {
		t.Fatalf("ObligationsFor polluted by review run: %d rules → %d", len(before.Rules), len(after.Rules))
	}
	if after.Rules[0].Rule != before.Rules[0].Rule {
		t.Errorf("served rule changed: %q → %q", before.Rules[0].Rule, after.Rules[0].Rule)
	}
}

// TestReview_Reconcile_FixedFindingSuperseded is defect 4's end-to-end
// regression test through a real store: emit finding → fix the code →
// re-ingest → re-review persisting → the prior claim is superseded (absent
// = fixed) and no longer served.
func TestReview_Reconcile_FixedFindingSuperseded(t *testing.T) {
	root, storePath := reviewIngestFixture(t)

	runReviewJSON(t, storePath)
	claims1 := reviewSvcClaims(t, storePath)
	if len(claims1) != 1 || claims1[0].Status != core.ClaimFresh {
		t.Fatalf("expected exactly 1 fresh claim after run 1, got %+v", claims1)
	}
	claimID := claims1[0].ClaimID

	// Fix: remove the [US-777] reference and re-ingest.
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package pkg\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite fixture: %v", err)
	}
	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("re-ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	runReviewJSON(t, storePath)
	claims2 := reviewSvcClaims(t, storePath)
	if len(claims2) != 1 {
		t.Fatalf("expected the claim to remain (superseded, not deleted), got %d", len(claims2))
	}
	if claims2[0].ClaimID != claimID {
		t.Fatalf("claim identity changed: %s → %s", claimID, claims2[0].ClaimID)
	}
	if claims2[0].Status != core.ClaimSuperseded {
		t.Errorf("status after fixed re-review = %q, want superseded", claims2[0].Status)
	}
	if claims2[0].Status.Served() {
		t.Error("superseded claim must not be served")
	}
}

// TestReview_NoPersist_WritesNothing is defect 3's regression test: the
// --no-persist mode (what the pre-push hook uses) performs no store write
// of any kind — no claims, no supersedes — and reports
// findings_persisted:false.
func TestReview_NoPersist_WritesNothing(t *testing.T) {
	_, storePath := reviewIngestFixture(t)

	payload := runReviewJSON(t, storePath, "--no-persist")
	if payload["findings_persisted"] != false {
		t.Errorf("findings_persisted = %v, want false", payload["findings_persisted"])
	}
	if claims := reviewSvcClaims(t, storePath); len(claims) != 0 {
		t.Fatalf("--no-persist must write nothing, found %d claim(s)", len(claims))
	}
}
