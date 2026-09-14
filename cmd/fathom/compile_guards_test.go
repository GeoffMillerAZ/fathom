package main

// compile_guards_test.go — TDD coverage for two compile-time guards that
// share cmdCompile's --store direct-mode edit region:
//
//   - A2-1 (docs/plans/activation.md): idempotency guard — a non-dry-run
//     compile refuses when the store's latest compile_run for (service,
//     layer) already ran at the same corpus watermark AND the same prompt
//     identity; --force is the logged escape.
//   - D3-1 (docs/plans/expert-depth.md): eval-gate — a non-dry-run compile
//     of a layer with no passing eval/RESULTS.md record (LayerDef.Evaled ==
//     false) refuses; --unsafe-skip-eval-gate is the logged escape.
//
// Both escapes are "logged into compile_runs" via core.CompileRun.Escape —
// proven here by reading the store back after an override run.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// fakeClaudeCLIClaimsScript is a hermetic stand-in for the `claude` binary
// that always returns a syntactically valid (but empty) claims envelope, so
// a compile started against it always completes cleanly (SectionsCompiled,
// zero claims) regardless of what corpus text it was actually asked about —
// these guard tests care about whether a run happened and what got logged
// into compile_runs, not claim content.
const fakeClaudeCLIClaimsScript = `#!/bin/sh
cat > /dev/null
printf '%s' '{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"result":"{\"claims\":[]}","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}'
exit 0
`

func newFakeClaudeCLIClaims(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte(fakeClaudeCLIClaimsScript), 0o755); err != nil {
		t.Fatalf("write fake claude script: %v", err)
	}
	return path
}

// newGuardTestStore creates an on-disk sqlite store and ingests a minimal
// "svc" service (a CLAUDE.md for the obligations corpus, a findings/ doc for
// the findings corpus) so both cmdCompile (opening the same store path
// afresh) and this test's own assertions see the same data.
func newGuardTestStore(t *testing.T) (storePath, root string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Rules\n\n- Run tests before committing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "findings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "findings", "f1.md"), []byte("# Finding 1\n\nSomething was found.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	storePath = filepath.Join(t.TempDir(), "guard.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	if _, err := c.Ingest(context.Background(), client.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	return storePath, root
}

func countCompileRuns(t *testing.T, storePath, service, layer string) int {
	t.Helper()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	runs, err := st.CompileRuns(context.Background(), service, layer)
	if err != nil {
		t.Fatalf("CompileRuns: %v", err)
	}
	return len(runs)
}

func latestCompileRunEscape(t *testing.T, storePath, service, layer string) string {
	t.Helper()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	runs, err := st.CompileRuns(context.Background(), service, layer)
	if err != nil {
		t.Fatalf("CompileRuns: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("no compile_runs recorded for (%s, %s)", service, layer)
	}
	return runs[0].Escape // newest-first
}

// ─── D3-1: eval-gate ──────────────────────────────────────────────────────

func TestCmdCompile_EvalGate_RefusesUnEvaledLayer(t *testing.T) {
	storePath, _ := newGuardTestStore(t)
	// findings is shipped without eval/ yet (docs/layers.md) — LayerDef.Evaled
	// is false. No LLM provider is configured at all: a correct guard refuses
	// BEFORE ever trying to build an LLM client.
	t.Setenv("FATHOM_LLM_PROVIDER", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "findings", "--store", storePath, "--dry-run=false",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (refused); stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "eval") {
		t.Errorf("stderr should explain the eval-gate refusal, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--unsafe-skip-eval-gate") {
		t.Errorf("stderr should name the escape flag, got %q", stderr.String())
	}
	if n := countCompileRuns(t, storePath, "svc", "findings"); n != 0 {
		t.Errorf("a refused compile must not write a compile_run row, got %d", n)
	}
}

func TestCmdCompile_EvalGate_UnsafeSkipOverridesAndLogsEscape(t *testing.T) {
	storePath, _ := newGuardTestStore(t)
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", newFakeClaudeCLIClaims(t))

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "findings", "--store", storePath, "--dry-run=false",
		"--unsafe-skip-eval-gate",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (override should let the compile proceed); stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if n := countCompileRuns(t, storePath, "svc", "findings"); n != 1 {
		t.Fatalf("expected exactly 1 compile_run after the override run, got %d", n)
	}
	escape := latestCompileRunEscape(t, storePath, "svc", "findings")
	if escape == "" {
		t.Fatal("compile_runs.escape must be non-empty when --unsafe-skip-eval-gate overrode a refusal (D3-1: \"logged into compile_runs\")")
	}
	if !strings.Contains(escape, "unsafe-skip-eval-gate") {
		t.Errorf("escape note = %q, want it to name unsafe-skip-eval-gate", escape)
	}
}

// TestCmdCompile_EvalGate_ProjectPackEvaledLieStillRefused replicates the
// D3-1 bounce finding-2 executed attack END TO END through the CLI: a
// project pack at <service root>/.fathom/layers/bogus/ declaring
// `evaled: true` with NO eval/ directory used to compile with exit 0 and an
// empty escape_note. The union loader now overrides the lie to
// Evaled=false (internal/semantic/packs.go), so the compile eval-gate must
// refuse exactly like any other un-evaled layer.
func TestCmdCompile_EvalGate_ProjectPackEvaledLieStillRefused(t *testing.T) {
	storePath, root := newGuardTestStore(t)
	packDir := filepath.Join(root, ".fathom", "layers", "bogus")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	layerCUE := `name:    "bogus"
version: "v1"
kind:    "bogus-kind"
corpus: include: ["docs/**"]
prompt_version: "bogus/fixture"
prompt_file:    "bogus/prompt.md"
tier:           "doc-claimed"
evaled:         true
budgets: {
	max_tokens_per_run: 1000
	max_usd_per_run:    0.1
}
`
	if err := os.WriteFile(filepath.Join(packDir, "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "prompt.md"), []byte("Emit {\"claims\": []}."), 0o644); err != nil {
		t.Fatal(err)
	}
	// No eval/ directory at all — the critic's exact scenario.

	t.Setenv("FATHOM_LLM_PROVIDER", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "bogus", "--store", storePath, "--dry-run=false",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (a project pack's evaled lie must not pass the gate); stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "eval") {
		t.Errorf("stderr should explain the eval-gate refusal, got %q", stderr.String())
	}
	if n := countCompileRuns(t, storePath, "svc", "bogus"); n != 0 {
		t.Errorf("a refused compile must not write a compile_run row, got %d", n)
	}
}

// ─── A2-1: idempotency guard ──────────────────────────────────────────────

func TestCmdCompile_Idempotency_RefusesSameWatermarkAndPrompt(t *testing.T) {
	storePath, _ := newGuardTestStore(t)
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", newFakeClaudeCLIClaims(t))

	// First compile: obligations is evaled:true, so only the idempotency
	// guard is in play, and there is no prior run yet — it must succeed.
	var out1, err1 bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run=false",
	}, &out1, &err1)
	if code != 0 {
		t.Fatalf("first compile: exit code = %d, want 0; stdout=%q stderr=%q", code, out1.String(), err1.String())
	}
	if n := countCompileRuns(t, storePath, "svc", "obligations"); n != 1 {
		t.Fatalf("expected 1 compile_run after the first compile, got %d", n)
	}

	// Second compile: nothing on disk changed (same watermark, same prompt
	// identity) — must refuse without writing a second row.
	var out2, err2 bytes.Buffer
	code = cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run=false",
	}, &out2, &err2)
	if code != 1 {
		t.Fatalf("second (idempotent) compile: exit code = %d, want 1 (refused); stdout=%q stderr=%q", code, out2.String(), err2.String())
	}
	if !strings.Contains(err2.String(), "--force") {
		t.Errorf("refusal stderr should name --force, got %q", err2.String())
	}
	if n := countCompileRuns(t, storePath, "svc", "obligations"); n != 1 {
		t.Errorf("a refused idempotent compile must not write a second compile_run row, got %d", n)
	}
}

func TestCmdCompile_Idempotency_ForceOverridesAndLogsEscape(t *testing.T) {
	storePath, _ := newGuardTestStore(t)
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", newFakeClaudeCLIClaims(t))

	var out1, err1 bytes.Buffer
	if code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run=false",
	}, &out1, &err1); code != 0 {
		t.Fatalf("first compile: exit code = %d, want 0; stdout=%q stderr=%q", code, out1.String(), err1.String())
	}

	var out2, err2 bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run=false",
		"--force",
	}, &out2, &err2)
	if code != 0 {
		t.Fatalf("forced recompile: exit code = %d, want 0; stdout=%q stderr=%q", code, out2.String(), err2.String())
	}
	if n := countCompileRuns(t, storePath, "svc", "obligations"); n != 2 {
		t.Fatalf("expected 2 compile_run rows after the forced recompile, got %d", n)
	}
	escape := latestCompileRunEscape(t, storePath, "svc", "obligations")
	if escape == "" {
		t.Fatal("compile_runs.escape must be non-empty when --force overrode a refusal (A2-1: \"logged escape\")")
	}
	if !strings.Contains(escape, "force") {
		t.Errorf("escape note = %q, want it to name force", escape)
	}
}

// TestCmdCompile_Idempotency_AllowsResumeAfterBudgetStop is the bounce
// finding-4 honesty fix: a BudgetStopped run's own documented remedy is
// "re-run to resume; the digest gate skips completed sections" — the guard
// must not refuse that resume (which would force --force on the honest path
// and pollute the escape counts the kill criteria read). Pre-insert a
// budget-stopped compile_run at the CURRENT watermark + prompt identity,
// then prove a plain re-run proceeds without --force.
func TestCmdCompile_Idempotency_AllowsResumeAfterBudgetStop(t *testing.T) {
	storePath, root := newGuardTestStore(t)
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", newFakeClaudeCLIClaims(t))

	// Compute the exact (watermark, prompt identity) the guard will compare
	// against, then record a budget-stopped run carrying both.
	ctx := context.Background()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	watermark, err := c.ServiceWatermark(ctx, "svc")
	if err != nil || watermark == "" {
		t.Fatalf("ServiceWatermark: %q, %v (need a non-empty watermark for the guard to engage)", watermark, err)
	}
	layerDef, err := semantic.LoadLayerIn(root, "obligations")
	if err != nil {
		t.Fatalf("LoadLayerIn: %v", err)
	}
	promptIdentity, err := semantic.PromptIdentity(layerDef)
	if err != nil {
		t.Fatalf("PromptIdentity: %v", err)
	}
	if err := st.RecordCompileRun(ctx, core.CompileRun{
		RunID: "stopped-run", ServiceID: "svc", Layer: "obligations",
		CorpusWatermark: watermark, PromptVersion: promptIdentity,
		RanAt:         time.Now().UTC(),
		BudgetStopped: true, RefusalReason: "token total crossed max_tokens_per_run",
	}); err != nil {
		t.Fatalf("RecordCompileRun: %v", err)
	}
	c.Close()
	st.Close()

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run=false",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("resume after a budget-stopped run: exit code = %d, want 0 (no --force needed); stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "--force") {
		t.Errorf("resume must not be told to use --force, stderr=%q", stderr.String())
	}
	// The resume ran and recorded its own row (2 total), with no escape.
	if n := countCompileRuns(t, storePath, "svc", "obligations"); n != 2 {
		t.Fatalf("expected 2 compile_run rows after the resume, got %d", n)
	}
	if escape := latestCompileRunEscape(t, storePath, "svc", "obligations"); escape != "" {
		t.Errorf("a resume is the honest path — escape must be empty, got %q", escape)
	}
}

// ─── O54 deliverable 3: --continue ─────────────────────────────────────────

// TestCmdCompile_Continue_RecordsResumedFrom: `--continue` routes through
// CompileLayerResume and the recorded compile_runs row names the section
// it resumed from — proof, not a claim, that the CLI verb actually reached
// semantic.CompileInput.Resume.
func TestCmdCompile_Continue_RecordsResumedFrom(t *testing.T) {
	storePath, _ := newGuardTestStore(t)
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", newFakeClaudeCLIClaims(t))

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run=false", "--continue",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("--continue: exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	wantSubject := "CLAUDE.md#rules"
	if !strings.Contains(stdout.String(), "resumed_from:      "+wantSubject) {
		t.Errorf("stdout must print resumed_from naming %q, got %q", wantSubject, stdout.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	runs, err := st.CompileRuns(context.Background(), "svc", "obligations")
	if err != nil || len(runs) != 1 {
		t.Fatalf("CompileRuns: %d, %v", len(runs), err)
	}
	if runs[0].ResumedFrom != wantSubject {
		t.Errorf("recorded run.ResumedFrom = %q, want %q", runs[0].ResumedFrom, wantSubject)
	}
}

// ─── HTTP mode refuses the guard flags outright ───────────────────────────

// TestCmdCompile_GuardFlagsRefusedWithoutStore (bounce finding 6): the
// guards exist only on the direct --store lane; passing either override
// flag in HTTP mode must refuse loudly (naming --store) rather than
// silently ignoring the flag — before any HTTP request is attempted.
func TestCmdCompile_GuardFlagsRefusedWithoutStore(t *testing.T) {
	for _, flag := range []string{"--force", "--unsafe-skip-eval-gate", "--continue"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := cmdCompile([]string{
				"--service", "svc", "--layer", "obligations", flag,
			}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1 (guard flags apply to --store mode only); stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "--store") {
				t.Errorf("stderr should name --store as the way out, got %q", stderr.String())
			}
		})
	}
}

// ─── dry-run is never gated ───────────────────────────────────────────────

// TestCmdCompile_DryRun_NeverGuardGated proves both guards apply only to a
// spending compile (dry_run defaults true, per compile.go's long-standing
// contract): an un-evaled layer's dry-run estimate must still work with no
// escape flag, since nothing is spent or written.
func TestCmdCompile_DryRun_NeverGuardGated(t *testing.T) {
	storePath, _ := newGuardTestStore(t)
	t.Setenv("FATHOM_LLM_PROVIDER", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "findings", "--store", storePath, "--dry-run=true",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run of an un-evaled layer should never be gated, got exit %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
