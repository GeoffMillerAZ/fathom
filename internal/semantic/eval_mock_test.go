package semantic

// eval_mock_test.go — a deterministic, network-free proof that RunEval
// (eval.go) works end-to-end against llmclient.MockClient, not just against
// a real model behind FATHOM_EVAL_LLM=1. This is the exact lane `fathom
// module eval --mock` (cmd/fathom/module.go) and scripts/module-cold-run.sh
// exercise — a cold run must never touch a real key (repo mandate), and
// this test is what proves that lane is wired correctly without spending
// anything or needing FATHOM_EVAL_LLM=1 at all.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

func writeEvalFixture(t *testing.T, dir string) {
	t.Helper()
	// A single ATX heading section (no separate top-level "# Rules" heading)
	// so the compiler's section split (SplitMarkdownSections) produces
	// exactly one section — one LLM call, matching the single scripted
	// MockEntry each test below sends. Line 3 is the labeled rule's text;
	// line 4 is a second, deliberately UNLABELED sentence a false-positive
	// test can cite instead (citing line 3 would still match the labeled
	// rule via its citation text, regardless of the claim's own invented
	// "rule" wording — RunEval's rule-matching checks the cited document
	// text first).
	doc := "## Sorting\n\nAlways sort output before returning it.\nNever cache stale results without invalidation.\n"
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	labels := EvalLabels{
		Docs:   []string{"doc.md"},
		Probes: []string{"internal/mcp/server.go", "internal/store/sqlite.go"},
		Rules: []struct {
			Key               string   `json:"key"`
			CiteSubstr        string   `json:"cite_substr"`
			Governs           []string `json:"governs"`
			TrapMustNotGovern []string `json:"trap_must_not_govern"`
		}{
			{Key: "sort-output", CiteSubstr: "Always sort output", Governs: []string{"internal/mcp/server.go", "internal/store/sqlite.go"}},
		},
	}
	raw, err := json.Marshal(labels)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "labels.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunEval_MockLane_PassesGate proves RunEval scores a perfectly
// labeled, perfectly compiled fixture at precision 1.0 / recall 1.0 — the
// scripted MockClient response cites exactly the labeled substring and
// scopes exactly the labeled probes, so the (claim, probe) scoring in
// eval.go finds zero FP/FN.
func TestRunEval_MockLane_PassesGate(t *testing.T) {
	dir := t.TempDir()
	writeEvalFixture(t, dir)

	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Always sort output before returning it.","scope":["internal/mcp/**","internal/store/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer: %v", err)
	}
	fixedNow := func() time.Time { return time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC) }

	result, err := RunEval(context.Background(), mock, "mock-model", layer, dir, fixedNow)
	if err != nil {
		t.Fatalf("RunEval: %v", err)
	}
	if !result.PassesGate() {
		t.Fatalf("expected the mock lane's perfectly-labeled fixture to pass the gate, got precision=%.2f recall=%.2f (tp=%d fp=%d fn=%d) missed=%v",
			result.Precision, result.Recall, result.TP, result.FP, result.FN, result.MissedRuleKeys)
	}
	if result.Run.ClaimsEmitted != 1 {
		t.Errorf("ClaimsEmitted = %d, want 1", result.Run.ClaimsEmitted)
	}
}

// TestRunEval_MockLane_FalsePositiveFailsGate proves the gate actually
// bites: an unlabeled, invented claim that also applies to a probe is
// scored as a false positive and drags precision below 0.90.
func TestRunEval_MockLane_FalsePositiveFailsGate(t *testing.T) {
	dir := t.TempDir()
	writeEvalFixture(t, dir)

	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"An invented rule nobody labeled.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer: %v", err)
	}
	fixedNow := func() time.Time { return time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC) }

	result, err := RunEval(context.Background(), mock, "mock-model", layer, dir, fixedNow)
	if err != nil {
		t.Fatalf("RunEval: %v", err)
	}
	if result.PassesGate() {
		t.Fatalf("expected the unlabeled-claim fixture to FAIL the gate, got precision=%.2f recall=%.2f", result.Precision, result.Recall)
	}
}

func TestRunEval_MissingLabelsJSON(t *testing.T) {
	dir := t.TempDir()
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer: %v", err)
	}
	if _, err := RunEval(context.Background(), llmclient.NewMockClient(), "mock-model", layer, dir, nil); err == nil {
		t.Fatal("expected an error for a fixtures dir with no labels.json")
	}
}
