package main

// metrics_series.go — Q-20 (docs/plans/expert-suite-q.md): `fathom metrics
// series <metric> --bucket hour|day --since`.
//
// --bucket day reads persisted daily rollups (metrics.ReadDailyRollups)
// plus TODAY's live tail (recomputed the same way `metrics rollup` itself
// would, over just today's window) — the design's own "rollups plus
// today's live tail" framing (§4).
//
// --bucket hour has NO persisted rollup to read at all (only daily
// granularity is ever written to disk) — it is computed ENTIRELY from live
// raw logs (call log + beat log) still on disk, so an hour bucket older
// than those logs' own retention window is unavailable; SeriesReport.Note
// states this honestly rather than silently returning zeros for buckets
// the raw logs no longer cover.

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/metrics"
)

// seriesMetricNames is the documented name→accessor map for `metric` — an
// unknown name lists these rather than silently returning empty.
var seriesMetricNames = []string{
	"spend_total_usd",
	"spend_cost_per_claim_usd",
	"traffic_total_calls",
	"traffic_error_rate",
	"channels_injection_coverage_rate",
	"channels_obligations_served",
	"interventions_unproven_done_refusals",
	"interventions_review_superseded",
}

// SeriesPoint is one bucket's value for the requested metric. Value is nil
// (with Note explaining why) whenever the underlying rollup/live field
// itself was nil — never a fabricated 0.
type SeriesPoint struct {
	Bucket string   `json:"bucket"`
	Value  *float64 `json:"value"`
	Note   string   `json:"note,omitempty"`
}

// SeriesReport is `metrics series`'s output.
type SeriesReport struct {
	Metric string        `json:"metric"`
	Bucket string        `json:"bucket"`
	Since  string        `json:"since"`
	Points []SeriesPoint `json:"points"`
	Note   string        `json:"note,omitempty"`
}

// seriesMetricValue extracts the named metric from a DailyRollup, mirroring
// seriesMetricNames' order/coverage exactly.
func seriesMetricValue(r metrics.DailyRollup, metricName string) (*float64, bool) {
	switch metricName {
	case "spend_total_usd":
		v := r.SpendTotalUSD
		return &v, true
	case "spend_cost_per_claim_usd":
		return r.SpendCostPerClaimUSD, true
	case "traffic_total_calls":
		v := float64(r.TrafficTotalCalls)
		return &v, true
	case "traffic_error_rate":
		return r.TrafficErrorRate, true
	case "channels_injection_coverage_rate":
		return r.ChannelsInjectionCoverageRate, true
	case "channels_obligations_served":
		v := float64(r.ChannelsObligationsServed)
		return &v, true
	case "interventions_unproven_done_refusals":
		if r.InterventionsUnprovenDoneRefusals == nil {
			return nil, true
		}
		v := float64(*r.InterventionsUnprovenDoneRefusals)
		return &v, true
	case "interventions_review_superseded":
		if r.InterventionsReviewSuperseded == nil {
			return nil, true
		}
		v := float64(*r.InterventionsReviewSuperseded)
		return &v, true
	default:
		return nil, false
	}
}

func cmdMetricsSeries(args []string, stdout, stderr io.Writer) int {
	// The metric name is accepted in EITHER position — `metrics series
	// spend_total_usd --bucket day` (the natural "subcommand-first" typing
	// most users reach for) or `metrics series --bucket day
	// spend_total_usd` (flag.Parse's own native positional-args-after-flags
	// order) — Go's flag package alone only supports the latter (it stops
	// parsing flags at the first non-flag token), so a leading non-flag
	// token is captured here before fs.Parse ever runs.
	var leadingMetric string
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		leadingMetric = args[0]
		rest = args[1:]
	}

	fs := newFlagSet("metrics series", stderr)
	bucket := fs.String("bucket", "day", "hour | day")
	since := fs.Duration("since", 7*24*time.Hour, "report window, measured back from now")
	root := fs.String("root", ".", "project root — the rollup directory resolves under here")
	rollupDir := fs.String("rollup-dir", "", "daily rollup directory; default: --root/"+defaultMetricsRollupDir)
	storePath := fs.String("store", "", "local store path (optional — for --bucket hour's spend numbers)")
	service := fs.String("service", "", "service ID (used only with --store)")
	module := fs.String("module", "", "module/layer filter (used only with --store)")
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: [daemon].call_log in config.toml, else "+defaultCallLogPath())
	hooksLogFlag := fs.String("hooks-log", "", "hooks beat log (JSONL); default: $FATHOM_HOOKS_LOG or ~/.fathom/hooks.jsonl")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	metricName := leadingMetric
	switch {
	case metricName == "" && fs.NArg() == 1:
		metricName = fs.Arg(0)
	case metricName == "" && fs.NArg() != 1:
		fmt.Fprintln(stderr, "error: metrics series requires exactly one positional <metric> argument")
		fs.Usage()
		return 1
	case metricName != "" && fs.NArg() != 0:
		fmt.Fprintf(stderr, "error: unexpected extra argument(s) %v\n", fs.Args())
		fs.Usage()
		return 1
	}
	if _, ok := seriesMetricValue(metrics.DailyRollup{}, metricName); !ok {
		fmt.Fprintf(stderr, "error: unknown metric %q — valid names: %s\n", metricName, joinNames(seriesMetricNames))
		return 1
	}
	if *bucket != "hour" && *bucket != "day" {
		fmt.Fprintf(stderr, "error: --bucket must be hour or day (got %q)\n", *bucket)
		return 1
	}

	now := time.Now().UTC()
	sinceT := now.Add(-*since)

	dir := *rollupDir
	if dir == "" {
		dir = filepath.Join(*root, filepath.FromSlash(defaultMetricsRollupDir))
	}

	callLogPath := *callLogFlag
	if callLogPath == "" {
		callLogPath = resolveCallLogPath()
	}
	hooksLogPath := *hooksLogFlag
	if hooksLogPath == "" {
		hooksLogPath = beatLogPath()
	}

	var report SeriesReport
	report.Metric, report.Bucket, report.Since = metricName, *bucket, sinceT.UTC().Format(time.RFC3339)

	if *bucket == "day" {
		rollups, rerr := metrics.ReadDailyRollups(dir, sinceT, now)
		if rerr != nil {
			fmt.Fprintf(stderr, "error: reading rollups: %v\n", rerr)
			return 1
		}
		for _, r := range rollups {
			v, _ := seriesMetricValue(r, metricName)
			pt := SeriesPoint{Bucket: r.Date, Value: v}
			if v == nil {
				pt.Note = "not measurable: this metric was nil in the rollup for this date"
			}
			report.Points = append(report.Points, pt)
		}
		// Today's live tail — rollups is date-bounded and TODAY'S rollup
		// (built by a scheduled `metrics rollup` run) may not exist yet.
		today := now.Format("2006-01-02")
		haveToday := false
		for _, p := range report.Points {
			if p.Bucket == today {
				haveToday = true
			}
		}
		if !haveToday {
			todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			live := buildLiveDailyRollup(todayStart, now, *storePath, *service, *module, callLogPath, hooksLogPath, stderr)
			v, _ := seriesMetricValue(live, metricName)
			pt := SeriesPoint{Bucket: today, Value: v}
			if v == nil {
				pt.Note = "not measurable: nil in today's live tail"
			}
			report.Points = append(report.Points, pt)
		}
		report.Note = "rollups plus today's live tail (docs/design/observability-surfaces.md §4)"
	} else {
		// --bucket hour: no persisted rollup exists at this granularity —
		// computed entirely from live raw logs still on disk.
		report.Note = "computed entirely from live raw logs (no persisted hourly rollup exists) — an hour bucket older than the raw logs' own retention window is simply absent from Points, never a fabricated zero"
		for h := sinceT.Truncate(time.Hour); !h.After(now); h = h.Add(time.Hour) {
			hEnd := h.Add(time.Hour - time.Nanosecond)
			live := buildLiveDailyRollup(h, hEnd, *storePath, *service, *module, callLogPath, hooksLogPath, stderr)
			v, _ := seriesMetricValue(live, metricName)
			pt := SeriesPoint{Bucket: h.Format("2006-01-02T15:00"), Value: v}
			if v == nil {
				pt.Note = "not measurable: nil for this hour"
			}
			report.Points = append(report.Points, pt)
		}
	}

	sort.Slice(report.Points, func(i, j int) bool { return report.Points[i].Bucket < report.Points[j].Bucket })

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "series: %s (bucket=%s, since=%s)\n", report.Metric, report.Bucket, report.Since)
	for _, p := range report.Points {
		if p.Value != nil {
			fmt.Fprintf(stdout, "  %-20s %.4f\n", p.Bucket, *p.Value)
		} else {
			fmt.Fprintf(stdout, "  %-20s %s\n", p.Bucket, p.Note)
		}
	}
	fmt.Fprintf(stdout, "%s\n", report.Note)
	return 0
}

// buildLiveDailyRollup computes one window's aggregate reports and flattens
// them via metrics.BuildDailyRollup — the same computation `metrics
// rollup` performs for a full day, reused here for both today's live tail
// (day bucket) and every hour bucket.
func buildLiveDailyRollup(since, until time.Time, storePath, service, module, callLogPath, hooksLogPath string, stderr io.Writer) metrics.DailyRollup {
	runs, _ := readCompileRunsFor(storePath, service, module)
	spend := metrics.AggregateSpend(runs, spendBudgetLookup("."), metrics.SpendOptions{Since: since, Until: until, GroupBy: metrics.SpendGroupByLayer})

	entries, verdicts := readCallDataFor(callLogPath, service, since, until)
	traffic := metrics.AggregateTraffic(entries, metrics.TrafficOptions{Since: since, Until: until, GroupBy: metrics.TrafficGroupBySurface})

	beats := readBeatsFor(hooksLogPath)
	channels := metrics.AggregateChannels(beats, since, until)

	claimEvents, queried := readClaimEventsFor(storePath, since, until, stderr)
	interventions := metrics.AggregateInterventions(verdicts, beats, claimEvents, queried, since, until)

	return metrics.BuildDailyRollup(since, spend, traffic, channels, interventions, until)
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
