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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/claudecli"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/store"
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

type evalLabels struct {
	Probes []string `json:"probes"`
	Rules  []struct {
		Key               string   `json:"key"`
		CiteSubstr        string   `json:"cite_substr"`
		Governs           []string `json:"governs"`
		TrapMustNotGovern []string `json:"trap_must_not_govern"`
	} `json:"rules"`
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
		return llm, model // claude-cli ignores an empty Model (uses the CLI's own default)
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

// evalResult is one layer's eval outcome, returned for the caller to gate
// and/or log to the pack's eval/RESULTS.md.
type evalResult struct {
	Precision      float64
	Recall         float64
	TP, FP, FN     int
	MissedRuleKeys []string
	Run            core.CompileRun
}

// runLayerEval compiles layerName's eval/*.md fixtures (every .md file in
// its pack's eval/ directory, sorted) through a real model and scores
// (claim, probe) applications against eval/labels.json.
func runLayerEval(t *testing.T, layerName string) evalResult {
	t.Helper()
	if os.Getenv("FATHOM_EVAL_LLM") != "1" {
		t.Skip("set FATHOM_EVAL_LLM=1 (and ANTHROPIC_API_KEY, or FATHOM_LLM_PROVIDER=claude-cli) to run the spending eval")
	}
	llm, model := evalLLM(t)

	dir := packEvalDir(t, layerName)
	var labels evalLabels
	lb, err := os.ReadFile(filepath.Join(dir, "labels.json"))
	if err != nil {
		t.Fatalf("read labels: %v", err)
	}
	if err := json.Unmarshal(lb, &labels); err != nil {
		t.Fatalf("parse labels: %v", err)
	}

	mdFiles, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil || len(mdFiles) == 0 {
		t.Fatalf("glob eval/*.md in %s: %v (found %d)", dir, err, len(mdFiles))
	}
	sort.Strings(mdFiles)

	var docs []core.SourceFile
	docContent := map[string][]byte{}
	for _, path := range mdFiles {
		name := filepath.Base(path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		docs = append(docs, core.SourceFile{Path: name, Content: data})
		docContent[name] = data
	}

	layer, err := LoadLayer(layerName)
	if err != nil {
		t.Fatalf("LoadLayer(%s): %v", layerName, err)
	}

	st := store.NewMemoryStore()
	c := &Compiler{
		St: st, LLM: llm, Model: model, Layer: layer,
		Now: func() time.Time { return time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC) },
	}
	run, err := c.Compile(context.Background(), CompileInput{
		ServiceID:    "eval",
		Docs:         docs,
		IndexedFiles: labels.Probes,
		Watermark:    "eval",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	t.Logf("compile run: %d claims emitted, %d rejected, %d in / %d out tokens",
		run.ClaimsEmitted, run.ClaimsRejected, run.TokensIn, run.TokensOut)

	claims, err := st.ClaimsForService(context.Background(), "eval", layerName)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	ids := make([]string, 0, len(claims))
	for _, cl := range claims {
		ids = append(ids, cl.ClaimID)
	}
	links, err := st.ClaimLinks(context.Background(), "eval", ids)
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	citedText := map[string]string{} // claimID -> cited lines
	for _, l := range links {
		if l.Rel != core.RelCites {
			continue
		}
		path, s, e, ok := core.ParseCiteRef(l.TargetRef)
		if !ok {
			continue
		}
		lines := strings.Split(strings.ReplaceAll(string(docContent[path]), "\r\n", "\n"), "\n")
		if e <= len(lines) {
			citedText[l.ClaimID] = strings.Join(lines[s-1:e], "\n")
		}
	}

	applies := func(cl core.SemanticClaim, probe string) bool {
		for _, g := range cl.Scope {
			if filter.New([]string{g}).Match(probe) {
				return true
			}
		}
		return false
	}

	var tp, fp, fn int
	matchedRule := map[string]bool{}
	for _, cl := range claims {
		// Match the claim to a labeled rule by citation content.
		ruleIdx := -1
		for i, r := range labels.Rules {
			if strings.Contains(citedText[cl.ClaimID], r.CiteSubstr) || strings.Contains(cl.Rule, r.CiteSubstr) {
				ruleIdx = i
				break
			}
		}
		if ruleIdx < 0 {
			// An invented/unlabeled rule: every application is a false positive.
			for _, probe := range labels.Probes {
				if applies(cl, probe) {
					fp++
					t.Logf("UNLABELED claim %q applies to %s (scope %v)", cl.Rule, probe, cl.Scope)
				}
			}
			continue
		}
		r := labels.Rules[ruleIdx]
		matchedRule[r.Key] = true
		governed := map[string]bool{}
		for _, g := range r.Governs {
			governed[g] = true
		}
		for _, probe := range labels.Probes {
			got := applies(cl, probe)
			want := governed[probe]
			switch {
			case got && want:
				tp++
			case got && !want:
				fp++
				t.Logf("FP [%s]: claim %q wrongly governs %s (scope %v)", r.Key, cl.Rule, probe, cl.Scope)
			case !got && want:
				fn++
				t.Logf("FN [%s]: claim %q misses %s (scope %v)", r.Key, cl.Rule, probe, cl.Scope)
			}
		}
	}

	var missed []string
	for _, r := range labels.Rules {
		if !matchedRule[r.Key] {
			missed = append(missed, r.Key)
		}
	}
	precision := 1.0
	if tp+fp > 0 {
		precision = float64(tp) / float64(tp+fp)
	}
	recall := 1.0
	if len(labels.Rules) > 0 {
		recall = float64(len(matchedRule)) / float64(len(labels.Rules))
	}
	t.Logf("precision=%.2f (tp=%d fp=%d fn=%d) rule-recall=%.2f missed=%v",
		precision, tp, fp, fn, recall, missed)

	return evalResult{Precision: precision, Recall: recall, TP: tp, FP: fp, FN: fn, MissedRuleKeys: missed, Run: run}
}

// assertGate applies the §4.6 admission bar to an evalResult.
func assertGate(t *testing.T, r evalResult) {
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
