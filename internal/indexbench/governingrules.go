// governingrules.go — VB-11 (docs/plans/verbs.md, ADR-035 decision 2): the
// free oracle bench-pyramid VB-12 needs before any "obligations ranks
// correctly" claim is allowed to ship.
//
// testdata/indexbench/governing-rules.json carries forty hand-derived
// labels — {path, governing_rule: {source, line_hint, text}, notes} — one
// per file across the fixture fleet and this repository's own tree, each
// written by READING the file's actual governing doc (internal/CLAUDE.md,
// cmd/fathom/CLAUDE.md, docs/CLAUDE.md, schema/CLAUDE.md, the root
// CLAUDE.md, ADR.md, docs/intent/) — never by running the obligations
// engine and copying its answer. That independence is the whole point of a
// FREE oracle (bench-pyramid §3.1's L0 discipline, applied to governance
// instead of go/types): the label is a fact about the repository, not a
// prediction about the tool.
//
// GoverningRuleOracle asks semantic.ObligationsForLayer (layer:"rules" —
// the human-authored-governance question VB-9 made the default) the same
// question an agent would for each labelled path, and scores whether the
// label's governing DOCUMENT is among the top 1 / top 3 served rules'
// citations. GrepNearestClaudeMD scores the "grep the nearest CLAUDE.md up
// the tree" baseline over the SAME forty labels with no ranker involved at
// all — a deterministic directory walk, no store, no daemon — because R2's
// finding (docs/vision/29-verbs-not-lookups.md) is that grep is the
// competitor obligations has to beat, not a strawman.
package indexbench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// GoverningRule is one hand-derived citation: the document a file's
// governing rule lives in, a human-readable anchor into it, and the rule
// text itself (kept for the record; the oracle scores on Source alone —
// text matching a served rule's own restated wording would be a stricter
// and much noisier question than "did the right DOCUMENT get served").
type GoverningRule struct {
	Source   string `json:"source"`
	LineHint string `json:"line_hint"`
	Text     string `json:"text"`
}

// GoverningRuleLabel is one entry in testdata/indexbench/governing-rules.json.
type GoverningRuleLabel struct {
	Path          string        `json:"path"`
	GoverningRule GoverningRule `json:"governing_rule"`
	Notes         string        `json:"notes"`
}

// GoverningRulesFixture is the whole committed hand-labelled file.
type GoverningRulesFixture struct {
	Oracle  string               `json:"oracle"`
	Entries []GoverningRuleLabel `json:"entries"`
}

// LoadGoverningRules reads and decodes the committed hand-labelled oracle
// file. Deterministic: the JSON array's own order is preserved untouched,
// so the same file produces the same slice on every call.
func LoadGoverningRules(path string) (GoverningRulesFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GoverningRulesFixture{}, fmt.Errorf("indexbench: load governing-rules fixture: %w", err)
	}
	var fx GoverningRulesFixture
	if err := json.Unmarshal(data, &fx); err != nil {
		return GoverningRulesFixture{}, fmt.Errorf("indexbench: decode governing-rules fixture: %w", err)
	}
	return fx, nil
}

// GoverningRuleReport is one oracle run's numbers — the shape committed as
// testdata/indexbench/governing-rules-baseline.json,
// governing-rules-after.json, and (for the grep comparator)
// governing-rules-grep.json.
type GoverningRuleReport struct {
	Oracle   string  `json:"oracle"`
	Labels   int     `json:"labels"`
	Top1Hits int     `json:"top1_hits"`
	Top3Hits int     `json:"top3_hits"`
	Top1Rate float64 `json:"top1_rate"`
	Top3Rate float64 `json:"top3_rate"`
	// Misses lists every labelled path that missed top-3, sorted, so a
	// rate is never read without the material to triage it.
	Misses []string `json:"misses,omitempty"`
}

// citationSource returns the document-path half of a "path#Lstart-Lend"
// citation (core.CiteRef's shape) — "" for an empty citation.
func citationSource(cite string) string {
	if i := strings.IndexByte(cite, '#'); i >= 0 {
		return cite[:i]
	}
	return cite
}

// GoverningRuleOracle scores the obligations engine's CURRENT ranker
// against every label in fx: for each labelled path, the top-3 rules
// semantic.ObligationsForLayer(layer:"rules") serves for it, checking
// whether any served rule's citation names the SAME governing document as
// the label. top1 counts only when the very first served rule matches;
// top3 counts a match anywhere in the first three.
//
// Deterministic given a deterministic store: no clock, no sampling, one
// query per label in the fixture's own order, and ObligationsForLayer
// itself is a pure function of (store contents, path) — see
// internal/semantic's determinism contract.
func GoverningRuleOracle(ctx context.Context, st store.Store, serviceID string, fx GoverningRulesFixture) (GoverningRuleReport, error) {
	rep := GoverningRuleReport{Oracle: "governing-rules", Labels: len(fx.Entries)}
	for _, e := range fx.Entries {
		obl, err := semantic.ObligationsForLayer(ctx, st, serviceID, e.Path, 3, semantic.ObligationLayerRules)
		if err != nil {
			return GoverningRuleReport{}, fmt.Errorf("indexbench: obligations for %s: %w", e.Path, err)
		}
		top1 := len(obl.Rules) > 0 && citationSource(obl.Rules[0].Src) == e.GoverningRule.Source
		top3 := false
		for i, r := range obl.Rules {
			if i >= 3 {
				break
			}
			if citationSource(r.Src) == e.GoverningRule.Source {
				top3 = true
				break
			}
		}
		if top1 {
			rep.Top1Hits++
		}
		if top3 {
			rep.Top3Hits++
		} else {
			rep.Misses = append(rep.Misses, e.Path)
		}
	}
	sort.Strings(rep.Misses)
	rep.Top1Rate = rate(rep.Top1Hits, rep.Labels)
	rep.Top3Rate = rate(rep.Top3Hits, rep.Labels)
	return rep, nil
}

// GrepNearestClaudeMD scores the "grep the nearest CLAUDE.md/AGENTS.md up
// the tree" baseline (VB-11): for each label, the nearest instruction file
// at or above the labelled path's own directory, scored as a hit when that
// file IS the label's governing source. No ranker, no store, no daemon —
// a deterministic filesystem walk under root; grep has no notion of "top
// 3", so top1 and top3 are identical here by construction.
func GrepNearestClaudeMD(root string, fx GoverningRulesFixture) GoverningRuleReport {
	rep := GoverningRuleReport{Oracle: "grep-nearest-claude-md", Labels: len(fx.Entries)}
	for _, e := range fx.Entries {
		nearest := nearestInstructionFile(root, e.Path)
		if nearest != "" && nearest == e.GoverningRule.Source {
			rep.Top1Hits++
			rep.Top3Hits++
		} else {
			rep.Misses = append(rep.Misses, e.Path)
		}
	}
	sort.Strings(rep.Misses)
	rep.Top1Rate = rate(rep.Top1Hits, rep.Labels)
	rep.Top3Rate = rate(rep.Top3Hits, rep.Labels)
	return rep
}

// instructionFileNames mirrors internal/semantic's instructionFileBases —
// duplicated rather than imported so this grep-shaped baseline never
// depends on the engine it is comparing against.
var instructionFileNames = []string{"CLAUDE.md", "AGENTS.md"}

// nearestInstructionFile returns the repo-relative path of the nearest
// CLAUDE.md/AGENTS.md at or above file's own directory, or "" when none
// exists up to root. Deterministic: pure string/directory walk, one disk
// stat per candidate.
func nearestInstructionFile(root, file string) string {
	dir := filepath.ToSlash(filepath.Dir(filepath.Clean(file)))
	for {
		for _, base := range instructionFileNames {
			candidate := base
			if dir != "." {
				candidate = dir + "/" + base
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err == nil {
				return candidate
			}
		}
		if dir == "." {
			return ""
		}
		dir = filepath.ToSlash(filepath.Dir(dir))
	}
}
