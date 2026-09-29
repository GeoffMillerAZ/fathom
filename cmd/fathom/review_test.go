package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
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
		[]string{
			"--service", "review-svc", "--since", "HEAD~1", "--store", storePath, "--json",
			"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
		},
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
	if !ok || len(dims) != 8 {
		// 8 (ADR-044 CV-12/CV-24): impact, obligations, arch_conformance,
		// spec_coverage, coverage, certifications, integrity, conservation.
		t.Fatalf("expected 8 dimensions, got %v", payload["dimensions"])
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
	// The "coverage" dimension is always present (RC-41) — checked:false
	// here since this run passed no --coverage-profile at all, the SAME
	// honesty discipline arch_conformance's own unchecked case demonstrates.
	var covDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "coverage" {
			covDim = d
		}
	}
	if covDim == nil {
		t.Fatal("missing coverage dimension")
	}
	if covDim["checked"] != false || covDim["reason"] != "no coverage profile supplied" {
		t.Errorf("coverage dimension = %+v", covDim)
	}

	t.Logf("one-call worked example output:\n%s", out.String())
}

// TestReview_CoverageProfile proves RC-41's --coverage-profile flag end to
// end: a real cover profile (func A's own [Line, EndLine] span, resolved
// from a real ingest — the same discipline testsCoverageFixture in
// tests_coverage_test.go uses) produces a checked "coverage" dimension
// carrying func A's joined statement coverage.
func TestReview_CoverageProfile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "cov-review-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	nodes, err := c.ListAllSymbols(context.Background(), "cov-review-svc")
	if err != nil {
		t.Fatalf("list symbols: %v", err)
	}
	c.Close()
	st.Close()
	var fn core.SymbolNode
	for _, n := range nodes {
		if n.Name == "A" {
			fn = n
		}
	}
	if fn.Name == "" {
		t.Fatalf("func A not found among indexed symbols: %+v", nodes)
	}

	profilePath := filepath.Join(t.TempDir(), "cover.out")
	body := fmt.Sprintf("mode: set\npkg/a.go:%d.1,%d.1 1 1\n", fn.Line, fn.EndLine)
	if err := os.WriteFile(profilePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{
			"--service", "cov-review-svc", "--since", "HEAD~1", "--store", storePath, "--json",
			"--no-persist", "--coverage-profile", profilePath,
			"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
		},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	dims := payload["dimensions"].([]any)
	var covDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "coverage" {
			covDim = d
		}
	}
	if covDim == nil {
		t.Fatal("missing coverage dimension")
	}
	if covDim["checked"] != true {
		t.Fatalf("expected checked:true with --coverage-profile supplied, got %+v", covDim)
	}
	items, _ := covDim["coverage"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected exactly one changed-symbol coverage entry (func A), got %+v", covDim["coverage"])
	}
}

// TestReview_CoverageProfile_TwoLayers proves --coverage-profile's real
// "by layer" attribution end to end: two separately-run profiles, each
// labeled with its own test layer, produce a coverage entry naming which
// layer covers the symbol and which does not.
func TestReview_CoverageProfile_TwoLayers(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "cov-review-layers-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	nodes, err := c.ListAllSymbols(context.Background(), "cov-review-layers-svc")
	if err != nil {
		t.Fatalf("list symbols: %v", err)
	}
	c.Close()
	st.Close()
	var fn core.SymbolNode
	for _, n := range nodes {
		if n.Name == "A" {
			fn = n
		}
	}
	if fn.Name == "" {
		t.Fatalf("func A not found among indexed symbols: %+v", nodes)
	}

	unitPath := filepath.Join(t.TempDir(), "unit.out")
	if err := os.WriteFile(unitPath, []byte(fmt.Sprintf("mode: set\npkg/a.go:%d.1,%d.1 1 0\n", fn.Line, fn.EndLine)), 0o644); err != nil {
		t.Fatalf("write unit profile: %v", err)
	}
	intPath := filepath.Join(t.TempDir(), "integration.out")
	if err := os.WriteFile(intPath, []byte(fmt.Sprintf("mode: set\npkg/a.go:%d.1,%d.1 1 1\n", fn.Line, fn.EndLine)), 0o644); err != nil {
		t.Fatalf("write integration profile: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{
			"--service", "cov-review-layers-svc", "--since", "HEAD~1", "--store", storePath, "--json",
			"--no-persist",
			"--coverage-profile", "unit=" + unitPath,
			"--coverage-profile", "integration=" + intPath,
			"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
		},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	dims := payload["dimensions"].([]any)
	var covDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "coverage" {
			covDim = d
		}
	}
	if covDim == nil {
		t.Fatal("missing coverage dimension")
	}
	items, _ := covDim["coverage"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected exactly one changed-symbol coverage entry (func A), got %+v", covDim["coverage"])
	}
	entry := items[0].(map[string]any)
	layers, _ := entry["layers"].([]any)
	if len(layers) != 1 || layers[0] != "integration" {
		t.Errorf("layers = %v, want [integration] (only that layer covers it)", entry["layers"])
	}
	uncoveredAt, _ := entry["uncovered_at"].([]any)
	if len(uncoveredAt) != 1 || uncoveredAt[0] != "unit" {
		t.Errorf("uncovered_at = %v, want [unit]", entry["uncovered_at"])
	}
	if entry["uncovered"] != false {
		t.Errorf("uncovered should be false — the union is covered by integration: %+v", entry)
	}
}

// TestReview_CoverageProfile_BadPathFailsLoudly proves a SUPPLIED but
// unreadable --coverage-profile is a hard error, unlike an ABSENT one
// (which is an honest checked:false) — the caller explicitly asked for
// coverage and got a bad file.
func TestReview_CoverageProfile_BadPathFailsLoudly(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{"--service", "svc", "--since", "origin/main", "--coverage-profile", "/does/not/exist.out"},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code == 0 {
		t.Fatal("expected a nonzero exit for an unreadable --coverage-profile")
	}
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
			[]string{
				"--service", "review-svc", "--since", "HEAD~1", "--store", storePath, "--json",
				"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
			},
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

// runReviewJSON runs a persisting review over pkg/a.go and returns the
// payload. --call-log is pinned to a per-test temp path (T5-7 made review
// append a call-log row on every reachable run) so these tests never write
// into the package's own working directory via resolveCallLogPath's
// relative default.
func runReviewJSON(t *testing.T, storePath string, extraArgs ...string) map[string]any {
	t.Helper()
	callLogPath := filepath.Join(t.TempDir(), "calls.jsonl")
	args := append([]string{
		"--service", "review-svc", "--since", "HEAD~1", "--store", storePath, "--json",
		"--call-log", callLogPath,
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

	// Q-7 (docs/plans/expert-suite-q.md): the reconcile must also leave a
	// durable "superseded" claim_event — the real mechanism Interventions
	// row 2 (docs/design/observability-surfaces.md §1) reads.
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	events, err := st.ClaimEvents(context.Background(), claimID)
	if err != nil {
		t.Fatalf("ClaimEvents: %v", err)
	}
	var supersededEvents int
	for _, ev := range events {
		if ev.Verdict == "superseded" {
			supersededEvents++
			if ev.Author != "review" {
				t.Errorf("superseded event author = %q, want %q", ev.Author, "review")
			}
			// docs/design/findings-as-evidence.md §3.4, §6 item 4: the
			// scoped file (pkg/a.go) still exists — it was EDITED, not
			// deleted — so this is a code-change resolution, with a
			// non-empty ResolutionCommit naming the fix's watermark.
			if ev.ResolutionKind != core.ResolutionCodeChange {
				t.Errorf("superseded event ResolutionKind = %q, want %q", ev.ResolutionKind, core.ResolutionCodeChange)
			}
			if ev.ResolutionCommit == "" {
				t.Error("superseded event ResolutionCommit is empty, want the fix's watermark")
			}
		}
	}
	if supersededEvents != 1 {
		t.Fatalf("superseded claim_events for %s = %d, want 1; events=%+v", claimID, supersededEvents, events)
	}
}

// TestReview_Reconcile_ChangedFindingWritesSupersedesChain (SU-19/SU-20,
// ADR-041 decision 7): a re-review that re-emits the SAME finding subject
// under DIFFERENT content (the spec id referenced changes, so the
// content-addressed claim id changes too, twice) supersedes the OLD claim
// with a real successor — closing its bi-temporal validity window
// (SetClaimValidTo) and recording an explicit "supersedes" claim link
// (WriteSupersedes) — and semantic.SupersessionChain can walk the full
// two-hop lineage back from the newest claim.
func TestReview_Reconcile_ChangedFindingWritesSupersedesChain(t *testing.T) {
	root, storePath := reviewIngestFixture(t)

	runReviewJSON(t, storePath)
	claims1 := reviewSvcClaims(t, storePath)
	if len(claims1) != 1 {
		t.Fatalf("expected exactly 1 claim after run 1, got %+v", claims1)
	}
	c1 := claims1[0].ClaimID

	// Change (not fix) the violation: a different spec id in the SAME
	// comment, same file — same finding subject, different message, so a
	// NEW content-addressed claim id is minted this run.
	rewrite := func(specID string) {
		src := "package pkg\n\nfunc A() {\n\t// satisfies [" + specID + "] but nothing implements it yet\n}\n"
		if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(src), 0o644); err != nil {
			t.Fatalf("rewrite fixture: %v", err)
		}
		var ingestOut, ingestErr bytes.Buffer
		if code := run([]string{"ingest", "--service", "review-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
			t.Fatalf("re-ingest exit = %d, stderr: %s", code, ingestErr.String())
		}
	}

	rewrite("US-778")
	runReviewJSON(t, storePath)
	claims2 := reviewSvcClaims(t, storePath)
	if len(claims2) != 2 {
		t.Fatalf("expected 2 claims after run 2 (old superseded, new fresh), got %+v", claims2)
	}
	var c2 string
	for _, cl := range claims2 {
		if cl.ClaimID != c1 {
			c2 = cl.ClaimID
		}
	}
	if c2 == "" {
		t.Fatalf("run 2 did not mint a new claim id: %+v", claims2)
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	oldAfterRun2, err := freshestByID(ctx, st, c1)
	if err != nil {
		t.Fatalf("re-read c1: %v", err)
	}
	if oldAfterRun2.Status != core.ClaimSuperseded {
		t.Fatalf("c1 status after run 2 = %q, want superseded", oldAfterRun2.Status)
	}
	if oldAfterRun2.ValidTo.IsZero() {
		t.Error("c1 ValidTo must be closed after being superseded (SU-19)")
	}

	links, err := st.ClaimLinks(ctx, "review-svc", []string{c2})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	foundSupersedes := false
	for _, l := range links {
		if l.Rel == core.RelSupersedes && l.TargetRef == c1 {
			foundSupersedes = true
		}
	}
	if !foundSupersedes {
		t.Fatalf("c2's links = %+v, want a supersedes link to c1", links)
	}

	rewrite("US-779")
	runReviewJSON(t, storePath)
	claims3 := reviewSvcClaims(t, storePath)
	var c3 string
	for _, cl := range claims3 {
		if cl.ClaimID != c1 && cl.ClaimID != c2 {
			c3 = cl.ClaimID
		}
	}
	if c3 == "" {
		t.Fatalf("run 3 did not mint a third claim id: %+v", claims3)
	}

	chain, err := semantic.SupersessionChain(ctx, st, "review-svc", c3)
	if err != nil {
		t.Fatalf("SupersessionChain: %v", err)
	}
	if len(chain) != 2 || chain[0] != c2 || chain[1] != c1 {
		t.Fatalf("SupersessionChain(c3) = %v, want [%s %s]", chain, c2, c1)
	}
}

// freshestByID returns the current stored claim for id, or an error if
// absent.
func freshestByID(ctx context.Context, st store.Store, id string) (core.SemanticClaim, error) {
	claims, err := st.ClaimsForService(ctx, "review-svc", "findings")
	if err != nil {
		return core.SemanticClaim{}, err
	}
	for _, cl := range claims {
		if cl.ClaimID == id {
			return cl, nil
		}
	}
	return core.SemanticClaim{}, fmt.Errorf("claim %s not found", id)
}

// TestReview_AssertedByIsIdempotencyKeyOrWatermark (SU-22, ADR-041 decision
// 7): a review-minted claim is asserted by --idempotency-key when given,
// else by its own corpus watermark.
func TestReview_AssertedByIsIdempotencyKeyOrWatermark(t *testing.T) {
	root, storePath := reviewIngestFixture(t)
	_ = root

	runReviewJSON(t, storePath, "--idempotency-key", "key-abc")
	claims := reviewSvcClaims(t, storePath)
	if len(claims) != 1 {
		t.Fatalf("expected exactly 1 claim, got %+v", claims)
	}
	want := core.AssertedBy("review", "key-abc")
	if claims[0].AssertedBy != want {
		t.Errorf("AssertedBy = %q, want %q", claims[0].AssertedBy, want)
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

// ─── T5-7: the forced-channel measurement (docs/plans/trust-and-tokenomics.md) ──

// TestReview_AppendsCallLogVerdictOutcome is T5-7's real evidence for the
// review CLI/hook path: C3-1's forced channel (`bin/fathom review
// --no-persist`, run by lefthook's pre-push step) never touches the MCP
// dispatch path, so before this feature its verdict was invisible to
// `instrument --adoption`. A real ingest + review run must append a
// call-log row carrying verdict_outcome keyed by exactly the CHECKED
// dimensions (spec_coverage is checked and finds the seeded [US-777] gap;
// impact and obligations are always checked by the CLI's direct-store
// client, at 0 findings for this fixture; arch_conformance is unchecked —
// no project rule pack configured — and must be ABSENT from the map, not
// present at 0).
func TestReview_AppendsCallLogVerdictOutcome(t *testing.T) {
	_, storePath := reviewIngestFixture(t)
	logPath := filepath.Join(t.TempDir(), "logs", "mcp-calls.jsonl")

	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{"--service", "review-svc", "--since", "HEAD~1", "--store", storePath, "--no-persist", "--call-log", logPath},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", errBuf.String())
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading call log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 call-log line, got %d: %q", len(lines), string(data))
	}
	var entry adoptionCallEntry
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("call-log line not JSON: %v (line=%q)", err, lines[0])
	}
	if entry.Tool != "review" {
		t.Errorf("Tool = %q, want review", entry.Tool)
	}
	if entry.Service != "review-svc" {
		t.Errorf("Service = %q, want review-svc", entry.Service)
	}
	if entry.Surface != "cli" {
		t.Errorf("Surface = %q, want cli", entry.Surface)
	}
	if entry.VerdictOutcome == nil {
		t.Fatal("VerdictOutcome is nil, want the per-checked-dimension counts")
	}
	if want := map[string]int{"impact": 0, "obligations": 0, "spec_coverage": 1, "integrity": 0}; !reflect.DeepEqual(entry.VerdictOutcome, want) {
		t.Errorf("VerdictOutcome = %v, want %v (arch_conformance absent — unchecked, no rule pack; integrity checked via --since with no findings)", entry.VerdictOutcome, want)
	}
}

// TestReview_CallLogDefaultsFromConfig proves the CLI defaults its
// call-log destination through resolveCallLogPath (the same config
// resolution instrument --adoption itself uses), when --call-log is not
// passed — so a repo that only ever configures [daemon].call_log gets its
// review rows in the log the daemon (and instrument --adoption) actually
// reads.
func TestReview_CallLogDefaultsFromConfig(t *testing.T) {
	_, storePath := reviewIngestFixture(t)
	// reviewIngestFixture and storePath are resolved BEFORE the chdir below
	// (withIsolatedConfigHome moves the working directory), so storePath
	// stays a valid absolute-enough path either way — t.TempDir() returns
	// absolute paths.
	_, projectDir := withIsolatedConfigHome(t)
	logPath := filepath.Join(projectDir, "custom", "review-calls.jsonl")
	writeProjectConfig(t, projectDir, `
[daemon]
call_log = "`+filepath.ToSlash(logPath)+`"
`)

	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{"--service", "review-svc", "--since", "HEAD~1", "--store", storePath, "--no-persist"},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected a call-log entry at the configured default path %q: %v", logPath, err)
	}
}

// ─── Q-7: review-finding supersede events (docs/plans/expert-suite-q.md) ───

// TestReview_NoPersist_NeverRecordsSupersedeEvents: --no-persist's whole
// point is "no store write of any kind" — a fixed-finding reconcile run
// under --no-persist must record ZERO superseded claim_events, not just
// leave the claim's status untouched.
func TestReview_NoPersist_NeverRecordsSupersedeEvents(t *testing.T) {
	root, storePath := reviewIngestFixture(t)
	runReviewJSON(t, storePath)
	claims1 := reviewSvcClaims(t, storePath)
	if len(claims1) != 1 {
		t.Fatalf("expected exactly 1 claim after run 1, got %+v", claims1)
	}
	claimID := claims1[0].ClaimID

	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package pkg\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite fixture: %v", err)
	}
	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("re-ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	runReviewJSON(t, storePath, "--no-persist")

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	events, err := st.ClaimEvents(context.Background(), claimID)
	if err != nil {
		t.Fatalf("ClaimEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("--no-persist must record zero claim_events, got %+v", events)
	}
}

// failingAppendEventStore wraps a real store.Store, delegating everything
// except AppendClaimEvent (which always fails) — a minimal test double for
// TestRecordReviewSupersedeEvents_StoreWriteFails_LoggedNotFatal.
type failingAppendEventStore struct{ store.Store }

func (failingAppendEventStore) AppendClaimEvent(context.Context, core.ClaimEvent) error {
	return errors.New("simulated AppendClaimEvent failure")
}

// TestRecordReviewSupersedeEvents_WritesOneEventPerID.
func TestRecordReviewSupersedeEvents_WritesOneEventPerID(t *testing.T) {
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	fixedNow := func() time.Time { return time.Unix(1700000000, 0).UTC() }
	var stderr bytes.Buffer
	resolve := func(claimID string) (string, string) { return core.ResolutionCodeChange, "abc123" }
	n := recordReviewSupersedeEvents(context.Background(), c, []string{"cl-1", "cl-2"}, resolve, fixedNow, &stderr)
	if n != 2 {
		t.Fatalf("recorded = %d, want 2", n)
	}
	for _, id := range []string{"cl-1", "cl-2"} {
		events, err := st.ClaimEvents(context.Background(), id)
		if err != nil || len(events) != 1 || events[0].Verdict != "superseded" || events[0].Author != "review" {
			t.Errorf("events for %s = %+v, err=%v", id, events, err)
		}
		if events[0].ResolutionKind != core.ResolutionCodeChange || events[0].ResolutionCommit != "abc123" {
			t.Errorf("events[0] resolution = %+v, want kind=%q commit=%q", events[0], core.ResolutionCodeChange, "abc123")
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty on success", stderr.String())
	}
}

// TestRecordReviewSupersedeEvents_NilResolve_EmptyResolutionFields proves
// resolve is optional: a nil resolve func writes events with empty
// ResolutionKind/ResolutionCommit rather than panicking.
func TestRecordReviewSupersedeEvents_NilResolve_EmptyResolutionFields(t *testing.T) {
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	fixedNow := func() time.Time { return time.Unix(1700000000, 0).UTC() }
	var stderr bytes.Buffer
	n := recordReviewSupersedeEvents(context.Background(), c, []string{"cl-1"}, nil, fixedNow, &stderr)
	if n != 1 {
		t.Fatalf("recorded = %d, want 1", n)
	}
	events, err := st.ClaimEvents(context.Background(), "cl-1")
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %+v, err=%v", events, err)
	}
	if events[0].ResolutionKind != "" || events[0].ResolutionCommit != "" {
		t.Errorf("events[0] = %+v, want empty resolution fields with nil resolve", events[0])
	}
}

// TestRecordReviewSupersedeEvents_NoIDs_NoOp.
func TestRecordReviewSupersedeEvents_NoIDs_NoOp(t *testing.T) {
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	var stderr bytes.Buffer
	n := recordReviewSupersedeEvents(context.Background(), c, nil, nil, time.Now, &stderr)
	if n != 0 {
		t.Fatalf("recorded = %d, want 0", n)
	}
}

// TestRecordReviewSupersedeEvents_StoreWriteFails_LoggedNotFatal: a write
// failure is logged to stderr and skipped, never panics, never aborts the
// remaining IDs.
func TestRecordReviewSupersedeEvents_StoreWriteFails_LoggedNotFatal(t *testing.T) {
	st := failingAppendEventStore{store.NewMemoryStore()}
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	var stderr bytes.Buffer
	n := recordReviewSupersedeEvents(context.Background(), c, []string{"cl-1", "cl-2"}, nil, time.Now, &stderr)
	if n != 0 {
		t.Fatalf("recorded = %d, want 0 (every write fails)", n)
	}
	if !strings.Contains(stderr.String(), "cl-1") || !strings.Contains(stderr.String(), "cl-2") {
		t.Errorf("stderr = %q, want both failed IDs named", stderr.String())
	}
}

// TestReviewNonInteractive pins IO-9 (observation O9, root cause corrected):
// the pre-push review never hangs a non-interactive push — with a budget set,
// a slow run prints a named advisory skip and exits 0 within the budget,
// instead of leaving lefthook's buffered output looking like a stall until
// an impatient wrapper kills the push.
func TestReviewNonInteractive(t *testing.T) {
	t.Run("budget_exceeded_names_the_skip_and_exits_zero", func(t *testing.T) {
		// The budgeted body blocks until the run has returned (the release
		// closes after), so runReview returning at all is "the budget bounded
		// the run"; the 1 s budget is the input.
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		slowGit := func(args ...string) ([]byte, error) {
			<-release
			return []byte("a.go\n"), nil
		}
		var stdout, stderr bytes.Buffer
		var code int
		returnsUnaided(t, "runReview over a body that blocks past its budget", func() {
			code = runReview([]string{"--service", "svc", "--since", "origin/main", "--no-persist", "--budget-seconds", "1", "--store", filepath.Join(t.TempDir(), "s.sqlite")}, slowGit, &stdout, &stderr)
		})
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (advisory is fail-open): %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "budget exceeded") {
			t.Errorf("skip must be named, got: %q", stdout.String())
		}
	})

	t.Run("budget_requires_no_persist", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := runReview([]string{"--service", "svc", "--since", "origin/main", "--budget-seconds", "5"}, func(args ...string) ([]byte, error) { return nil, nil }, &stdout, &stderr)
		if code == 0 {
			t.Fatal("budget without --no-persist must refuse: a bounded run must never be a store writer")
		}
		if !strings.Contains(stderr.String(), "no-persist") {
			t.Errorf("refusal should name the rule, got: %q", stderr.String())
		}
	})

	t.Run("fast_run_unaffected_by_budget", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := runReview([]string{"--service", "svc", "--since", "origin/main", "--no-persist", "--budget-seconds", "30"}, func(args ...string) ([]byte, error) { return []byte(""), nil }, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit = %d: %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "no changed files") {
			t.Errorf("normal path should complete, got: %q", stdout.String())
		}
	})
}

// TestCmdReviewReceiptFormats proves CX-6 (docs/plans/change-and-
// compliance.md, design doc §2): `fathom review --receipt --format
// sarif|check` renders the change risk receipt through the shared emitter
// (GL-20 verify.RenderSARIF, GL-21 verify.RenderCheckRun) instead of the
// full review — and refuses any other --format, since the design names
// exactly those two.
func TestCmdReviewReceiptFormats(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "receipt-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	t.Run("sarif", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{"--service", "receipt-svc", "--since", "HEAD~1", "--store", storePath, "--no-persist", "--receipt", "--format", "sarif"},
			fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
		)
		if code != 0 {
			t.Fatalf("review --receipt --format sarif exit = %d, stderr: %s", code, errBuf.String())
		}
		var doc map[string]any
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("unmarshal SARIF: %v\noutput: %s", err, out.String())
		}
		if doc["version"] != "2.1.0" {
			t.Errorf("SARIF version = %v, want 2.1.0", doc["version"])
		}
		if _, ok := doc["runs"]; !ok {
			t.Error("SARIF output missing \"runs\"")
		}
	})

	t.Run("check", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{"--service", "receipt-svc", "--since", "HEAD~1", "--store", storePath, "--no-persist", "--receipt", "--format", "check"},
			fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
		)
		if code != 0 {
			t.Fatalf("review --receipt --format check exit = %d, stderr: %s", code, errBuf.String())
		}
		var doc map[string]any
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("unmarshal check-run: %v\noutput: %s", err, out.String())
		}
		// CX-8/ADR-040: the receipt never scores a pass/fail verdict from
		// the band — the conclusion is always "neutral", whatever the
		// band. CI decides what a band does, fathom never blocks.
		if doc["conclusion"] != "neutral" {
			t.Errorf("check-run conclusion = %v, want \"neutral\" (CX-8: fathom never blocks on its own authority)", doc["conclusion"])
		}
		out2, ok := doc["output"].(map[string]any)
		if !ok {
			t.Fatalf("check-run missing \"output\": %v", doc)
		}
		if summary, _ := out2["summary"].(string); !strings.Contains(summary, "band:") {
			t.Errorf("check-run summary = %q, want it to name the band", summary)
		}
	})

	t.Run("receipt_requires_sarif_or_check_format", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{"--service", "receipt-svc", "--since", "HEAD~1", "--store", storePath, "--no-persist", "--receipt"},
			fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
		)
		if code == 0 {
			t.Fatal("--receipt with the default --format text must refuse")
		}
		if !strings.Contains(errBuf.String(), "--receipt") {
			t.Errorf("refusal should name --receipt, got: %q", errBuf.String())
		}
	})

	t.Run("receipt_false_still_allows_text_format", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{"--service", "receipt-svc", "--since", "HEAD~1", "--store", storePath, "--no-persist"},
			fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
		)
		if code != 0 {
			t.Fatalf("plain review exit = %d, stderr: %s", code, errBuf.String())
		}
		if !strings.Contains(out.String(), "review: receipt-svc") {
			t.Errorf("plain review output unaffected by --receipt wiring, got: %q", out.String())
		}
	})
}
