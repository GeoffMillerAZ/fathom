package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// resolveTouchedPlanRows does the I/O review.RenderGitHubReview itself
// never performs (VB-22's "a verify line when a plan row's anchored symbol
// is touched"): scan docs/plans/*.md under corpusRoot, parse each via
// internal/verify.ParseMarkdownPlan (the same parser verify_plan/`fathom
// verify` use), and report every row whose {symbol[, package]} contract
// names a symbol defined in one of this diff's changed files. README.md is
// never a gated plan (docs/AGENTS.md) and is skipped. Best-effort: a glob
// or read failure degrades to an empty result, never an error — this is a
// review-comment enrichment, not a correctness-critical path. Deterministic:
// results are sorted by (plan, row id).
func resolveTouchedPlanRows(corpusRoot string, in review.Input) []review.TouchedPlanRow {
	if corpusRoot == "" {
		return nil
	}
	changed := make(map[string]bool, len(in.ChangedFiles))
	for _, f := range in.ChangedFiles {
		changed[f] = true
	}
	// Changed symbol name -> the file(s) it's defined in (a bare name may
	// repeat across files/packages; Expect.Package narrows exactly like
	// verify_plan's own FindSymbol+Package filter).
	filesByName := make(map[string][]string, len(in.Nodes))
	for _, n := range in.Nodes {
		if changed[n.FilePath] {
			filesByName[n.Name] = append(filesByName[n.Name], n.FilePath)
		}
	}
	if len(filesByName) == 0 {
		return nil
	}

	matches, err := filepath.Glob(filepath.Join(corpusRoot, "docs", "plans", "*.md"))
	if err != nil {
		return nil
	}
	sort.Strings(matches)

	var out []review.TouchedPlanRow
	for _, path := range matches {
		if filepath.Base(path) == "README.md" {
			continue
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}
		rel, rerr := filepath.Rel(corpusRoot, path)
		if rerr != nil {
			rel = filepath.Base(path)
		}
		rel = filepath.ToSlash(rel)

		plan := verify.ParseMarkdownPlan("", data)
		for _, t := range plan.Tasks {
			if t.Expect.Symbol == "" {
				continue
			}
			files, ok := filesByName[t.Expect.Symbol]
			if !ok {
				continue
			}
			if t.Expect.Package != "" && !anyHasPrefix(files, t.Expect.Package) {
				continue
			}
			out = append(out, review.TouchedPlanRow{Plan: rel, RowID: t.ID, Symbol: t.Expect.Symbol})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Plan != out[j].Plan {
			return out[i].Plan < out[j].Plan
		}
		return out[i].RowID < out[j].RowID
	})
	return out
}

func anyHasPrefix(paths []string, prefix string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}
