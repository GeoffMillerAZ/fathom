package semantic

// eval.go — the precision/recall gate machinery (docs/design/semantic-
// layers.md §4.6, docs/plans/layer-catalog.md), extracted from
// eval_test.go's runLayerEval into an exported, non-test function so a
// runtime caller (cmd/fathom's `module eval` verb, docs/vision/13-expert-
// suite.md §4/§9 row J) can run the SAME gate a layer pack's own eval_test.go
// runs, instead of forking it. eval_test.go's runLayerEval is now a thin
// wrapper over RunEval — the deterministic scoring logic lives in exactly
// one place.
//
// Gate (release-blocking for the layer, not for the build): precision >= 0.90
// on (claim, probe) application pairs; recall >= 0.75 on labeled rules.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// EvalLabels is the eval/labels.json shape: the explicit fixture-doc list
// (never a glob — an eval/ dir also holds RESULTS.md and other non-fixture
// notes, and a glob would silently sweep those into the corpus), the probe
// file list, and the labeled rules each fixture doc is expected to compile
// into.
type EvalLabels struct {
	Docs   []string `json:"docs"`
	Probes []string `json:"probes"`
	Rules  []struct {
		Key               string   `json:"key"`
		CiteSubstr        string   `json:"cite_substr"`
		Governs           []string `json:"governs"`
		TrapMustNotGovern []string `json:"trap_must_not_govern"`
	} `json:"rules"`
}

// EvalResult is one layer's eval outcome — the run's precision/recall plus
// enough detail (TP/FP/FN counts, missed rule keys, the underlying
// CompileRun) for a caller to gate on it, log it to eval/RESULTS.md, or
// render a doctor verdict line.
type EvalResult struct {
	Precision      float64
	Recall         float64
	TP, FP, FN     int
	MissedRuleKeys []string
	Run            core.CompileRun
}

// PassesGate applies §4.6's admission bar: precision >= 0.90, recall >=
// 0.75.
func (r EvalResult) PassesGate() bool {
	return r.Precision >= 0.90 && r.Recall >= 0.75
}

// LoadEvalLabels reads and parses dir/labels.json.
func LoadEvalLabels(dir string) (EvalLabels, error) {
	var labels EvalLabels
	raw, err := os.ReadFile(filepath.Join(dir, "labels.json"))
	if err != nil {
		return EvalLabels{}, fmt.Errorf("semantic: read %s/labels.json: %w", dir, err)
	}
	if err := json.Unmarshal(raw, &labels); err != nil {
		return EvalLabels{}, fmt.Errorf("semantic: parse %s/labels.json: %w", dir, err)
	}
	if len(labels.Docs) == 0 {
		return EvalLabels{}, fmt.Errorf("semantic: %s/labels.json must list its fixture docs explicitly (\"docs\": [...]) — a glob would sweep RESULTS.md and other non-fixture files into the corpus", dir)
	}
	return labels, nil
}

// loadEvalDocs reads each of labels.Docs from dir, returning both the
// core.SourceFile list (compile input order = labels.Docs order) and a
// path->content map for later citation-text lookup.
func loadEvalDocs(dir string, labels EvalLabels) ([]core.SourceFile, map[string][]byte, error) {
	var docs []core.SourceFile
	content := map[string][]byte{}
	for _, name := range labels.Docs {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, nil, fmt.Errorf("semantic: read fixture doc %s: %w", name, err)
		}
		docs = append(docs, core.SourceFile{Path: name, Content: data})
		content[name] = data
	}
	return docs, content, nil
}

// RunEval compiles layer's eval/*.md fixtures (read from dir, the pack's
// eval/ directory) through llm and scores (claim, probe) applications
// against dir/labels.json's ground truth. now defaults to time.Now when nil
// (repo convention: never call time.Now() directly in logic a caller may
// want to pin — the eval score has traditionally been pinned in tests via a
// fixed date; callers that don't care may pass nil).
//
// This is the exact scoring logic eval_test.go's runLayerEval used to
// implement inline — extracted so a real (non-test) caller, e.g. `fathom
// module eval`, can run the same gate against a mock LLM (deterministic,
// no network, no key) for a scripted cold run, or a real client for a
// genuine layer-shipping decision.
func RunEval(ctx context.Context, llm llmclient.LLMClient, model string, layer LayerDef, dir string, now func() time.Time) (EvalResult, error) {
	labels, err := LoadEvalLabels(dir)
	if err != nil {
		return EvalResult{}, err
	}
	docs, docContent, err := loadEvalDocs(dir, labels)
	if err != nil {
		return EvalResult{}, err
	}
	if now == nil {
		now = time.Now
	}

	st := store.NewMemoryStore()
	c := &Compiler{St: st, LLM: llm, Model: model, Layer: layer, Now: now}
	run, err := c.Compile(ctx, CompileInput{
		ServiceID:    "eval",
		Docs:         docs,
		IndexedFiles: labels.Probes,
		Watermark:    "eval",
	})
	if err != nil {
		return EvalResult{}, fmt.Errorf("semantic: eval compile: %w", err)
	}

	claims, err := st.ClaimsForService(ctx, "eval", layer.Name)
	if err != nil {
		return EvalResult{}, fmt.Errorf("semantic: eval load claims: %w", err)
	}
	ids := make([]string, 0, len(claims))
	for _, cl := range claims {
		ids = append(ids, cl.ClaimID)
	}
	links, err := st.ClaimLinks(ctx, "eval", ids)
	if err != nil {
		return EvalResult{}, fmt.Errorf("semantic: eval load claim links: %w", err)
	}
	citedText := map[string]string{}
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
		ruleIdx := -1
		for i, r := range labels.Rules {
			if strings.Contains(citedText[cl.ClaimID], r.CiteSubstr) || strings.Contains(cl.Rule, r.CiteSubstr) {
				ruleIdx = i
				break
			}
		}
		if ruleIdx < 0 {
			for _, probe := range labels.Probes {
				if applies(cl, probe) {
					fp++
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
			case !got && want:
				fn++
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

	return EvalResult{Precision: precision, Recall: recall, TP: tp, FP: fp, FN: fn, MissedRuleKeys: missed, Run: run}, nil
}
