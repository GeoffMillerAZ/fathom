package main

// conserve.go — cmdConserve (CV-19, ADR-044 §0/§6): the CLI surface for
// internal/conserve's report —
//
//	fathom conserve --since <ref> [--json] [--strict] [--service S] [--store DB]
//
// Store-direct (mirrors cmd/fathom/review.go and cmd/fathom/verify.go):
// opens --store directly, resolves the repo root the same way `fathom
// verify`/`fathom review` do (verifyRepoRoot), and calls
// conserve.NewInput + conserve.Run over conserve.Planes() — every
// registered plane, including ones registered by another file in this
// wave (surface, certificates, governance here; integrity/absence
// elsewhere) — so this command never needs editing when a new plane
// lands.
//
// Exit codes: 0 pass (including --strict with no losses); 1 (--strict
// only) any plane's Verdict is "losses"; 2 an operational failure
// (bad flags, unreadable store, an unresolvable --since ref).
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func cmdConserve(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("conserve", stderr)
	since := fs.String("since", "", "git ref to compare HEAD against (required)")
	service := fs.String("service", "", "service ID (required)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	jsonOut := fs.Bool("json", false, "emit the fathom.conserve/v1 JSON report instead of a text table")
	strict := fs.Bool("strict", false, "exit 1 when any plane's verdict is \"losses\"")
	residueMax := fs.Int("residue-max", conserve.DefaultResidueMax, "cap on the reading set's item list (design §3.4); the report says how many were cut")
	coverageMeta := fs.String("coverage-meta", "", "path to a coverage.meta.json binding a Go coverage profile for the reading set's execution evidence (design §3.5); default: [tests] results_dir/coverage.meta.json when configured")
	var plans planFlags
	fs.Var(&plans, "plan", "plan file to read retires:/surface_change: declarations from (repeatable); default: every docs/plans/*.md at the repo root (non-recursive, the same set the gate itself walks)")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *since == "" {
		fmt.Fprintln(stderr, "error: conserve requires --since <ref>")
		fs.Usage()
		return 2
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: conserve requires --service")
		fs.Usage()
		return 2
	}

	root := verifyRepoRoot()
	planPaths := []string(plans)
	if len(planPaths) == 0 {
		planPaths = discoverPlanPaths(root)
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 2
	}
	defer st.Close()

	ctx := context.Background()
	rep, err := conserveReportFor(ctx, st, root, *since, *service, planPaths, *coverageMeta, *residueMax)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	residue := *rep.Residue

	if *jsonOut {
		b, jerr := json.MarshalIndent(rep, "", "  ")
		if jerr != nil {
			fmt.Fprintf(stderr, "error: encode JSON: %v\n", jerr)
			return 2
		}
		fmt.Fprintln(stdout, string(b))
	} else {
		printConserveReport(stdout, rep)
		printConserveResidue(stdout, residue)
	}

	if *strict {
		for _, pr := range rep.Planes {
			if pr.Verdict == conserve.Losses {
				return 1
			}
		}
	}
	return 0
}

// conserveReportFor is `fathom conserve --since --json`'s whole report:
// every registered plane over base..worktree, the per-language tiers and
// the reading set. The console's refactor page (OC-26) calls it with the
// service's registered root, so the page and the verb cannot disagree.
func conserveReportFor(ctx context.Context, st store.Store, root, since, service string, planPaths []string, coverageMeta string, residueMax int) (conserve.Report, error) {
	in, err := conserve.NewInput(ctx, root, since, st, service, planPaths)
	if err != nil {
		return conserve.Report{}, err
	}
	loadConserveCoverage(&in, root, coverageMeta)
	rep := conserve.Run(ctx, in, conserve.Planes())
	rep.Tiers = conserve.TiersByLanguage(ctx, in)
	residue := conserve.Residue(ctx, in, residueMax)
	rep.Residue = &residue
	return rep, nil
}

// loadConserveCoverage resolves and loads the coverage.meta.json (CV-67,
// design §3.5) execution evidence draws from: coverageMetaFlag when set
// (`--coverage-meta`), else `[tests] results_dir/coverage.meta.json` when
// root's own project (or the user) .fathom/config.toml names one. A thin
// wrapper over conserve.LoadConfiguredCoverage (internal/conserve/
// coverageload.go) — the one implementation `fathom review --since` and
// MCP review_diff now share too, so every surface answers the reading
// set's execution evidence identically for the same range (CV-65's own
// promise, design §3.4). Neither being configured, or the default
// results_dir guess simply not existing there, is NOT itself a
// reportable failure — most runs have no profile at all. An actual
// read/parse failure, an explicit `--coverage-meta` naming a path that
// does not exist, or a profile that loads but maps no block to any
// repository file at all, is recorded on in.CoverageLoadError, surfaced
// verbatim as ResidueReport.CoverageUnusable — I44's own "the report
// names why", never silently dropped.
func loadConserveCoverage(in *conserve.Input, root, coverageMetaFlag string) {
	conserve.LoadConfiguredCoverage(in, root, coverageMetaFlag)
}

// discoverPlanPaths returns every docs/plans/*.md at root, non-recursive —
// the same set the pre-push gate walks (lefthook.yml, docs/plans/
// README.md: "both the pre-push verify command and CI walk only the top
// level of docs/plans/"). A glob error or an absent directory yields an
// empty slice, never a failure: --since still runs, just with no prior
// declarations to excuse anything.
func discoverPlanPaths(root string) []string {
	matches, err := filepath.Glob(filepath.Join(root, "docs", "plans", "*.md"))
	if err != nil {
		return nil
	}
	sort.Strings(matches)
	return matches
}

// printConserveReport renders rep as a short, scannable text table — one
// line per plane (verdict, finding count), findings/missing indented
// beneath, then the per-language tier line. Mirrors cmd/fathom/review.go's
// printReviewSummary shape.
func printConserveReport(w io.Writer, rep conserve.Report) {
	fmt.Fprintf(w, "conserve: %s (base %s, head %s)\n", rep.Service, rep.Base, rep.Head)
	for _, pr := range rep.Planes {
		mark := "[x]"
		if pr.Verdict != conserve.Conserved {
			mark = "[ ]"
		}
		fmt.Fprintf(w, "%s %s: %s (%d finding(s))\n", mark, pr.Plane, pr.Verdict, len(pr.Findings))
		for _, m := range pr.Missing {
			fmt.Fprintf(w, "    missing: %s\n", m)
		}
		for _, f := range pr.Findings {
			cite := f.Head
			if cite == "" {
				cite = f.Base
			}
			suffix := ""
			if cite != "" {
				suffix = fmt.Sprintf(" (%s)", cite)
			}
			// f.Declared is the wire value (JSON keeps "prior" for a
			// "moved" finding — see conserve.Finding.DeclaredLabel's own
			// comment); the human-facing line renders the LABEL instead,
			// so a "moved" finding never says both "no retirement was
			// declared" and "(declared: prior)" in the same breath.
			declaredNote := fmt.Sprintf("declared: %s", f.Declared)
			if label := f.DeclaredLabel(); label != f.Declared {
				declaredNote = label
			}
			fmt.Fprintf(w, "  - [%s] %s: %s (%s)%s\n", f.Kind, f.Key, f.Detail, declaredNote, suffix)
			if f.Remedy != "" {
				fmt.Fprintf(w, "      remedy: %s\n", f.Remedy)
			}
		}
	}
	if len(rep.Tiers) > 0 {
		fmt.Fprint(w, "tiers:")
		for _, t := range rep.Tiers {
			fmt.Fprintf(w, " %s=%s", t.Language, t.Tier)
		}
		fmt.Fprintln(w)
	}
}

// printConserveResidue renders the reading set (CV-64, design §3.4) AFTER
// every plane's own table — it is not a plane, changes no verdict, and
// never affects --strict's exit, so it prints last, clearly set apart from
// the plane-by-plane findings above it.
func printConserveResidue(w io.Writer, rep conserve.ResidueReport) {
	fmt.Fprintf(w, "reading set: %d changed, %d attested, %d unattested, %d unreached, %d partial, %d unexecuted, %d unanalyzed function(s)\n",
		rep.Changed, rep.Attested, rep.Unattested, rep.Unreached, rep.Partial, rep.Unexecuted, rep.UnanalyzedFunctions)
	if rep.CoverageUnusable != "" {
		fmt.Fprintf(w, "    coverage unusable: %s (falling back to static reach)\n", rep.CoverageUnusable)
	}
	if rep.CoverageUnmapped > 0 {
		fmt.Fprintf(w, "    coverage unmapped: %d profile file key(s) matched no module here, e.g. %s (functions in them fall back to static reach)\n",
			rep.CoverageUnmapped, strings.Join(rep.CoverageUnmappedSample, ", "))
	}
	for _, m := range rep.Missing {
		fmt.Fprintf(w, "    missing: %s\n", m)
	}
	for _, u := range rep.Unanalyzed {
		fmt.Fprintf(w, "    unanalyzed: %s\n", u)
	}
	for _, it := range rep.Items {
		tests := "no reaching test"
		if len(it.Tests) > 0 {
			tests = strings.Join(it.Tests, ", ")
		}
		fmt.Fprintf(w, "  - [%s] %s %s (%s:%d) — callers=%d, evidence=%s, tests: %s\n",
			it.Class, it.Change, it.Symbol, it.File, it.Line, it.Callers, it.Evidence, tests)
		if len(it.UnexecutedLines) > 0 {
			fmt.Fprintf(w, "      unexecuted: %s\n", strings.Join(it.UnexecutedLines, ", "))
		}
	}
	if rep.Truncated > 0 {
		fmt.Fprintf(w, "    (%d more item(s) cut by --residue-max)\n", rep.Truncated)
	}
}
