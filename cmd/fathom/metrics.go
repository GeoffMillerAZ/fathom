package main

// metrics.go — Q-23 (docs/plans/expert-suite-q.md): `fathom metrics`
// dispatch, mirroring cmdModule's dispatch shape exactly (module.go).
// `docs/design/observability-surfaces.md` §8 items 2/4: the five
// `fathom metrics {spend,traffic,channels,interventions,series}` verbs,
// plus `rollup`/`rotate` (the daily-rollup builder and the explicit,
// archive-never-delete retention verb).
//
// SR-31 (surface reckoning F-52, design §4) folds `instrument`, `flywheel`,
// `retro`, `rate` and `arm` in as five more subverbs of this SAME `metrics`
// dispatcher — the measurement/observability verb was already the natural
// home, not a new competing one. cmdMetrics stays a pure dispatcher for
// these too: each keeps its own flags, output and exit code byte-for-byte
// (TestMetricsSubverbsMatchRetiredVerbs). The five top-level verbs
// themselves stay fully dispatched and callable — verbs.go's `Hidden`
// flag (the SR-16/SR-18/SR-30/SR-32 precedent) freezes them out of
// `fathom help` for one release, so hooks and skills that already call
// them directly keep working, then they go.

import (
	"fmt"
	"io"
	"time"
)

// metricsVerbList is the one place both the empty-args and unknown-verb
// error messages draw their verb list from, so SR-31's five additions
// never drift out of sync between the two.
const metricsVerbList = "spend | traffic | channels | interventions | series | rollup | rotate | impact | catches | sessions | load | instrument | flywheel | retro | rate | arm"

func cmdMetrics(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "error: metrics requires a verb: %s\n", metricsVerbList)
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
	case "impact":
		return cmdMetricsImpact(args[1:], stdout, stderr)
	case "catches":
		return cmdMetricsCatches(args[1:], stdout, stderr)
	case "load":
		return cmdMetricsLoad(args[1:], stdout, stderr)
	case "sessions":
		return cmdMetricsSessions(args[1:], stdout, stderr)
	// SR-31 (surface reckoning F-52, design §4): the five verbs the
	// reckoning consolidates, dispatched byte-for-byte unchanged.
	case "instrument":
		return cmdInstrument(args[1:], stdout, stderr)
	case "flywheel":
		return cmdFlywheel(args[1:], stdout, stderr)
	case "retro":
		return cmdRetro(args[1:], stdout, stderr)
	case "rate":
		return cmdRate(args[1:], stdout, stderr)
	case "arm":
		return cmdArm(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown metrics verb %q (want %s)\n", args[0], metricsVerbList)
		printMetricsUsage(stderr)
		return 1
	}
}

func printMetricsUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  fathom metrics spend         --store PATH [--service NAME] [--module LAYER] [--by layer|day|model] [--since DUR | --from RFC3339 --to RFC3339] [--root DIR] [--json]")
	fmt.Fprintln(w, "  fathom metrics traffic       [--service NAME] [--by surface|tool|hour|day|caller] [--since DUR | --from RFC3339 --to RFC3339] [--call-log PATH] [--url U] [--json]")
	fmt.Fprintln(w, "  fathom metrics channels      [--since DUR | --from RFC3339 --to RFC3339] [--hooks-log PATH] [--json]")
	fmt.Fprintln(w, "  fathom metrics interventions [--since DUR | --from RFC3339 --to RFC3339] [--store PATH] [--call-log PATH] [--hooks-log PATH] [--json]")
	fmt.Fprintln(w, "  fathom metrics series        <metric> --bucket hour|day [--since DUR] [--root DIR] [--json]  (<metric> may also come after the flags)")
	fmt.Fprintln(w, "  fathom metrics rollup        [--date YYYY-MM-DD] [--root DIR] [--store PATH] [--call-log PATH] [--hooks-log PATH] [--json]")
	fmt.Fprintln(w, "  fathom metrics rotate        [--call-log PATH] [--keep-days N] [--apply]")
	fmt.Fprintln(w, "  fathom metrics impact        [--call-log PATH] [--url U] [--json]")
	fmt.Fprintln(w, "  fathom metrics catches       [--kind K]... [--service S] [--days N] [--page N] [--per-page N] [--call-log PATH] [--url U] [--json]")
	fmt.Fprintln(w, "  fathom metrics sessions      [--arm on|off] [--repo R] [--q TEXT] [--sort KEY] [--asc] [--page N] [--per-page N] [--json]")
	fmt.Fprintln(w, "  fathom metrics sessions      --id SESSION [--channel C]... [--page N] [--per-page N] [--json]")
	fmt.Fprintln(w, "  fathom metrics load          [--call-log PATH] [--url U] [--json]")
	fmt.Fprintln(w, "  fathom metrics instrument    [--log PATH] [--window DUR] | --adoption [...] (the flywheel gauge)")
	fmt.Fprintln(w, "  fathom metrics flywheel      [--service S] [--url U] [--json]")
	fmt.Fprintln(w, "  fathom metrics retro         [--session S] [--project DIR] [--write] [--json]")
	fmt.Fprintln(w, "  fathom metrics rate          <1-5> [--note \"...\"] [--session S] [--repo R] | --list")
	fmt.Fprintln(w, "  fathom metrics arm           off --for D --sessions N [--note ...] | on")
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

// metricsRunner runs `fathom metrics` (registered below; EW-45).
type metricsRunner struct{}

func (metricsRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdMetrics(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "metrics", Group: "govern", Order: 80, Run: metricsRunner{},
		Summary: "Observability surfaces: spend | traffic | channels | interventions | series | rollup | rotate | impact (the four questions, arms as ranges) | catches (the catches timeline) | sessions (the session table, or one session's windowed replay) | load (calls per day per surface, tools by caller class) | instrument | flywheel | retro | rate | arm (SR-31)"})
}
