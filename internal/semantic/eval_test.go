package semantic

// eval_test.go — the precision/recall gate every layer must clear before it
// ships embedded (docs/design/semantic-layers.md §4.6, docs/plans/
// layer-catalog.md). runLayerEval is the shared harness: load the layer,
// compile its eval/*.md fixtures through a REAL model, and score
// (claim, probe) applications against eval/labels.json's ground truth. Each
// layer gets a thin *_test.go wrapper that calls it and asserts the gate —
// the deterministic suite (compile_test.go et al.) proves the machinery;
// this proves the *scope mapping quality of a real model*. It spends real
// tokens, so it only runs when explicitly asked:
//
//	FATHOM_EVAL_LLM=1 ANTHROPIC_API_KEY=... go test ./internal/semantic -run TestObligationsEval -v
//	FATHOM_EVAL_LLM=1 FATHOM_LLM_PROVIDER=claude-cli go test ./internal/semantic -run TestADRRulesEval -v
//
// Gate (release-blocking for the layer, not for the build): precision >= 0.90
// on (claim, probe) application pairs; recall >= 0.75 on labeled rules.
// Every-edit exposure makes false positives trust-fatal — precision leads.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/claudecli"
)

// repoRootForTest resolves the repo root from this test file's own location
// (internal/semantic/eval_test.go is two directories below root) — the same
// runtime.Caller trick manifest_test.go/schema_test.go already use.
func repoRootForTest(t *testing.T) (string, error) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", ".."), nil
}

// evalLLM constructs the eval's LLM client per FATHOM_LLM_PROVIDER — unset
// or "anthropic" (the metered API key lane) or "claude-cli" (the
// subscription lane via a logged-in Claude Code CLI, no API key needed;
// FATHOM_CLAUDE_BIN optionally overrides the resolved binary, mirroring
// cmd/fathom/compile.go's buildClaudeCLIClient). Any other value is a typed
// configuration error, matching buildLLMClient's convention.
func evalLLM(t *testing.T) (llmclient.LLMClient, string) {
	t.Helper()
	model := os.Getenv("FATHOM_EVAL_MODEL")
	switch provider := os.Getenv("FATHOM_LLM_PROVIDER"); provider {
	case "", "anthropic":
		llm, err := anthropic.New(anthropic.Config{})
		if err != nil {
			t.Skipf("anthropic client unavailable: %v", err)
		}
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		return llm, model
	case "claude-cli":
		llm, err := claudecli.New(claudecli.Config{Bin: os.Getenv("FATHOM_CLAUDE_BIN")})
		if err != nil {
			t.Skipf("claude-cli unavailable: %v", err)
		}
		if model == "" {
			// Pin the PRODUCTION compile default (compileModel()'s default,
			// cmd/fathom/compile.go) instead of floating on the CLI's own
			// default model: an eval result recorded under an unpinned model
			// doesn't transfer to the fleet-compile lane, which is the whole
			// point of gating (critic finding 4c on the LC slice).
			model = "claude-haiku-4-5-20251001"
		}
		return llm, model
	default:
		t.Fatalf("unknown FATHOM_LLM_PROVIDER %q (want \"\", \"anthropic\", or \"claude-cli\")", provider)
		return nil, ""
	}
}

// packEvalDir resolves <pack>'s pack-shape eval fixtures
// (schema/semantic/layers/<pack>/eval/ — dev/gate-time artifacts read
// straight from the source tree, never go:embed'd into the binary).
func packEvalDir(t *testing.T, pack string) string {
	t.Helper()
	root, err := repoRootForTest(t)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "schema", "semantic", "layers", pack, "eval")
}

// runLayerEval compiles layerName's eval fixtures (the labels.json "docs"
// list — explicit, never a glob, so RESULTS.md and other non-fixture files
// in eval/ are never swept into the corpus) through a real model and scores
// (claim, probe) applications against eval/labels.json. The scoring
// machinery itself lives in RunEval (eval.go) — this wrapper only resolves
// the pack's fixtures dir/layer, gates on FATHOM_EVAL_LLM, and logs a
// human-readable summary the exported RunEval (a non-test function with no
// *testing.T to log through) cannot.
func runLayerEval(t *testing.T, layerName string) EvalResult {
	t.Helper()
	if os.Getenv("FATHOM_EVAL_LLM") != "1" {
		t.Skip("set FATHOM_EVAL_LLM=1 (and ANTHROPIC_API_KEY, or FATHOM_LLM_PROVIDER=claude-cli) to run the spending eval")
	}
	llm, model := evalLLM(t)
	lane := os.Getenv("FATHOM_LLM_PROVIDER")
	if lane == "" {
		lane = "anthropic"
	}
	t.Logf("eval lane=%q model=%q", lane, model)

	layer, err := LoadLayer(layerName)
	if err != nil {
		t.Fatalf("LoadLayer(%s): %v", layerName, err)
	}

	dir := packEvalDir(t, layerName)
	fixedNow := func() time.Time { return time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC) }
	result, err := RunEval(context.Background(), llm, model, layer, dir, fixedNow)
	if err != nil {
		t.Fatalf("RunEval: %v", err)
	}
	t.Logf("compile run: %d claims emitted, %d rejected, %d in / %d out tokens",
		result.Run.ClaimsEmitted, result.Run.ClaimsRejected, result.Run.TokensIn, result.Run.TokensOut)
	t.Logf("precision=%.2f (tp=%d fp=%d fn=%d) rule-recall=%.2f missed=%v",
		result.Precision, result.TP, result.FP, result.FN, result.Recall, result.MissedRuleKeys)

	return result
}

// assertGate applies the §4.6 admission bar to an EvalResult.
func assertGate(t *testing.T, r EvalResult) {
	t.Helper()
	if r.Precision < 0.90 {
		t.Errorf("precision %.2f < 0.90 — the layer does NOT pass its shipping gate (docs/design/semantic-layers.md §4.6)", r.Precision)
	}
	if r.Recall < 0.75 {
		t.Errorf("rule recall %.2f < 0.75 — the layer does NOT pass its shipping gate", r.Recall)
	}
}

func TestObligationsEval_Precision(t *testing.T) {
	assertGate(t, runLayerEval(t, LayerObligations))
}

// TestADRRulesEval_Precision gates the LC-5 layer-2 proof pack
// (docs/plans/layer-catalog.md): if this fails after honest prompt
// iteration, adr-rules ships with a KILL.md recording the failing metrics
// and stays OUT of the embedded set — see
// schema/semantic/layers/adr-rules/KILL.md.
func TestADRRulesEval_Precision(t *testing.T) {
	assertGate(t, runLayerEval(t, "adr-rules"))
}

// TestSpecBindingsEval_Precision gates the LC-6 SDD pack (docs/plans/
// layer-catalog.md): same admission bar as adr-rules — see
// schema/semantic/layers/spec-bindings/KILL.md if it fails.
func TestSpecBindingsEval_Precision(t *testing.T) {
	assertGate(t, runLayerEval(t, "spec-bindings"))
}
