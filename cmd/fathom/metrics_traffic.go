package main

// metrics_traffic.go — Q-17 (docs/plans/expert-suite-q.md): `fathom
// metrics traffic`. Reuses readCallLog/resolveCallLogPath
// (instrument_adoption.go, same package) and converts each
// adoptionCallEntry into an internal/metrics.CallEntry.

import (
	"encoding/json"
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
	result := trafficReport(resolved, *service, *by, sinceT, untilT)
	report := result.Report

	if *jsonOut {
		jsonPrint(stdout, withCallLog(result, resolved))
		return 0
	}
	fmt.Fprintf(stdout, "traffic: %s .. %s (by %s)\n", report.Since, report.Until, report.GroupBy)
	fmt.Fprintln(stdout, callLogLine(resolved))
	if result.Reason != "" {
		label := "not measured"
		if result.Error {
			label = "unreadable"
		}
		fmt.Fprintf(stdout, "  %s — %s\n", label, result.Reason)
		return 0
	}
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

// trafficResult is `metrics traffic`'s answer: the report over a read log,
// or — when the log is absent or could not be read — the report's window
// and grouping with the read's truth state in place of every count. An
// unread log is never an idle daemon (cmd/fathom/CLAUDE.md rule 6: absent
// is never 0, and JSON carries no zero for it).
type trafficResult struct {
	Report metrics.TrafficReport
	Reason string // "" when the log was read
	Error  bool   // the log exists and could not be read
}

// MarshalJSON writes the report itself, or {since, until, group_by, rows:
// [], state, reason, note} with no count at all when the log was not read.
func (r trafficResult) MarshalJSON() ([]byte, error) {
	if r.Reason == "" {
		return json.Marshal(r.Report)
	}
	state := console.StateAbsent
	if r.Error {
		state = console.StateError
	}
	return json.Marshal(struct {
		Since   string               `json:"since"`
		Until   string               `json:"until"`
		GroupBy string               `json:"group_by"`
		Rows    []metrics.TrafficRow `json:"rows"`
		State   console.State        `json:"state"`
		Reason  string               `json:"reason"`
		Note    string               `json:"note"`
	}{r.Report.Since, r.Report.Until, r.Report.GroupBy, []metrics.TrafficRow{}, state, r.Reason, r.Reason})
}

// trafficReport is `metrics traffic`'s computation over the resolved log;
// the console's Operate page calls trafficResultFrom over its own read
// with the same classification (P10). An absent or unreadable log is a
// result naming why, never fatal and never zeros.
func trafficReport(resolved console.ResolvedCallLog, service, by string, since, until time.Time) trafficResult {
	var entries []adoptionCallEntry
	var err error
	if resolved.Path != "" {
		entries, _, _, err = callLogReadCountingSince(resolved.Path, since)
	}
	reason, isErr, _ := callLogState(callLogFactsFor(resolved, entries, err))
	return trafficResultFrom(entries, reason, isErr, service, by, since, until)
}

// trafficResultFrom is trafficReport over entries already read, with that
// read's state (callLogState: "" when it read).
func trafficResultFrom(raw []adoptionCallEntry, reason string, isErr bool, service, by string, since, until time.Time) trafficResult {
	return trafficResult{Report: trafficReportFrom(raw, service, by, since, until), Reason: reason, Error: isErr}
}

// trafficReportFrom is the aggregation over entries already read.
func trafficReportFrom(raw []adoptionCallEntry, service, by string, since, until time.Time) metrics.TrafficReport {
	entries, _ := callDataFrom(raw, service, since, until)
	return metrics.AggregateTraffic(entries, metrics.TrafficOptions{Since: since, Until: until, GroupBy: by})
}
