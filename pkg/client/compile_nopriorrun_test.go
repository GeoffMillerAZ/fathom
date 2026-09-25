package client_test

// compile_nopriorrun_test.go — refactor-conservation gap-closing wave 3,
// item 3 (docs/plans/refactor-conservation.md, CV-41): CompileLayer's own
// wiring of semantic.CompileInput.NoPriorRun from the store's real
// compile_runs history. A project-local pack (writeProjectPack's own
// pattern, projectlayer_test.go) with a deliberately tiny
// max_tokens_per_run, and a TWO-section corpus so a mid-run stop genuinely
// leaves work uncompiled — the exact shape that refuses outright at the
// pre-flight gate on a plain re-run — proves the wiring end to end through
// pkg/client.CompileLayer, not internal/semantic.Compile in isolation
// (already covered by
// TestCompile_NoPriorRun_SkipsPreflightRefusal_AndBudgetStopsInstead).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// writeTinyBudgetPack mirrors writeProjectPack (projectlayer_test.go) but
// with a max_tokens_per_run small enough that promptOverheadTokens alone
// (260, internal/semantic/compile.go) already exceeds it — the pre-flight
// estimate over ANY non-empty remaining corpus refuses under the OLD,
// NoPriorRun-less behavior.
func writeTinyBudgetPack(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, semantic.ProjectLayersDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	layerCUE := `name:    "` + name + `"
version: "v1"
kind:    "project-kind"
corpus: include: ["docs/**"]
prompt_version: "` + name + `/fixture"
prompt_file:    "` + name + `/prompt.md"
tier:           "doc-claimed"
budgets: {
	max_tokens_per_run: 100
	max_usd_per_run:    1000
}
`
	if err := os.WriteFile(filepath.Join(dir, "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatalf("write layer.cue: %v", err)
	}
	prompt := `You compile widget rules from governing documents into strict JSON.
Output ONLY a JSON object {"claims": [...]} whose claims array (possibly
empty) holds objects of the shape {"rule", "scope", "cite_start", "cite_end"}.
`
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o644); err != nil {
		t.Fatalf("write prompt.md: %v", err)
	}
}

// TestClient_CompileLayer_FirstCompileOverBudget_BudgetStopsNotRefuses
// pins the reported gap directly: a service's VERY FIRST compile of a
// layer whose whole corpus estimate exceeds max_tokens_per_run must make
// real progress and stop as BudgetStopped, never refuse outright with zero
// claims — before this fix, CV-41's new default layer set could never
// activate adr-rules/spec-bindings on a real repository bigger than one
// run's budget without an operator already passing --continue for a layer
// that had never compiled anything. The SECOND call, once a real
// compile_runs row exists and work genuinely remains, still refuses at the
// pre-flight gate exactly as before (a re-compile is not exempted).
func TestClient_CompileLayer_FirstCompileOverBudget_BudgetStopsNotRefuses(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCorpusFile(t, filepath.Join(root, "mod", "widget.go"), projectLayerModSrc)
	writeCorpusFile(t, filepath.Join(root, "docs", "a.md"), "# A\n\n- Rule in A.\n")
	writeCorpusFile(t, filepath.Join(root, "docs", "b.md"), "# B\n\n- Rule in B.\n")
	writeTinyBudgetPack(t, root, "tiny-budget")

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// docs/a.md's section alone: real usage crosses the 100-token budget, so
	// the run stops BEFORE docs/b.md is ever reached. scope names mod/**
	// (the one indexed Go file) — a scope matching no indexed file is
	// rejected by the compile-time scope-trap defense (ADR-016 §4.4),
	// exactly like projectlayer_test.go's own working claim.
	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text:  `{"claims":[{"rule":"Rule in A.","scope":["mod/**"],"cite_start":3,"cite_end":3}]}`,
		Usage: llmclient.Usage{InputTokens: 150, OutputTokens: 50},
	}})
	c.WithLLM(mock, "mock-model", 0.001, 0.005)

	run, err := c.CompileLayer(ctx, "svc", "tiny-budget", false)
	if err != nil {
		t.Fatalf("CompileLayer (first compile, over budget): %v", err)
	}
	if run.Refused {
		t.Errorf("run.Refused = true, want false — this is the service's very first compile of this layer")
	}
	if !run.BudgetStopped {
		t.Errorf("run.BudgetStopped = false, want true")
	}
	if run.SectionsCompiled != 1 || run.ClaimsEmitted != 1 {
		t.Errorf("compiled/emitted = %d/%d, want 1/1 — a first compile must persist real progress (docs/a.md), not zero", run.SectionsCompiled, run.ClaimsEmitted)
	}
	if mock.CallCount() != 1 {
		t.Errorf("CallCount = %d, want 1 — docs/b.md must never be reached", mock.CallCount())
	}

	// The SECOND call against the SAME (service, layer) now has a real
	// compile_runs row (the budget-stopped one above), AND docs/b.md still
	// genuinely needs compiling — it must revert to the ordinary pre-flight
	// refusal, exactly the "pass --continue" guidance a budget-stopped run's
	// own output already names.
	mock2 := llmclient.NewMockClient() // no entries: a real call would prove the gate leaked
	c.WithLLM(mock2, "mock-model", 0.001, 0.005)
	run2, err := c.CompileLayer(ctx, "svc", "tiny-budget", false)
	if err == nil {
		t.Fatal("second compile against the same over-budget layer should refuse at the pre-flight gate")
	}
	if !run2.Refused {
		t.Errorf("second run.Refused = false, want true — a prior compile_run now exists and docs/b.md still needs compiling")
	}
	if mock2.CallCount() != 0 {
		t.Errorf("second CallCount = %d, want 0 — a refused run must never call the LLM", mock2.CallCount())
	}
}

// TestClient_CompileLayer_ResumeAfterBudgetStop_MakesFurtherProgress proves
// the "resumable with --continue, as today" half of the fix: after the
// first-compile budget stop above, an explicit CompileLayerResume call
// (mirroring `fathom compile --continue`) makes further progress on the
// section the first call never reached, exactly as an ordinary re-compile
// already does — NoPriorRun changes nothing about the EXISTING --continue
// path, only about the very first call.
func TestClient_CompileLayer_ResumeAfterBudgetStop_MakesFurtherProgress(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCorpusFile(t, filepath.Join(root, "mod", "widget.go"), projectLayerModSrc)
	writeCorpusFile(t, filepath.Join(root, "docs", "a.md"), "# A\n\n- Rule in A.\n")
	writeCorpusFile(t, filepath.Join(root, "docs", "b.md"), "# B\n\n- Rule in B.\n")
	writeTinyBudgetPack(t, root, "tiny-budget")

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text:  `{"claims":[{"rule":"Rule in A.","scope":["mod/**"],"cite_start":3,"cite_end":3}]}`,
		Usage: llmclient.Usage{InputTokens: 150, OutputTokens: 50},
	}})
	c.WithLLM(mock, "mock-model", 0.001, 0.005)
	if _, err := c.CompileLayer(ctx, "svc", "tiny-budget", false); err != nil {
		t.Fatalf("first compile: %v", err)
	}

	// --continue must not refuse, and must compile docs/b.md (the section
	// the first call never reached).
	mock2 := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Rule in B.","scope":["mod/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	c.WithLLM(mock2, "mock-model", 0.001, 0.005)
	run2, err := c.CompileLayerResume(ctx, "svc", "tiny-budget", false, "")
	if err != nil {
		t.Fatalf("CompileLayerResume: %v", err)
	}
	if run2.Refused {
		t.Errorf("resumed run.Refused = true, want false")
	}
	if run2.SectionsCompiled != 1 || run2.ClaimsEmitted != 1 {
		t.Errorf("resumed compiled/emitted = %d/%d, want 1/1 (docs/b.md only)", run2.SectionsCompiled, run2.ClaimsEmitted)
	}
}
