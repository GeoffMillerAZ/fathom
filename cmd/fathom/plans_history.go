package main

// plans_history.go — `fathom plans history` (OC-22, design §3 "Burn-up"):
// one plan's scope against claimed rows over its own git history,
// cross-checked with the verify ledger's runs of it, with the gate's proven
// trend drawn only once the ledger holds 14 days of this plan's rows (SD-3).
// The plan page's burn-up panel renders this verb's JSON (P10).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// planHistoryMaxPoints bounds the revisions a burn-up draws.
const planHistoryMaxPoints = 30

func cmdPlansHistory(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans history", stderr)
	service := fs.String("service", "", "service ID (required)")
	plan := fs.String("plan", "", "the plan: a path, a path under the service's registered root, or a name under its docs/plans (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (resolves the service's registered root)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" || *plan == "" {
		fmt.Fprintln(stderr, "error: --service and --plan are required")
		fs.Usage()
		return 1
	}
	root := ""
	if svcs, err := fetchServices(&http.Client{Timeout: 4 * time.Second}, *url); err == nil {
		if info, ok := svcs[strings.ToLower(*service)]; ok {
			root = info.path
		}
	}
	path, rel, err := resolvePlanFile(root, *plan)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	b := planBurnupFor(*service, root, path, rel, nil)
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(b)
		return 0
	}
	fmt.Fprintf(stdout, "burn-up: %s — %d commit(s)", rel, b.Commits)
	if b.Omitted > 0 {
		fmt.Fprintf(stdout, ", the oldest %d not drawn", b.Omitted)
	}
	fmt.Fprintln(stdout)
	if b.State != console.StatePresent {
		fmt.Fprintf(stdout, "  no history: %s\n", b.Reason)
	}
	for _, p := range b.Points {
		fmt.Fprintf(stdout, "  %s  %s  scope %3d  claimed %3d\n", p.At, p.Commit, p.Scope, p.Claimed)
	}
	if b.Proven.State == console.StatePresent {
		fmt.Fprintf(stdout, "proven trend (verify ledger, %d days):\n", b.Proven.Days)
		for _, p := range b.Proven.Points {
			fmt.Fprintf(stdout, "  %s  proven %3d of %3d claimed  failing %d\n", p.Day, p.Proven, p.Claimed, p.Failing)
		}
	} else {
		fmt.Fprintf(stdout, "proven trend: %s\n", b.Proven.Reason)
	}
	cc := b.CrossCheck
	fmt.Fprintf(stdout, "cross-check: %d of %d ledger run(s) on a plan revision agree with it; %d ran on a commit that did not touch the plan\n", cc.Agree, cc.Matched, cc.Unmatched)
	for _, d := range cc.Disagree {
		fmt.Fprintf(stdout, "  disagrees: %s\n", d)
	}
	return 0
}

// resolvePlanFile finds a plan named on the command line: an absolute path,
// a path under root, or a name (with or without .md) under root/docs/plans;
// with no root, a path relative to the working directory. rel is the path
// under root the gate names (docs/plans/x.md).
func resolvePlanFile(root, arg string) (abs, rel string, err error) {
	var candidates []string
	switch {
	case filepath.IsAbs(arg):
		candidates = []string{arg}
	case root != "":
		candidates = []string{filepath.Join(root, arg), filepath.Join(root, "docs", "plans", arg), filepath.Join(root, "docs", "plans", arg+".md")}
	default:
		candidates = []string{arg}
	}
	for _, c := range candidates {
		if st, serr := os.Stat(c); serr == nil && !st.IsDir() {
			abs, _ = filepath.Abs(c)
			base := root
			if base == "" {
				base = verifyRepoRoot()
			}
			if r, ok := relativeToRoot(base, abs); ok {
				return abs, r, nil
			}
			return abs, filepath.Base(abs), nil
		}
	}
	return "", "", fmt.Errorf("plan %q not found (tried %s)", arg, strings.Join(candidates, ", "))
}

// planBurnupFor is the burn-up the verb prints and the plan page draws:
// the plan's git history in root and the verify ledger's runs of it. runs,
// when non-nil, is an already-read ledger (the page's cached read).
func planBurnupFor(service, root, path, rel string, ledger *ledgerRead) console.Burnup {
	base := root
	if base == "" {
		base = verifyRepoRoot()
	}
	revs, older, note := planGitHistory(base, rel)
	return planBurnupFrom(service, path, revs, older, note, ledger)
}

// planBurnupFrom is planBurnupFor over a history already read.
func planBurnupFrom(service, path string, revs []planRevisionText, older int, note string, ledger *ledgerRead) console.Burnup {
	if ledger == nil {
		runs, lpath, lerr := ledgerRuns()
		ledger = &ledgerRead{Runs: runs, Path: lpath, Err: lerr}
	}
	b := console.PlanBurnup(console.BurnupInput{
		Service: service, Plan: path,
		Revisions: planRevisions(service, revs), HistoryNote: note,
		Ledger: ledger.Runs, LedgerPath: ledger.Path, LedgerErr: ledger.Err,
		MinDays: console.DefaultTrendDays, MaxPoints: planHistoryMaxPoints,
	})
	b.Commits += older
	b.Omitted += older
	return b
}

// ledgerRead is one read of the verify ledger.
type ledgerRead struct {
	Runs []console.LedgerRun
	Path string
	Err  string
}
