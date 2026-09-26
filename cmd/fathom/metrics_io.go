package main

// metrics_io.go — the shared I/O helpers every `fathom metrics` verb
// (spend/traffic/channels/interventions/series/rollup) uses to read
// compile_runs, the call log, the hooks beat log, and claim_events into the
// shapes internal/metrics' pure aggregators need. Factored out once
// cmdMetricsRollupBuild (Q-21) and cmdMetricsSeries (Q-20) needed the exact
// same four reads the four report verbs already perform — one
// implementation, not five copies.

import (
	"context"
	"fmt"
	"io"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/metrics"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// readCompileRunsFor mounts storePath read-only and returns its
// compile_runs (optionally filtered to one module/layer). An empty
// storePath returns (nil, nil) — the honest "not measurable" path callers
// (cmdMetricsSpend indirectly via its own required-flag check;
// cmdMetricsRollupBuild, which treats --store as optional) already handle.
func readCompileRunsFor(storePath, service, module string) ([]core.CompileRun, error) {
	if storePath == "" {
		return nil, nil
	}
	mounted, mErr := store.MountReadOnly(storePath, store.LevelL0)
	if mErr != nil {
		return nil, fmt.Errorf("mount store %s: %w", storePath, mErr)
	}
	defer mounted.Close()
	return mounted.CompileRuns(context.Background(), service, module)
}

// spendBudgetLookup builds an internal/metrics.CompileRunBudgetLookup that
// resolves a layer's budget via semantic.LoadLayerIn(root, ...).
func spendBudgetLookup(root string) metrics.CompileRunBudgetLookup {
	return func(layer string) (float64, bool) {
		def, err := semantic.LoadLayerIn(root, layer)
		if err != nil {
			return 0, false
		}
		return def.Budgets.MaxUSDPerRun, true
	}
}

// readCallDataFor reads callLogPath (best-effort; an absent/unreadable log
// yields empty slices, never an error) and returns both shapes
// AggregateTraffic/AggregateInterventions need, scoped to [since, until]
// and (optionally) one service.
func readCallDataFor(callLogPath, service string, since, until time.Time) (entries []metrics.CallEntry, verdicts []metrics.CallVerdictOutcome) {
	rawEntries, _, _ := readCallLog(callLogPath)
	return callDataFrom(rawEntries, service, since, until)
}

// callDataFrom is readCallDataFor over entries already read (the console's
// Operate page reads the log once for every section it shows).
func callDataFrom(rawEntries []adoptionCallEntry, service string, since, until time.Time) (entries []metrics.CallEntry, verdicts []metrics.CallVerdictOutcome) {
	for _, e := range rawEntries {
		ts, ok := parseCallLogTS(e.TS)
		if !ok || ts.Before(since) || ts.After(until) {
			continue
		}
		if service != "" && e.Service != service {
			continue
		}
		entries = append(entries, metrics.CallEntry{
			TS: e.TS, Tool: e.Tool, Service: e.Service, Surface: e.Surface,
			LatencyMS: e.LatencyMS, Error: e.Error,
			Caller: string(console.ClassifyCaller(e.Tool, e.Surface, e.Origin)),
		})
		if e.VerdictOutcome != nil {
			verdicts = append(verdicts, metrics.CallVerdictOutcome{Tool: e.Tool, Outcome: e.VerdictOutcome})
		}
	}
	return entries, verdicts
}

// readBeatsFor reads hooksLogPath (best-effort — absent/unreadable yields
// nil, never an error).
func readBeatsFor(hooksLogPath string) []heartbeat.Beat {
	beats, _ := readBeatLog(hooksLogPath)
	return beats
}

// readClaimEventsFor mounts storePath read-only (when non-empty) and reads
// claim_events in [since, until]. queried is false exactly when storePath
// is empty — the caller (AggregateInterventions) needs this distinct from
// "queried a real store with zero results" (Q-6/Q-12's own honesty
// requirement). A mount/read error is reported to stderr and degrades to
// (nil, false) rather than aborting the whole command.
func readClaimEventsFor(storePath string, since, until time.Time, stderr io.Writer) (events []core.ClaimEvent, queried bool) {
	if storePath == "" {
		return nil, false
	}
	mounted, mErr := store.MountReadOnly(storePath, store.LevelL0)
	if mErr != nil {
		fmt.Fprintf(stderr, "warn: mount store %s: %v (review-findings-superseded row degrades to not-measurable)\n", storePath, mErr)
		return nil, false
	}
	defer mounted.Close()
	got, evErr := mounted.ClaimEventsSince(context.Background(), since, until)
	if evErr != nil {
		fmt.Fprintf(stderr, "warn: ClaimEventsSince: %v\n", evErr)
		return nil, false
	}
	return got, true
}
