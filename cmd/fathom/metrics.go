package main

// metrics.go — Q-23 (docs/plans/expert-suite-q.md): `fathom metrics`
// dispatch, mirroring cmdModule's dispatch shape exactly (module.go).
// `docs/design/observability-surfaces.md` §8 items 2/4: the five
// `fathom metrics {spend,traffic,channels,interventions,series}` verbs,
// plus `rollup`/`rotate` (the daily-rollup builder and the explicit,
// archive-never-delete retention verb).

import (
	"fmt"
	"io"
	"time"
)

func cmdMetrics(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: metrics requires a verb: spend | traffic | channels | interventions | series | rollup | rotate")
		printMetricsUsage(stderr)
		return 1
	}
	switch args[0] {
	case "spend":
		return cmdMetricsSpend(args[1:], stdout, stderr)
	case "traffic":
		return cmdMetricsTraffic(args[1:], stdout, stderr)
	case "channels":
		return cmdMetricsChannels(args[1:], stdout, stderr)
	case "interventions":
		return cmdMetricsInterventions(args[1:], stdout, stderr)
	case "series":
		return cmdMetricsSeries(args[1:], stdout, stderr)
	case "rollup":
		return cmdMetricsRollupBuild(args[1:], stdout, stderr)
	case "rotate":
		return cmdMetricsRotate(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown metrics verb %q (want spend | traffic | channels | interventions | series | rollup | rotate)\n", args[0])
		printMetricsUsage(stderr)
		return 1
	}
}

func printMetricsUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  fathom metrics spend         --store PATH [--service NAME] [--module LAYER] [--by layer|day|model] [--since DUR | --from RFC3339 --to RFC3339] [--root DIR] [--json]")
	fmt.Fprintln(w, "  fathom metrics traffic       [--service NAME] [--by surface|tool|hour] [--since DUR | --from RFC3339 --to RFC3339] [--call-log PATH] [--json]")
	fmt.Fprintln(w, "  fathom metrics channels      [--since DUR | --from RFC3339 --to RFC3339] [--hooks-log PATH] [--json]")
	fmt.Fprintln(w, "  fathom metrics interventions [--since DUR | --from RFC3339 --to RFC3339] [--store PATH] [--call-log PATH] [--hooks-log PATH] [--json]")
	fmt.Fprintln(w, "  fathom metrics series        <metric> --bucket hour|day [--since DUR] [--root DIR] [--json]  (<metric> may also come after the flags)")
	fmt.Fprintln(w, "  fathom metrics rollup        [--date YYYY-MM-DD] [--root DIR] [--store PATH] [--call-log PATH] [--hooks-log PATH] [--json]")
	fmt.Fprintln(w, "  fathom metrics rotate        [--call-log PATH] [--keep-days N] [--apply]")
}

// resolveMetricsWindow is every metrics verb's shared --since/--from/--to
// resolver: --from/--to (RFC3339, given together) win when either is set;
// otherwise the window is [now-sinceDur, now] (--since, phase-P's own
// duration-flag precedent, default handled by each verb's own flag
// default).
func resolveMetricsWindow(sinceDur time.Duration, from, to string, now time.Time) (since, until time.Time, err error) {
	if from != "" || to != "" {
		if from == "" || to == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("--from and --to must be given together")
		}
		since, err = time.Parse(time.RFC3339, from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--from: %w", err)
		}
		until, err = time.Parse(time.RFC3339, to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--to: %w", err)
		}
		if until.Before(since) {
			return time.Time{}, time.Time{}, fmt.Errorf("--to (%s) is before --from (%s)", to, from)
		}
		return since, until, nil
	}
	until = now.UTC()
	since = until.Add(-sinceDur)
	return since, until, nil
}
