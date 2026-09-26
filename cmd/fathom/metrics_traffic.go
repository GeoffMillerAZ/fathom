package main

// metrics_traffic.go — Q-17 (docs/plans/expert-suite-q.md): `fathom
// metrics traffic`. Reuses readCallLog/resolveCallLogPath
// (instrument_adoption.go, same package) and converts each
// adoptionCallEntry into an internal/metrics.CallEntry.

import (
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/metrics"
)

func cmdMetricsTraffic(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics traffic", stderr)
	service := fs.String("service", "", "optional service filter (matches the call log's service field)")
	by := fs.String("by", metrics.TrafficGroupBySurface, "group by: surface | tool | hour | day | caller (agent, edit hook, gate, refresh, plumbing, unattributed)")
	since := fs.Duration("since", 7*24*time.Hour, "report window, measured back from now (ignored if --from/--to given)")
	from := fs.String("from", "", "window start, RFC3339 (must be given with --to)")
	to := fs.String("to", "", "window end, RFC3339 (must be given with --from)")
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: [daemon].call_log in config.toml, else "+defaultCallLogPath())
	url := fs.String("url", defaultFathomURL(), "the daemon whose record names the call log")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	switch *by {
	case metrics.TrafficGroupBySurface, metrics.TrafficGroupByTool, metrics.TrafficGroupByHour, metrics.TrafficGroupByDay, metrics.TrafficGroupByCaller:
	default:
		fmt.Fprintf(stderr, "error: --by must be surface, tool, hour, day or caller (got %q)\n", *by)
		return 1
	}

	sinceT, untilT, werr := resolveMetricsWindow(*since, *from, *to, consoleVerbNow())
	if werr != nil {
		fmt.Fprintf(stderr, "error: %v\n", werr)
		return 1
	}

	// OC-2: the one call-log resolution (console_calllog.go) — the daemon's
	// configured log when a daemon record names one, never a log relative
	// to this command's working directory.
	resolved := resolveConsoleCallLogFor(callLogQuery{Flag: *callLogFlag, BaseURL: *url})
	report := trafficReport(resolved, *service, *by, sinceT, untilT)

	if *jsonOut {
		jsonPrint(stdout, withCallLog(report, resolved))
		return 0
	}
	fmt.Fprintf(stdout, "traffic: %s .. %s (by %s)\n", report.Since, report.Until, report.GroupBy)
	fmt.Fprintln(stdout, callLogLine(resolved))
	if report.TotalCalls == 0 {
		fmt.Fprintf(stdout, "  %s\n", report.Note)
		return 0
	}
	for _, row := range report.Rows {
		errRate := row.ErrorRateNote
		if row.ErrorRate != nil {
			errRate = fmt.Sprintf("%.1f%%", *row.ErrorRate)
		}
		fmt.Fprintf(stdout, "  %-20s calls=%-6d p50=%.0fms p95=%.0fms error_rate=%s\n",
			row.Group, row.Calls, *row.P50MS, *row.P95MS, errRate)
	}
	fmt.Fprintf(stdout, "total: calls=%d discovery=%d plumbing=%d other=%d\n", report.TotalCalls, report.DiscoveryCalls, report.PlumbingCalls, report.OtherCalls)
	fmt.Fprintf(stdout, "%s\n", report.Note)
	return 0
}

// trafficReport is `metrics traffic`'s computation over the resolved log;
// the console's Operate page calls it for its sections (P10). An absent or
// unreadable log is an empty report whose note says so, never fatal.
func trafficReport(resolved console.ResolvedCallLog, service, by string, since, until time.Time) metrics.TrafficReport {
	raw, _, _ := readCallLog(resolved.Path)
	return trafficReportFrom(raw, service, by, since, until)
}

// trafficReportFrom is trafficReport over entries already read.
func trafficReportFrom(raw []adoptionCallEntry, service, by string, since, until time.Time) metrics.TrafficReport {
	entries, _ := callDataFrom(raw, service, since, until)
	return metrics.AggregateTraffic(entries, metrics.TrafficOptions{Since: since, Until: until, GroupBy: by})
}
