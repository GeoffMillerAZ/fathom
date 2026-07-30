package main

// metrics_channels.go — Q-18 (docs/plans/expert-suite-q.md): `fathom
// metrics channels`. Reuses readBeatLog/beatLogPath (beats.go,
// research_report.go — same package).

import (
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/metrics"
)

func cmdMetricsChannels(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics channels", stderr)
	since := fs.Duration("since", 7*24*time.Hour, "report window, measured back from now (ignored if --from/--to given)")
	from := fs.String("from", "", "window start, RFC3339 (must be given with --to)")
	to := fs.String("to", "", "window end, RFC3339 (must be given with --from)")
	hooksLogFlag := fs.String("hooks-log", "", "hooks beat log (JSONL); default: $FATHOM_HOOKS_LOG or ~/.fathom/hooks.jsonl")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	sinceT, untilT, werr := resolveMetricsWindow(*since, *from, *to, time.Now())
	if werr != nil {
		fmt.Fprintf(stderr, "error: %v\n", werr)
		return 1
	}

	hooksLogPath := *hooksLogFlag
	if hooksLogPath == "" {
		hooksLogPath = beatLogPath()
	}
	beats := readBeatsFor(hooksLogPath) // absent/unreadable log: AggregateChannels reports honest zeros/notes

	report := metrics.AggregateChannels(beats, sinceT, untilT)

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "channels: %s .. %s\n", report.Since, report.Until)
	fmt.Fprintf(stdout, "  injection: seen=%d answered=%d served=%d\n", report.InjectionSeen, report.InjectionAnswered, report.ObligationsServed)
	if report.InjectionCoverageRate != nil {
		fmt.Fprintf(stdout, "    coverage_rate=%.1f%%\n", *report.InjectionCoverageRate*100)
	} else {
		fmt.Fprintf(stdout, "    coverage_rate: %s\n", report.InjectionCoverageNote)
	}
	fmt.Fprintf(stdout, "  obligations_evicted: %s\n", report.ObligationsEvictedNote)
	fmt.Fprintf(stdout, "  guard: pre_seen=%d pre_served=%d post_seen=%d post_served=%d dedup_suppressed=%d\n",
		report.GuardPreSeen, report.GuardPreServed, report.GuardPostSeen, report.GuardPostServed, report.GuardDedupSuppressed)
	fmt.Fprintf(stdout, "    %s\n", report.GuardCoverageNote)
	if len(report.ModuleRulesServed) > 0 {
		fmt.Fprintf(stdout, "  module_rules_served: %+v\n", report.ModuleRulesServed)
	} else {
		fmt.Fprintf(stdout, "  module_rules_served: %s\n", report.ModuleServingNote)
	}
	return 0
}
