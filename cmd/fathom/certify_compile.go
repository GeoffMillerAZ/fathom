package main

// certify_compile.go — wires internal/certify.CompileCertifications into
// both `fathom tests ingest` and plain `fathom ingest` for a Go service
// (vision 33 / ADR-043 stage 1, TC-6's own wiring instruction): the
// certifications layer recompiles whenever declarations or runs might
// have moved, hash-gated so a repeat ingest against an unchanged
// declaration/watermark set skips the full re-derive and runs
// internal/certify.SweepCertifications instead (TC-8), which re-checks
// every existing certificate against the index and runs just ingested.
import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// certifySweepRuns counts the sweeps runCertifyCompileGated starts, so a
// test can assert the gate through the CLI ingest entry point. The call
// itself stays direct (certify.AfterIngest calls
// certify.SweepCertifications): a function-valued seam would leave it
// reachable only through a heuristic edge (I57).
var certifySweepRuns int

// runCertifyCompileGated runs the post-ingest certifications step for
// service (certify.AfterIngest: nothing, a compile, a sweep, or a sweep
// skipped because sweep is false) against st, stamped with watermark, and
// reports it. The daemon's ingest runs the same step (pkg/client, O1271).
// Prints a one-line compile summary to stdout unless quiet is true (the
// JSON-output callers render their own shape instead). Errors are
// surfaced but never abort the caller's own ingest — certification
// compilation is a best-effort enrichment pass over data the ingest
// already wrote successfully.
//
// repoDir is the working tree the reach check runs `git diff` against
// (CV-5, O114), the same repo root the caller is already ingesting from.
// sweep is false for an ingest that read no file and picked up no new
// test run (O1270).
func runCertifyCompileGated(ctx context.Context, st store.Store, service, watermark, repoDir string, stdout, stderr io.Writer, quiet, sweep bool) {
	out, err := certify.AfterIngest(ctx, st, service, watermark, repoDir, time.Now(), sweep)
	if out.Step == certify.StepSweep {
		certifySweepRuns++
	}
	if err != nil {
		fmt.Fprintf(stderr, "warning: certify: %v\n", err)
		return
	}
	switch out.Step {
	case certify.StepNothing:
		return
	case certify.StepSweepSkipped:
		if !quiet {
			fmt.Fprintln(stdout, "certify: sweep skipped (no file read, no new test run)")
		}
		return
	case certify.StepCompile:
		if out.Report.Declared == 0 {
			return
		}
		if !quiet {
			fmt.Fprintf(stdout, "certify: %d declared, %d claim(s) compiled\n", out.Report.Declared, out.Report.ClaimsWritten)
		}
	}
	for _, w := range out.Report.Skipped {
		fmt.Fprintf(stderr, "warning: certify: %s\n", w)
	}
}
