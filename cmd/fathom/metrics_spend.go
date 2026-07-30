package main

// metrics_spend.go — Q-16 (docs/plans/expert-suite-q.md): `fathom metrics
// spend`. Reads compile_runs via store.MountReadOnly (never a writable
// open), resolves each layer's budget via semantic.LoadLayerIn(--root, …),
// and calls internal/metrics.AggregateSpend.

import (
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/metrics"
)

func cmdMetricsSpend(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics spend", stderr)
	storePath := fs.String("store", "", "local store path (required)")
	// service is required — compile_runs.service_id has no "all services"
	// query mode (mirrors module_runs.go's own cmdModuleRuns requirement);
	// a store mounted read-only always describes exactly one service in
	// practice, so this is a real constraint, not busywork.
	service := fs.String("service", "", "service ID (required)")
	module := fs.String("module", "", "module/layer filter (optional)")
	by := fs.String("by", metrics.SpendGroupByLayer, "group by: layer | day | model")
	since := fs.Duration("since", 7*24*time.Hour, "report window, measured back from now (ignored if --from/--to given)")
	from := fs.String("from", "", "window start, RFC3339 (must be given with --to)")
	to := fs.String("to", "", "window end, RFC3339 (must be given with --from)")
	root := fs.String("root", ".", "project root — layer packs for budget lookup resolve under here")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: --store and --service are required")
		fs.Usage()
		return 1
	}
	switch *by {
	case metrics.SpendGroupByLayer, metrics.SpendGroupByDay, metrics.SpendGroupByModel:
	default:
		fmt.Fprintf(stderr, "error: --by must be layer, day, or model (got %q)\n", *by)
		return 1
	}

	sinceT, untilT, werr := resolveMetricsWindow(*since, *from, *to, time.Now())
	if werr != nil {
		fmt.Fprintf(stderr, "error: %v\n", werr)
		return 1
	}

	runs, rErr := readCompileRunsFor(*storePath, *service, *module)
	if rErr != nil {
		fmt.Fprintf(stderr, "error: %v\n", rErr)
		return 1
	}

	report := metrics.AggregateSpend(runs, spendBudgetLookup(*root), metrics.SpendOptions{Since: sinceT, Until: untilT, GroupBy: *by})

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "spend: %s .. %s (by %s)\n", report.Since, report.Until, report.GroupBy)
	if report.Note != "" && len(report.Rows) == 0 {
		fmt.Fprintf(stdout, "  %s\n", report.Note)
		return 0
	}
	for _, row := range report.Rows {
		costPerClaim := row.CostPerClaimNote
		if row.CostPerClaimUSD != nil {
			costPerClaim = fmt.Sprintf("$%.4f", *row.CostPerClaimUSD)
		}
		fmt.Fprintf(stdout, "  %-20s runs=%-4d cost=$%.4f claims=%-4d cost/claim=%s escape_hatch=%d\n",
			row.Group, row.Runs, row.CostUSD, row.ClaimsEmitted, costPerClaim, row.EscapeHatchRuns)
		if row.BudgetMaxUSD != nil {
			fmt.Fprintf(stdout, "    budget: max=$%.4f headroom=$%.4f\n", *row.BudgetMaxUSD, *row.BudgetHeadroomUSD)
		} else if row.BudgetNote != "" {
			fmt.Fprintf(stdout, "    budget: %s\n", row.BudgetNote)
		}
	}
	fmt.Fprintf(stdout, "total: runs=%d cost=$%.4f\n", report.TotalRuns, report.TotalCostUSD)
	return 0
}
