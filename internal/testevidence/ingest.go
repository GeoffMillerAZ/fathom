package testevidence

// ingest.go — IngestTestEvidence (TC-2): writes loaded manifests and runs
// into the additive test_suites/test_runs/test_cases store tables,
// joining each run's cases to the service's Test* symbols on the way
// (TC-3, JoinCasesToSymbols) so TestCase.TestSymbolKey and the reported
// join rate are computed once, at ingest, rather than recomputed per
// read.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Report is IngestTestEvidence's return: what it wrote, plus every
// warning collected along the way (the caller's own LoadManifests/
// LoadEnvelopes warnings, folded in by the caller — see cmd/fathom's
// `tests ingest`) — never a silent drop.
type Report struct {
	SuitesWritten int
	RunsWritten   int
	CasesWritten  int
	CasesJoined   int
	// CasesUnmatched names (case ids, sorted) every case that joined no
	// test symbol, so a join miss is shown, never silent (TQ-28).
	CasesUnmatched []string
}

// IngestTestEvidence writes manifests (as declared test_suites rows,
// wholesale-replacing service's prior set — a fresh manifest load IS the
// current truth, design doc §2) and runs (as append-only test_runs/
// test_cases rows, idempotent by run digest) for service, stamped with
// watermark. Every run's cases are joined against the service's
// currently-indexed Test* symbols before being written.
//
// Suites and runs are independent writes: a caller with runs but no
// manifests (a plain `fathom ingest` picking up `[tests] results_dir`
// envelopes with no `--manifest` of its own — see IngestRuns) must never
// wholesale-replace a service's previously-declared suites with an empty
// set just because THIS call did not also load them.
func IngestTestEvidence(ctx context.Context, st store.Store, service, watermark string, manifests []Manifest, runs []Run) (Report, error) {
	var report Report

	suites := make([]store.TestSuite, 0, len(manifests))
	for _, m := range manifests {
		for _, su := range m.Suites {
			certifiesJSON := "[]"
			if len(su.Certifies) > 0 {
				b, err := json.Marshal(su.Certifies)
				if err != nil {
					return report, fmt.Errorf("marshal declared_certifies for %s/%s: %w", m.Component, su.Name, err)
				}
				certifiesJSON = string(b)
			}
			suites = append(suites, store.TestSuite{
				ServiceID: service, Component: m.Component, Suite: su.Name,
				Class: su.Class, Tier: su.Tier, Port: su.Port,
				DeclaredCertifiesJSON: certifiesJSON, Watermark: watermark,
			})
		}
	}
	if err := st.PutTestSuites(ctx, service, suites); err != nil {
		return report, fmt.Errorf("PutTestSuites: %w", err)
	}
	report.SuitesWritten = len(suites)

	runReport, err := IngestRuns(ctx, st, service, watermark, runs)
	if err != nil {
		return report, err
	}
	report.RunsWritten = runReport.RunsWritten
	report.CasesWritten = runReport.CasesWritten
	report.CasesJoined = runReport.CasesJoined
	report.CasesUnmatched = runReport.CasesUnmatched
	return report, nil
}

// IngestRuns writes runs (as append-only test_runs/test_cases rows,
// idempotent by run digest) for service, stamped with watermark, WITHOUT
// touching test_suites — the half of IngestTestEvidence a plain `fathom
// ingest` uses when `[tests] results_dir` is configured (cmd/fathom's
// cmdIngest): envelopes picked up incidentally on every ingest must never
// wipe out suites a separate `fathom tests ingest --manifest` call
// declared. Every run's cases are joined against the service's
// currently-indexed Test* symbols before being written.
func IngestRuns(ctx context.Context, st store.Store, service, watermark string, runs []Run) (Report, error) {
	var report Report
	if len(runs) == 0 {
		return report, nil
	}

	// The service's currently-indexed Test* symbols, for the join —
	// through the same suppression chokepoint every other content read
	// uses (QueryObservations).
	obs, err := st.QueryObservations(ctx, service, "")
	if err != nil {
		return report, fmt.Errorf("QueryObservations for join: %w", err)
	}
	symbols := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		symbols = append(symbols, o.ToSymbolNode())
	}

	storeRuns := make([]store.TestRun, 0, len(runs))
	for _, r := range runs {
		joined, stats := JoinCasesToSymbols(r.Cases, r.Suite.Name, symbols)
		report.CasesJoined += stats.Joined
		report.CasesUnmatched = append(report.CasesUnmatched, stats.Unmatched...)

		totalsJSON, err := json.Marshal(r.Totals)
		if err != nil {
			return report, fmt.Errorf("marshal totals for run %s: %w", r.Digest, err)
		}
		coverageJSON := ""
		if r.Coverage != nil {
			b, err := json.Marshal(r.Coverage)
			if err != nil {
				return report, fmt.Errorf("marshal coverage for run %s: %w", r.Digest, err)
			}
			coverageJSON = string(b)
		}

		cases := make([]store.TestCase, 0, len(joined))
		for _, jc := range joined {
			failureJSON := ""
			if jc.Failure != nil {
				b, err := json.Marshal(jc.Failure)
				if err != nil {
					return report, fmt.Errorf("marshal failure for case %s: %w", jc.ID, err)
				}
				failureJSON = string(b)
			}
			cases = append(cases, store.TestCase{
				CaseID: jc.ID, Name: jc.Name, Status: jc.Status, Attempts: jc.Attempts,
				DurationMS: parseDurationMS(jc.Duration), FailureJSON: failureJSON,
				TestSymbolKey: jc.TestSymbolKey,
			})
		}
		report.CasesWritten += len(cases)

		storeRuns = append(storeRuns, store.TestRun{
			ServiceID: service, RunDigest: r.Digest, Component: r.Suite.Component, Suite: r.Suite.Name,
			StartedAt: r.StartedAt, Verdict: r.Verdict, TotalsJSON: string(totalsJSON), CoverageJSON: coverageJSON,
			SignerTier: r.SignerTier, Watermark: watermark, Cases: cases,
			Commit: r.Commit, Dirty: r.Dirty, Race: r.Race,
		})
	}
	if err := st.PutTestRuns(ctx, service, storeRuns); err != nil {
		return report, fmt.Errorf("PutTestRuns: %w", err)
	}
	report.RunsWritten = len(storeRuns)

	return report, nil
}

// parseDurationMS best-effort parses a #Case.duration string (a Go-style
// duration, e.g. "1.2ms") into milliseconds; an unparseable value (a
// producer that wrote something else) stores as 0 rather than failing the
// whole ingest over one case's timing.
func parseDurationMS(s string) int64 {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d.Milliseconds()
}
