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
	"fmt"
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

// TestCmdCompile_Continue_AfterFullCompile_PrintsNothingToResume is O54's
// NIT: a second --continue against a layer that already fully compiled —
// same corpus watermark, same prompt identity as the run before it —
// bypasses the idempotency guard (resume IS the "yes, proceed" signal) and
// the compile itself finds nothing left to do, so the CLI prints one
// plain line instead of the full run dump.
//
// Unlike the other guard tests, this needs the FIRST run to actually
// WRITE a claim (newFakeClaudeCLIClaims's shared script always returns
// empty claims, which the OTHER tests don't care about but this one
// does — an empty-claims "compile" leaves nothing for the digest gate to
// match against, so the second call would recompile instead of finding
// nothing to resume). A dedicated fixture with one real indexed source
// file and a script that returns one valid, in-scope claim.
func TestCmdCompile_Continue_AfterFullCompile_PrintsNothingToResume(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Rules\n\n- Run tests before committing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "foo.go"), []byte("package pkg\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(t.TempDir(), "nit.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	if _, err := c.Ingest(context.Background(), client.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	c.Close()
	st.Close()

	claimScript := `#!/bin/sh
cat > /dev/null
printf '%s' '{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"result":"{\"claims\":[{\"rule\":\"Run tests before committing.\",\"scope\":[\"pkg/**\"],\"cite_start\":3,\"cite_end\":3}]}","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}'
exit 0
`
	binPath := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(binPath, []byte(claimScript), 0o755); err != nil {
		t.Fatalf("write fake claude script: %v", err)
	}
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", binPath)

	var stdout1, stderr1 bytes.Buffer
	if code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run=false",
	}, &stdout1, &stderr1); code != 0 {
		t.Fatalf("first compile: exit %d; stdout=%q stderr=%q", code, stdout1.String(), stderr1.String())
	}

	var stdout2, stderr2 bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run=false", "--continue",
	}, &stdout2, &stderr2)
	if code != 0 {
		t.Fatalf("second --continue: exit %d; stdout=%q stderr=%q", code, stdout2.String(), stderr2.String())
	}
	want := "layer obligations is complete: 1/1 sections compiled; nothing to resume\n"
	if stdout2.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout2.String(), want)
	}
	if stderr2.String() != "" {
		t.Errorf("stderr = %q, want empty — bypassing the idempotency guard via --continue is not an error path", stderr2.String())
	}

	// Both runs recorded; the second neither refused nor emitted a
	// (misleading, since nothing was overridden) --force escape note.
	if n := countCompileRuns(t, storePath, "svc", "obligations"); n != 2 {
		t.Fatalf("compile_runs rows = %d, want 2", n)
	}
	if escape := latestCompileRunEscape(t, storePath, "svc", "obligations"); escape != "" {
		t.Errorf("second run's Escape = %q, want empty — --continue is not an override", escape)
	}
}

// ─── HTTP mode refuses the guard flags outright ───────────────────────────

// TestCmdCompile_GuardFlagsRefusedWithoutStore (bounce finding 6): the
// guards exist only on the direct --store lane; passing either override
// flag in HTTP mode must refuse loudly (naming --store) rather than
// silently ignoring the flag — before any HTTP request is attempted.
func TestCmdCompile_GuardFlagsRefusedWithoutStore(t *testing.T) {
	for _, flag := range []string{"--force", "--unsafe-skip-eval-gate", "--continue", "--max-tokens-per-run=all", "--max-usd-per-run=all"} {
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

// ─── wave O4, item 2: --max-tokens-per-run / --max-usd-per-run ────────────

// newBudgetStopFixture builds a store whose "obligations" corpus has 5
// small, distinct nested-guide sections (g1..g5/CLAUDE.md) and a project
// layer pack overriding "obligations" with a deliberately tiny
// max_tokens_per_run — small enough that the PRE-FLIGHT cost estimate for
// this whole 5-section corpus (cost.Estimate's bytes/4 heuristic plus the
// fixed 260-token prompt overhead per section, summed over every section
// needing compile) already exceeds it, so an unmodified compile refuses
// outright before calling the LLM for even one section — the layer's own
// default budget standing between the corpus and ANY progress, exactly the
// shape vs-pilot-5's second reading found (a fixed run budget too small for
// the corpus it was asked to cover). Uses the shared empty-claims fake
// claude-cli script (fakeClaudeCLIClaimsScript): claim CONTENT is
// irrelevant here — SectionsCompiled counts sections the loop reached, not
// claims written, so a rejected/empty claim still proves the override let
// the run proceed.
func newBudgetStopFixture(t *testing.T) (storePath, root, service string) {
	t.Helper()
	root = t.TempDir()
	for i := 1; i <= 5; i++ {
		dir := filepath.Join(root, fmt.Sprintf("g%d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf("# Guide %d\n\n- Rule number %d applies here.\n", i, i)
		if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	storePath = filepath.Join(t.TempDir(), "budget.sqlite")
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

	packDir := filepath.Join(root, ".fathom", "layers", "obligations")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	layerCUE := `name:    "obligations"
version: "v1"
kind:    "obligation"
corpus: include: ["**/CLAUDE.md"]
prompt_version: "obligations/content-addressed"
prompt_file:    "schema/semantic/layers/obligations/prompt.md"
tier:           "doc-claimed"
role:           "governs"
stratum:        "concern"
evaled:         true
budgets: {
	max_tokens_per_run: 100
	max_usd_per_run:    1000
}
`
	if err := os.WriteFile(filepath.Join(packDir, "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatal(err)
	}
	// decodePack (manifest.go) resolves a project pack's prompt as a SIBLING
	// of layer.cue itself — the layer.cue's own prompt_file value above is
	// documentation only, never the resolution path.
	if err := os.WriteFile(filepath.Join(packDir, "prompt.md"), []byte("Emit {\"claims\": []}."), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", newFakeClaudeCLIClaims(t))
	return storePath, root, "svc"
}

// TestCmdCompile_MaxTokensPerRunOverride_UnblocksAPreflightRefusal is the
// CLI-level end-to-end proof for wave O4 item 2: the fixture's project pack
// sets max_tokens_per_run so small that a plain compile refuses PRE-FLIGHT
// (never reaches the LLM for any section — vs-pilot-5's second reading
// found exactly this class of defect: a fixed per-run budget too small for
// the corpus, silently limiting how much of `obligations` a bench subject's
// compile step ever covers); --max-tokens-per-run=all removes the cap and
// the SAME corpus compiles completely in the very next call, on the same
// store, with no other guard escape needed (a pre-flight REFUSAL — unlike a
// mid-run BudgetStopped — never counts as a "completed" prior run either
// way, so the idempotency guard was never going to be the obstacle here).
func TestCmdCompile_MaxTokensPerRunOverride_UnblocksAPreflightRefusal(t *testing.T) {
	storePath, _, svc := newBudgetStopFixture(t)

	// This project pack's own `evaled: true` gets forced to false (no
	// eval/RESULTS.md on disk — packs.go's D3-1 honesty override), so every
	// call here needs --unsafe-skip-eval-gate; that guard is orthogonal to
	// the budget guard this test is actually about.
	var stdout1, stderr1 bytes.Buffer
	code := cmdCompile([]string{
		"--service", svc, "--layer", "obligations", "--store", storePath, "--dry-run=false",
		"--unsafe-skip-eval-gate",
	}, &stdout1, &stderr1)
	if code != 1 {
		t.Fatalf("first (unbudgeted) compile: exit %d, want 1 (pre-flight refusal); stdout=%q stderr=%q", code, stdout1.String(), stderr1.String())
	}
	if !strings.Contains(stderr1.String(), "max_tokens_per_run") {
		t.Errorf("stderr should name the exceeded budget, got %q", stderr1.String())
	}
	if n := countCompileRuns(t, storePath, svc, "obligations"); n != 1 {
		t.Fatalf("a refused run is still recorded (ADR-016 §4.1); compile_runs = %d, want 1", n)
	}

	// With --max-tokens-per-run=all: the SAME corpus, same store, same
	// layer default budget everywhere else — but this run's own override
	// removes the cap, so the pre-flight estimate that just refused above
	// now passes, and every section actually compiles.
	var stdout2, stderr2 bytes.Buffer
	code = cmdCompile([]string{
		"--service", svc, "--layer", "obligations", "--store", storePath, "--dry-run=false",
		"--unsafe-skip-eval-gate", "--max-tokens-per-run=all",
	}, &stdout2, &stderr2)
	if code != 0 {
		t.Fatalf("second (--max-tokens-per-run=all) compile: exit %d; stdout=%q stderr=%q", code, stdout2.String(), stderr2.String())
	}
	if !strings.Contains(stdout2.String(), "sections_total:    5") {
		t.Errorf("stdout must report sections_total: 5, got %q", stdout2.String())
	}
	if !strings.Contains(stdout2.String(), "sections_compiled: 5") {
		t.Errorf("stdout must report sections_compiled: 5 (the override let every section reach the LLM), got %q", stdout2.String())
	}
	if strings.Contains(stdout2.String(), "budget_stopped:") {
		t.Errorf("--max-tokens-per-run=all must not budget-stop either, got stdout=%q", stdout2.String())
	}
	if strings.Contains(stdout2.String(), "refused:") {
		t.Errorf("--max-tokens-per-run=all must not refuse, got stdout=%q", stdout2.String())
	}
}

// TestCmdCompile_MaxUSDPerRunOverride_UnblocksAPreflightRefusal is
// --max-tokens-per-run's sibling proof for the dollar cap: the SAME
// fixture's project pack budget (max_usd_per_run left at its own default
// above, but overridden here to a value too small to pass pre-flight)
// refuses without an override and completes with --max-usd-per-run=all.
func TestCmdCompile_MaxUSDPerRunOverride_UnblocksAPreflightRefusal(t *testing.T) {
	storePath, root, svc := newBudgetStopFixture(t)
	// Loosen the token cap (already proven above) and instead tighten the
	// dollar cap far below what even a tiny pre-flight estimate prices at
	// the profile's own input rate — the two overrides are independent
	// flags/fields; this test isolates the USD one.
	layerCUE := `name:    "obligations"
version: "v1"
kind:    "obligation"
corpus: include: ["**/CLAUDE.md"]
prompt_version: "obligations/content-addressed"
prompt_file:    "schema/semantic/layers/obligations/prompt.md"
tier:           "doc-claimed"
role:           "governs"
stratum:        "concern"
evaled:         true
budgets: {
	max_tokens_per_run: 1000000
	max_usd_per_run:    0.00001
}
`
	if err := os.WriteFile(filepath.Join(root, ".fathom", "layers", "obligations", "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout1, stderr1 bytes.Buffer
	code := cmdCompile([]string{
		"--service", svc, "--layer", "obligations", "--store", storePath, "--dry-run=false",
		"--unsafe-skip-eval-gate",
	}, &stdout1, &stderr1)
	if code != 1 {
		t.Fatalf("first (unbudgeted) compile: exit %d, want 1 (pre-flight refusal); stdout=%q stderr=%q", code, stdout1.String(), stderr1.String())
	}
	if !strings.Contains(stderr1.String(), "max_usd_per_run") {
		t.Errorf("stderr should name the exceeded dollar budget, got %q", stderr1.String())
	}

	var stdout2, stderr2 bytes.Buffer
	code = cmdCompile([]string{
		"--service", svc, "--layer", "obligations", "--store", storePath, "--dry-run=false",
		"--unsafe-skip-eval-gate", "--max-usd-per-run=all",
	}, &stdout2, &stderr2)
	if code != 0 {
		t.Fatalf("second (--max-usd-per-run=all) compile: exit %d; stdout=%q stderr=%q", code, stdout2.String(), stderr2.String())
	}
	if !strings.Contains(stdout2.String(), "sections_compiled: 5") {
		t.Errorf("stdout must report sections_compiled: 5, got %q", stdout2.String())
	}
}

// TestParseBudgetOverride pins parseBudgetOverride's own contract directly
// (cmd/fathom/compile.go): "" for both means no override at all (nil, not
// a zero-value BudgetOverride — compileViaStore's "was an override even
// requested" branch depends on this); "all" (case-insensitive) maps to a
// pointer to 0/0.0 (the "unlimited" sentinel both enforceBudgets and
// semantic.BudgetOverride.Apply already use); a plain number parses
// straight through; a malformed value is a usage error naming the flag and
// the bad value.
func TestParseBudgetOverride(t *testing.T) {
	t.Run("both empty is nil, not a zero-value override", func(t *testing.T) {
		got, err := parseBudgetOverride("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("parseBudgetOverride(\"\", \"\") = %+v, want nil", got)
		}
	})

	t.Run("\"all\" maps to the unlimited sentinel for both fields", func(t *testing.T) {
		got, err := parseBudgetOverride("all", "ALL")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.MaxTokensPerRun == nil || *got.MaxTokensPerRun != 0 {
			t.Fatalf("MaxTokensPerRun override = %+v, want a pointer to 0", got)
		}
		if got.MaxUSDPerRun == nil || *got.MaxUSDPerRun != 0 {
			t.Fatalf("MaxUSDPerRun override (case-insensitive \"ALL\") = %+v, want a pointer to 0", got)
		}
	})

	t.Run("a plain number parses through", func(t *testing.T) {
		got, err := parseBudgetOverride("12345", "6.5")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.MaxTokensPerRun == nil || *got.MaxTokensPerRun != 12345 {
			t.Fatalf("MaxTokensPerRun override = %+v, want a pointer to 12345", got)
		}
		if got.MaxUSDPerRun == nil || *got.MaxUSDPerRun != 6.5 {
			t.Fatalf("MaxUSDPerRun override = %+v, want a pointer to 6.5", got)
		}
	})

	t.Run("only one flag set leaves the other nil", func(t *testing.T) {
		got, err := parseBudgetOverride("all", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.MaxTokensPerRun == nil {
			t.Fatal("MaxTokensPerRun override should be set")
		}
		if got.MaxUSDPerRun != nil {
			t.Errorf("MaxUSDPerRun override = %v, want nil (flag was not given)", *got.MaxUSDPerRun)
		}
	})

	t.Run("a malformed value is a usage error naming the flag", func(t *testing.T) {
		if _, err := parseBudgetOverride("not-a-number", ""); err == nil {
			t.Fatal("expected an error for a malformed --max-tokens-per-run value")
		} else if !strings.Contains(err.Error(), "max-tokens-per-run") {
			t.Errorf("error should name the flag, got %v", err)
		}
		if _, err := parseBudgetOverride("", "not-a-number"); err == nil {
			t.Fatal("expected an error for a malformed --max-usd-per-run value")
		} else if !strings.Contains(err.Error(), "max-usd-per-run") {
			t.Errorf("error should name the flag, got %v", err)
		}
	})
}
