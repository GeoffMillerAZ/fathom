package main

// metrics_interventions.go — Q-19 (docs/plans/expert-suite-q.md): `fathom
// metrics interventions`. --store is OPTIONAL: it is needed only for row 2
// (review findings superseded, from claim_events) — every other row
// computes without it.

import (
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/metrics"
)

// defaultStorePath is the literal store-path default every required-store
// command in cmd/fathom already falls back to when unconfigured (ingest.go,
// query.go, review.go, et al.) — the same "repo's resolved store path" that
// resolveStorePath below layers [daemon].store over.
const defaultStorePath = "workspace/db/fathom.sqlite"

// resolveStorePath mirrors resolveCallLogPath's own default resolution
// (instrument_adoption.go): config.toml's [daemon].store, falling back to
// the literal default every other store-reading command already defaults
// to when unconfigured — so `fathom metrics interventions` stops silently
// skipping claim_events (row 2) just because --store was omitted (IT-8).
func resolveStorePath() string {
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		return defaultStorePath
	}
	return firstNonEmpty(loaded.Config.Daemon.Store, defaultStorePath)
}

func cmdMetricsInterventions(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics interventions", stderr)
	since := fs.Duration("since", 7*24*time.Hour, "report window, measured back from now (ignored if --from/--to given)")
	from := fs.String("from", "", "window start, RFC3339 (must be given with --to)")
	to := fs.String("to", "", "window end, RFC3339 (must be given with --from)")
	storePath := fs.String("store", "", "local store path — needed only for the review-findings-superseded row; defaults to [daemon].store in config.toml, else "+defaultStorePath)
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: [daemon].call_log in config.toml, else "+defaultCallLogPath)
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

	callLogPath := *callLogFlag
	if callLogPath == "" {
		callLogPath = resolveCallLogPath()
	}
	_, callVerdicts := readCallDataFor(callLogPath, "", sinceT, untilT)

	hooksLogPath := *hooksLogFlag
	if hooksLogPath == "" {
		hooksLogPath = beatLogPath()
	}
	beats := readBeatsFor(hooksLogPath)

	storePathResolved := *storePath
	if storePathResolved == "" {
		storePathResolved = resolveStorePath()
	}
	claimEvents, claimEventsQueried := readClaimEventsFor(storePathResolved, sinceT, untilT, stderr)

	report := metrics.AggregateInterventions(callVerdicts, beats, claimEvents, claimEventsQueried, sinceT, untilT)

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "interventions: %s .. %s\n", report.Since, report.Until)
	for _, row := range report.Rows {
		if row.Count != nil {
			fmt.Fprintf(stdout, "  %-40s %-6d  (%s)\n", row.Name, *row.Count, row.Mechanism)
		} else {
			fmt.Fprintf(stdout, "  %-40s %s  (%s)\n", row.Name, row.Note, row.Mechanism)
		}
	}
	return 0
}
