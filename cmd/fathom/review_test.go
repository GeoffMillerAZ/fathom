package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	storePath := filepath.Join(t.TempDir(), "s.duckdb")

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
	storePath := filepath.Join(t.TempDir(), "s.duckdb")

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
