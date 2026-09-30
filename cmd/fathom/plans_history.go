package main

// plans_history.go — `fathom plans history` (OC-22, design §3 "Burn-up"):
// one plan's scope against claimed rows over its own git history,
// cross-checked with the verify ledger's runs of it, with the gate's proven
// trend drawn only once the ledger holds 14 days of this plan's rows (SD-3).
// The plan page's burn-up panel renders this verb's JSON (P10).
//
// Without --plan it prints the claimed-vs-proven trend of the current plans
// of one service (--service) or of every registered service: the verify
// ledger's runs of those plans only, matched by service and file name
// (console.PlansTrend) — the ledger also holds scratch services, deleted
// plans and README.md, which a portfolio's trend must never sum. /plans'
// trend panel renders this verb's JSON, at the page's own ?svc= scope.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// planHistoryMaxPoints bounds the revisions a burn-up draws.
const planHistoryMaxPoints = 30

func cmdPlansHistory(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans history", stderr)
	service := fs.String("service", "", "service ID (required with --plan; without --plan, the trend of this service's plans instead of the registry's)")
	plan := fs.String("plan", "", "the plan: a path, a path under the service's registered root, or a name under its docs/plans (omit for the claimed-vs-proven trend of the current plans)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (resolves the service's registered root)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *plan == "" {
		return plansTrendCmd(*service, *url, *jsonOut, stdout, stderr)
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required with --plan")
		fs.Usage()
		return 1
	}
	root := ""
	svcs, svcErr := fetchServices(&http.Client{Timeout: 4 * time.Second}, *url)
	if svcErr == nil {
		if info, ok := svcs[strings.ToLower(*service)]; ok {
			root = info.path
		}
	}
	path, rel, err := resolvePlanFile(root, *plan)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	// PE-24: the ledger under the registered roots this one registry read names.
	ledger := readLedgerRuns(ledgerRootsOf(svcs, svcErr))
	b := planBurnupFor(*service, root, path, rel, &ledger)
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
	if p := ledger.Ignored.Phrase(verifyLedgerNoun); p != "" {
		fmt.Fprintf(stdout, "verify ledger: %s\n", p)
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
	revs, older, note := planGitHistory(context.Background(), base, rel)
	return planBurnupFrom(service, path, revs, older, note, ledger)
}

// planBurnupFrom is planBurnupFor over a history already read.
func planBurnupFrom(service, path string, revs []planRevisionText, older int, note string, ledger *ledgerRead) console.Burnup {
	if ledger == nil {
		read := readLedgerRuns(cliLedgerRoots(defaultFathomURL()))
		ledger = &read
	}
	b := console.PlanBurnup(console.BurnupInput{
		Service: service, Plan: path,
		Revisions: planRevisions(service, revs), HistoryNote: note,
		Ledger: ledger.Runs, LedgerPath: ledger.Path, LedgerErr: ledger.Err,
		MinDays: console.DefaultTrendDays, MaxPoints: planHistoryMaxPoints,
	})
	b.Commits += older
	b.Omitted += older
	b.Proven.Source = withIgnored(b.Proven.Source, ledger.Ignored, verifyLedgerNoun)
	return b
}

// ledgerRead is one read of the verify ledger, kept to the registered
// roots; Ignored is what that left out (PE-24).
type ledgerRead struct {
	Runs    []console.LedgerRun
	Path    string
	Err     string
	Ignored ledgerroots.Ignored
}

// readLedgerRuns is ledgerRuns as one ledgerRead.
func readLedgerRuns(roots ledgerroots.Roots) ledgerRead {
	runs, path, err, ignored := ledgerRuns(roots)
	return ledgerRead{Runs: runs, Path: path, Err: err, Ignored: ignored}
}

// plansTrendCmd prints the claimed-vs-proven trend of the current plans of
// one service, or of every registered service when service is "".
func plansTrendCmd(service, url string, jsonOut bool, stdout, stderr io.Writer) int {
	svcs, err := fetchServices(&http.Client{Timeout: 5 * time.Second}, url)
	if err != nil {
		fmt.Fprintf(stderr, "error: the service registry at %s could not be read: %v\n", url, err)
		return 1
	}
	var roots []serviceRoot
	for name, info := range svcs {
		if service == "" || strings.EqualFold(name, service) {
			roots = append(roots, serviceRoot{Service: name, Root: info.path})
		}
	}
	if service != "" && len(roots) == 0 {
		fmt.Fprintf(stderr, "error: no indexed service named %q\n", service)
		return 1
	}
	ledger := readLedgerRuns(ledgerRootsOf(svcs, nil)) // PE-24: the registered roots' rows
	t := plansTrendOf(roots, ledger)
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(t)
		return 0
	}
	fmt.Fprintf(stdout, "claimed against proven: %s\n", t.Scope)
	if p := ledger.Ignored.Phrase(verifyLedgerNoun); p != "" {
		fmt.Fprintf(stdout, "  verify ledger: %s\n", p)
	}
	if t.State != console.StatePresent {
		fmt.Fprintf(stdout, "  not drawn: %s\n", t.Reason)
		return 0
	}
	for _, p := range t.Points {
		fmt.Fprintf(stdout, "  %s  proven %4d of %4d claimed  failing %3d  plans %3d\n", p.Day, p.Proven, p.Claimed, p.Failing, p.Plans)
	}
	return 0
}

// plansTrendOf is the trend of the current plans under roots — the one
// discovery (servicePlanFiles) both this verb and /plans read, so the page
// and the verb count the same plans.
func plansTrendOf(roots []serviceRoot, ledger ledgerRead) console.Trend {
	sps := make([]console.ServicePlans, 0, len(roots))
	for _, sr := range roots {
		sp := console.ServicePlans{Service: sr.Service, Root: sr.Root}
		files, errText := servicePlanFiles(sr.Root)
		sp.Err = errText
		for _, f := range files {
			sp.Plans = append(sp.Plans, console.PlanFacts{Service: sr.Service, Path: f})
		}
		sps = append(sps, sp)
	}
	t := console.PlansTrend(ledger.Runs, ledger.Path, ledger.Err, console.DefaultTrendDays, sps)
	t.Source = withIgnored(t.Source, ledger.Ignored, verifyLedgerNoun)
	return t
}
