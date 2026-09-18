package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

// TestReview_FormatGitHub_FauxtownGolden is VB-22's golden test: a real
// ingest of the fauxtown payment-service fixture, `fathom review --format
// github` against a scripted diff touching payment.go, and an assertion on
// the deterministic Markdown shape — the marker line, the changed-symbol
// section with call-site completeness and a caller's resolution tier, the
// test-gap section (the fixture has no _test.go), no coverage block (no
// profile supplied), and a byte-identical second render (VB-22's
// determinism requirement).
func TestReview_FormatGitHub_FauxtownGolden(t *testing.T) {
	paymentPath := fauxPaymentPathForTest(t)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-gh-svc", "--path", paymentPath, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	runOnce := func() string {
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{
				"--service", "review-gh-svc", "--since", "HEAD~1", "--store", storePath,
				"--format", "github", "--no-persist",
				"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
			},
			fakeGitRunner([]string{"payment.go"}), &out, &errBuf,
		)
		if code != 0 {
			t.Fatalf("review --format github exit = %d, stderr: %s", code, errBuf.String())
		}
		return out.String()
	}

	out := runOnce()

	if !strings.HasPrefix(out, review.GitHubReviewMarker) {
		t.Fatalf("output does not open with %q:\n%s", review.GitHubReviewMarker, out)
	}
	if !strings.Contains(out, "Index commit `") {
		t.Errorf("missing index commit header: %s", out)
	}
	if !strings.Contains(out, "diff range `HEAD~1..HEAD`") {
		t.Errorf("missing diff range header: %s", out)
	}
	if !strings.Contains(out, "### Changed symbols") {
		t.Errorf("missing Changed symbols section: %s", out)
	}
	if !strings.Contains(out, "validateCharge") {
		t.Errorf("expected validateCharge among the changed symbols: %s", out)
	}
	if !strings.Contains(out, "called from 1 place(s), 1 updated in this diff") {
		t.Errorf("expected validateCharge's call-site completeness line (called once, from Charge, also in this diff): %s", out)
	}
	if !strings.Contains(out, "[heuristic, depth 1]") {
		t.Errorf("expected at least one direct heuristic-tier caller (this fixture carries no typed edges from a real Go module): %s", out)
	}
	if !strings.Contains(out, "### Test gap") {
		t.Errorf("missing Test gap section: %s", out)
	}
	if !strings.Contains(out, "### Verify") || !strings.Contains(out, "_no plan row anchors a symbol in this diff._") {
		t.Errorf("expected an honest empty Verify section (fixture has no docs/plans/): %s", out)
	}
	if strings.Contains(out, "### Coverage") {
		t.Errorf("coverage section should be absent with no --coverage-profile: %s", out)
	}

	// VB-22's determinism requirement: the SAME diff renders the identical
	// comment on a second run.
	again := runOnce()
	if out != again {
		t.Fatalf("fathom review --format github is not deterministic across two runs:\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}

// TestReview_FormatGitHub_OutFlag_WritesFile is VB-22/VB-23's `--out <file>`
// wiring: a GitHub Action step captures the rendered comment to a file (to
// hand to the comment-upsert step, docs/adoption/github-action.md) instead
// of parsing stdout. `--out` must write the IDENTICAL bytes `--format
// github` would otherwise print to stdout, and stdout itself must carry
// only a short confirmation line — never the report body twice.
func TestReview_FormatGitHub_OutFlag_WritesFile(t *testing.T) {
	paymentPath := fauxPaymentPathForTest(t)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-gh-out-svc", "--path", paymentPath, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	outPath := filepath.Join(t.TempDir(), "review-comment.md")
	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{
			"--service", "review-gh-out-svc", "--since", "HEAD~1", "--store", storePath,
			"--format", "github", "--no-persist", "--out", outPath,
			"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
		},
		fakeGitRunner([]string{"payment.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review --format github --out exit = %d, stderr: %s", code, errBuf.String())
	}

	written, rerr := os.ReadFile(outPath)
	if rerr != nil {
		t.Fatalf("reading --out file: %v", rerr)
	}
	if !strings.HasPrefix(string(written), review.GitHubReviewMarker) {
		t.Fatalf("--out file does not open with the marker:\n%s", written)
	}
	if !strings.Contains(string(written), "validateCharge") {
		t.Errorf("--out file missing validateCharge: %s", written)
	}

	if strings.Contains(out.String(), review.GitHubReviewMarker) {
		t.Errorf("stdout must not also carry the full report when --out is set: %s", out.String())
	}
	if !strings.Contains(out.String(), outPath) {
		t.Errorf("stdout should confirm where the report was written, got: %s", out.String())
	}
}
