package main

// intentadrrules.go — `fathom intent adr-rules` (CX-19, design
// docs/design/change-and-compliance.md §5 step 1, F24 promoted): the
// dependency statements in the repository's decision records, each proposed
// as an archconformance deny rule with its ADR, file:line and sentence.
// Fathom never writes the rule pack: a human accepts a proposal by copying
// its CUE into .fathom/rules/archconformance.cue (ADR-016's compile
// discipline). A proposal already in the pack is marked so.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/review"
)

// adrRuleRow is one proposal and whether the rule pack already holds it.
type adrRuleRow struct {
	review.RuleProposal
	InRulePack bool `json:"in_rule_pack"`
}

func cmdIntentADRRules(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent adr-rules", stderr)
	root := fs.String("root", ".", "repository root")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	c, err := intent.Load(abs)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading the intent corpus: %v\n", err)
		return 1
	}
	if c.DecisionsHome == "" {
		fmt.Fprintln(stderr, "error: no decision records home resolved (the router names none and no conventional home exists)")
		return 1
	}
	files, err := decisionRecordFiles(abs, c.DecisionsHome)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	var props []review.RuleProposal
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(abs, rel))
		if err != nil {
			fmt.Fprintf(stderr, "error: reading %s: %v\n", rel, err)
			return 1
		}
		props = append(props, review.ProposeADRRules(rel, data)...)
	}
	pack, err := projectrules.Load(abs)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	inPack := map[string]bool{}
	for _, r := range pack.Rules {
		if !r.Allow {
			inPack[archconformance.RuleKey(r)] = true
		}
	}
	rows := make([]adrRuleRow, 0, len(props))
	for _, p := range props {
		rows = append(rows, adrRuleRow{RuleProposal: p, InRulePack: inPack[archconformance.RuleKey(archconformance.Rule{FromPkg: p.FromPkg, ToPkg: p.ToPkg})]})
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{"root": abs, "decisions": files, "rule_pack": projectrules.RelPath, "proposals": rows}); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%d dependency statement(s) in %s proposed as archconformance rules — fathom never writes %s; accept one by adding it there\n", len(rows), strings.Join(files, ", "), projectrules.RelPath)
	for _, r := range rows {
		fmt.Fprintln(stdout)
		if r.InRulePack {
			fmt.Fprintf(stdout, "// already in %s\n", projectrules.RelPath)
		}
		fmt.Fprint(stdout, review.RenderRuleProposal(r.RuleProposal))
	}
	return 0
}

// decisionRecordFiles lists the repo-relative Markdown files under home (a
// file, or a directory of records), sorted.
func decisionRecordFiles(root, home string) ([]string, error) {
	p := filepath.Join(root, home)
	st, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("decision records %s: %w", home, err)
	}
	if !st.IsDir() {
		return []string{filepath.ToSlash(home)}, nil
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		return nil, fmt.Errorf("decision records %s: %w", home, err)
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, filepath.ToSlash(filepath.Join(home, e.Name())))
		}
	}
	sort.Strings(out)
	return out, nil
}
