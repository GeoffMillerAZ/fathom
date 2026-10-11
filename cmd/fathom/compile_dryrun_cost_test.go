package main

// compile_dryrun_cost_test.go — refactor-conservation gap-closing wave 3,
// item 2 (docs/plans/refactor-conservation.md): `fathom compile --dry-run`
// must never print cost_usd "est. 0.00-0.00" for a model it simply never
// priced — before this fix, WithLLM (the only call site that sets
// Compiler.InRatePerKTok/OutRatePerKTok) ran only inside compileViaStore's
// `!dryRun` branch, so a dry run's cost band was ALWAYS zero regardless of
// --profile. Two ends: printCompileRunCore's own formatting (unit-level,
// mirrors compile_test.go's TestPrintCompileRunCore_* pattern) and
// compileViaStore's real wiring end to end (via cmdCompile --store).

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// isolateConfigDirs points HOME and the CWD at fresh, config-file-less
// temp directories for the duration of the test, so resolveActiveProfile
// (config.DefaultUserPath/DefaultProjectPath) never reads this machine's
// real ~/.fathom/config.toml or this repository's own project
// .fathom/config.toml — mirrors llmprofile_test.go's
// TestResolveActiveProfile_ReadsProjectConfig/UnknownFlagProfile_Errors
// isolation exactly, factored out since every test in this file needs it.
func isolateConfigDirs(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

// ─── printCompileRunCore: formatting only ─────────────────────────────────

func TestPrintCompileRunCore_DryRun_PriceKnown_ShowsRealRange(t *testing.T) {
	var buf bytes.Buffer
	printCompileRunCore(&buf, core.CompileRun{
		ServiceID: "svc", Layer: "obligations", DryRun: true,
		TokensIn: 1000, EstTokensOutLow: 2000, EstTokensOutHigh: 8000,
		EstCostLowUSD: 0.011, EstCostHighUSD: 0.041, PriceKnown: true,
		Model: "claude-haiku-4-5-20251001",
	})
	out := buf.String()
	if !strings.Contains(out, "cost_usd:          est. 0.01-0.04") {
		t.Errorf("output missing the real est. range; got:\n%s", out)
	}
	if strings.Contains(out, "cost unknown") {
		t.Errorf("a known-price dry run must not say cost unknown; got:\n%s", out)
	}
}

func TestPrintCompileRunCore_DryRun_PriceUnknown_DisclosesInsteadOfZero(t *testing.T) {
	var buf bytes.Buffer
	printCompileRunCore(&buf, core.CompileRun{
		ServiceID: "svc", Layer: "obligations", DryRun: true,
		TokensIn: 1000, EstTokensOutLow: 2000, EstTokensOutHigh: 8000,
		EstCostLowUSD: 0, EstCostHighUSD: 0, PriceKnown: false,
		Model: "some-custom-unpriced-model",
	})
	out := buf.String()
	if strings.Contains(out, "est. 0.00-0.00") {
		t.Errorf("output must never print a confident zero range for an unpriced model; got:\n%s", out)
	}
	want := "cost_usd:          unpriced (no price for model some-custom-unpriced-model)"
	if !strings.Contains(out, want) {
		t.Errorf("output missing %q; got:\n%s", want, out)
	}
}

func TestPrintCompileRunCore_DryRun_PriceUnknown_NoModelNamesUnknown(t *testing.T) {
	var buf bytes.Buffer
	printCompileRunCore(&buf, core.CompileRun{
		ServiceID: "svc", Layer: "obligations", DryRun: true,
		TokensIn: 1000, EstTokensOutLow: 2000, EstTokensOutHigh: 8000,
		PriceKnown: false, // Model deliberately empty
	})
	out := buf.String()
	want := "cost_usd:          unpriced (no price for model (unknown))"
	if !strings.Contains(out, want) {
		t.Errorf("output missing %q; got:\n%s", want, out)
	}
}

// ─── compileViaStore / cmdCompile: real end-to-end wiring ─────────────────

// TestCmdCompile_DryRun_KnownModelShowsRealCostRange reproduces the exact
// reported defect end to end: a dry run against the default (priced)
// compile model must show a real, nonzero cost estimate, not "est.
// 0.00-0.00" — even though no --profile was given, matching the bug
// report's "even with --profile api" shape (a resolved profile/model that
// IS priced still showed zero before this fix, since WithLLM was never
// called on the dry-run path at all).
func TestCmdCompile_DryRun_KnownModelShowsRealCostRange(t *testing.T) {
	isolateConfigDirs(t)
	clearProfileEnv(t)
	storePath, _ := newGuardTestStore(t)

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "est. 0.00-0.00") {
		t.Errorf("dry run must not print a confident zero cost range for the known-priced default model; got:\n%s", out)
	}
	if strings.Contains(out, "cost unknown") {
		t.Errorf("the default compile model has a known price; must not report cost unknown:\n%s", out)
	}
	if !strings.Contains(out, "cost_usd:          est. ") {
		t.Errorf("expected a real cost_usd est. line, got:\n%s", out)
	}
	if !strings.Contains(out, "model:             "+defaultCompileModel) {
		t.Errorf("expected the dry run to record which model it priced, got:\n%s", out)
	}
}

// TestCmdCompile_DryRun_UnknownModelDisclosesInsteadOfZero pins the other
// half: a model this binary has never priced (here, FATHOM_COMPILE_MODEL
// pointing at an unrecognized string, the same env override
// compileModelForProfile already honors over any profile) must disclose
// "cost unknown" naming the model, never a fabricated or zero-shaped range.
func TestCmdCompile_DryRun_UnknownModelDisclosesInsteadOfZero(t *testing.T) {
	isolateConfigDirs(t)
	clearProfileEnv(t)
	storePath, _ := newGuardTestStore(t)
	t.Setenv("FATHOM_COMPILE_MODEL", "some-custom-unpriced-model")

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "est. 0.00-0.00") || strings.Contains(out, "0.00-0.00") {
		t.Errorf("must never print a zero-shaped range for an unpriced model; got:\n%s", out)
	}
	want := "cost_usd:          unpriced (no price for model some-custom-unpriced-model)"
	if !strings.Contains(out, want) {
		t.Errorf("output missing %q; got:\n%s", want, out)
	}
}

// TestCmdCompile_DryRun_UnresolvableProfileFailsOpen: a --profile naming a
// profile that does not exist must not crash or block the dry-run estimate
// (profile resolution during a dry run is best-effort, never a refusal —
// only a spending compile refuses on a bad --profile). The estimate falls
// back to compileModel()'s own env/default resolution.
func TestCmdCompile_DryRun_UnresolvableProfileFailsOpen(t *testing.T) {
	isolateConfigDirs(t)
	clearProfileEnv(t)
	storePath, _ := newGuardTestStore(t)

	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{
		"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run",
		"--profile", "nonexistent-profile",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("a dry run must fail OPEN on an unresolvable --profile, not refuse; exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "model:             "+defaultCompileModel) {
		t.Errorf("expected the fallback default model to be named, got:\n%s", out)
	}
}
