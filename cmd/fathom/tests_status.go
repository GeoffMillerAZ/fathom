package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// cmdTestsStatus implements "fathom tests status" (vision 33 / ADR-043
// stage 0, TC-4): per declared suite, its class, tier, the last ingested
// run's verdict and totals, and the case-to-symbol join rate
// (internal/testevidence.JoinCasesToSymbols, computed at ingest and read
// back here via store.TestSuiteStatuses). "no manifests found" (no
// suites declared at all) is a named skip, never a clean empty table.
//
//	fathom tests status [--service S] [--store DB] [--json]
func cmdTestsStatus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("tests status", stderr)
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name, the same guess the Claude Code hooks use)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: tests status requires --service (could not guess one from the current directory)")
			return 1
		}
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	ctx := context.Background()

	statuses, err := st.TestSuiteStatuses(ctx, svc)
	if err != nil {
		fmt.Fprintf(stderr, "error: tests status for %q: %v\n", svc, err)
		return 1
	}

	if *jsonOut {
		return renderTestsStatusJSON(stdout, svc, statuses)
	}
	renderTestsStatusTable(stdout, statuses)
	return 0
}

func renderTestsStatusTable(w io.Writer, statuses []store.TestSuiteStatus) {
	if len(statuses) == 0 {
		fmt.Fprintln(w, "tests status: no manifests found — nothing declared")
		return
	}
	fmt.Fprintf(w, "%-20s %-16s %-10s %-10s %-10s %-10s %s\n", "COMPONENT/SUITE", "CLASS", "TIER", "LAST RUN", "TOTALS", "JOINED", "")
	for _, s := range statuses {
		name := s.Component + "/" + s.Suite
		lastRun := "never"
		totals := "-"
		joined := fmt.Sprintf("joined %d of %d cases", s.CasesJoined, s.CasesTotal)
		if s.HasRun {
			lastRun = s.LastRun.Verdict
			totals = s.LastRun.TotalsJSON
		}
		fmt.Fprintf(w, "%-20s %-16s %-10s %-10s %-10s %s\n", name, s.Class, s.Tier, lastRun, totals, joined)
	}
}

func renderTestsStatusJSON(w io.Writer, service string, statuses []store.TestSuiteStatus) int {
	items := make([]any, len(statuses))
	for i, s := range statuses {
		item := map[string]any{
			"component": s.Component,
			"suite":     s.Suite,
			"class":     s.Class,
			"tier":      s.Tier,
			"has_run":   s.HasRun,
			"joined":    s.CasesJoined,
			"total":     s.CasesTotal,
		}
		if s.HasRun {
			item["verdict"] = s.LastRun.Verdict
			item["run_digest"] = s.LastRun.RunDigest
			item["started_at"] = s.LastRun.StartedAt
			var totals map[string]any
			if err := json.Unmarshal([]byte(s.LastRun.TotalsJSON), &totals); err == nil {
				item["totals"] = totals
			}
		}
		items[i] = item
	}
	payload := map[string]any{
		"service": service,
		"suites":  items,
	}
	if len(statuses) == 0 {
		payload["skip"] = "no manifests found"
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 1
	}
	return 0
}
