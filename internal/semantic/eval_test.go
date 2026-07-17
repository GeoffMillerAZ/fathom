package semantic

// eval_test.go — the obligations layer's precision gate (ADR-016 §4.6).
//
// The deterministic suite (compile_test.go et al.) proves the machinery; this
// eval proves the *scope mapping quality of a real model* against a labeled
// corpus with deliberate scope traps. It spends real tokens, so it only runs
// when explicitly asked:
//
//	FATHOM_EVAL_LLM=1 ANTHROPIC_API_KEY=... go test ./internal/semantic -run TestObligationsEval -v
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
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type evalLabels struct {
	Probes []string `json:"probes"`
	Rules  []struct {
		Key               string   `json:"key"`
		CiteSubstr        string   `json:"cite_substr"`
		Governs           []string `json:"governs"`
		TrapMustNotGovern []string `json:"trap_must_not_govern"`
	} `json:"rules"`
}

// evalDir resolves the obligations pack's eval fixtures. LC-4 (docs/plans/
// layer-catalog.md) moved these from internal/semantic/testdata/eval/ to
// schema/semantic/layers/obligations/eval/ — the pack-shape location
// (docs/vision/02-shape.md §3) — since eval fixtures are dev/gate-time
// artifacts read straight from the source tree, never go:embed'd into the
// binary.
func evalDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "schema", "semantic", "layers", "obligations", "eval")
}

func TestObligationsEval_Precision(t *testing.T) {
	if os.Getenv("FATHOM_EVAL_LLM") != "1" {
		t.Skip("set FATHOM_EVAL_LLM=1 (and ANTHROPIC_API_KEY) to run the spending eval")
	}
	llm, err := anthropic.New(anthropic.Config{})
	if err != nil {
		t.Skipf("anthropic client unavailable: %v", err)
	}
	model := os.Getenv("FATHOM_EVAL_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	dir := evalDir(t)
	var labels evalLabels
	lb, err := os.ReadFile(filepath.Join(dir, "labels.json"))
	if err != nil {
		t.Fatalf("read labels: %v", err)
	}
	if err := json.Unmarshal(lb, &labels); err != nil {
		t.Fatalf("parse labels: %v", err)
	}

	var docs []core.SourceFile
	docContent := map[string][]byte{}
	for _, name := range []string{"standards.md", "security.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		docs = append(docs, core.SourceFile{Path: name, Content: data})
		docContent[name] = data
	}

	st := store.NewMemoryStore()
	c := &Compiler{
		St: st, LLM: llm, Model: model,
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

	claims, err := st.ClaimsForService(context.Background(), "eval", LayerObligations)
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
	recall := float64(len(matchedRule)) / float64(len(labels.Rules))
	t.Logf("precision=%.2f (tp=%d fp=%d fn=%d) rule-recall=%.2f missed=%v",
		precision, tp, fp, fn, recall, missed)

	if precision < 0.90 {
		t.Errorf("precision %.2f < 0.90 — the layer does NOT pass its shipping gate (ADR-016 §4.6)", precision)
	}
	if recall < 0.75 {
		t.Errorf("rule recall %.2f < 0.75 — the layer does NOT pass its shipping gate", recall)
	}
	_ = fmt.Sprintf
}
