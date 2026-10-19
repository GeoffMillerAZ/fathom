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

// certifyStep is what the post-ingest certifications pass does for one
// service: nothing, a full compile, or a sweep of the certificates the
// layer already holds.
type certifyStep int

const (
	certifyNothing certifyStep = iota // nothing declared: the count read is the whole cost
	certifyCompile                    // declarations or watermark moved: re-derive the layer
	certifySweep                      // compiled at this watermark: re-check what exists
)

// certifySweepFn is the sweep runCertifyCompileGated calls; a package var
// so a test can count calls through the CLI ingest entry point.
var certifySweepFn = certify.SweepCertifications

// runCertifyCompileGated picks the step for service (certifyNextStep) and runs
// it against st, stamped with watermark: certify.CompileCertifications
// when the declarations or the watermark moved, certify.SweepCertifications
// when the layer is already compiled at this watermark, so a test that
// moved or went away, or a new attested run picked up from `[tests]
// results_dir` at the same commit, still reaches the certificates. Both
// run synchronously inside the caller's ingest; the sweep's git reads stop
// when ctx ends. Prints a
// one-line compile summary to stdout unless quiet is true (the
// JSON-output callers render their own shape instead). Errors are
// surfaced but never abort the caller's own ingest — certification
// compilation is a best-effort enrichment pass over data the ingest
// already wrote successfully.
//
// repoDir is CV-5's own addition (design doc §1.1, O114): the working
// tree evaluateCertificate's reach check runs `git diff` against, using
// the real system git binary (certify.ExecGit) — the same repo root the
// caller is already ingesting from.
//
// sweep gates the sweep step. It is
// false for an ingest that read no file and picked up no new test run,
// where nothing a sweep checks (a test or reached symbol moved or gone, a
// new attested run) can have changed.
func runCertifyCompileGated(ctx context.Context, st store.Store, service, watermark, repoDir string, stdout, stderr io.Writer, quiet, sweep bool) {
	step, err := certifyNextStep(ctx, st, service, watermark)
	if err != nil {
		fmt.Fprintf(stderr, "warning: certify: hash-gate check: %v\n", err)
		return
	}
	switch step {
	case certifyNothing:
		return
	case certifySweep:
		if !sweep {
			if !quiet {
				fmt.Fprintln(stdout, "certify: sweep skipped (no file read, no new test run)")
			}
			return
		}
		report, err := certifySweepFn(ctx, st, service, repoDir, certify.ExecGitContext(ctx))
		if err != nil {
			fmt.Fprintf(stderr, "warning: certify: sweep: %v\n", err)
			return
		}
		for _, w := range report.Skipped {
			fmt.Fprintf(stderr, "warning: certify: %s\n", w)
		}
		return
	}
	report, err := certify.CompileCertifications(ctx, st, service, watermark, time.Now(), repoDir, certify.ExecGit)
	if err != nil {
		fmt.Fprintf(stderr, "warning: certify: compile: %v\n", err)
		return
	}
	if report.Declared == 0 {
		return
	}
	if !quiet {
		fmt.Fprintf(stdout, "certify: %d declared, %d claim(s) compiled\n", report.Declared, report.ClaimsWritten)
	}
	for _, w := range report.Skipped {
		fmt.Fprintf(stderr, "warning: certify: %s\n", w)
	}
}

// certifyNextStep reports what the certifications layer needs:
// certifyNothing when nothing is declared at all (a compile would be a
// pure no-op, and no claim is read); certifySweep when the layer has
// already been compiled at least once, every existing claim already
// carries the CURRENT watermark, and the declared-entry count has changed
// neither up nor down since — the cheap proxy for "the declarations and
// watermark are unchanged"; certifyCompile otherwise.
//
// A NEW attested run ingested at the SAME watermark (no commit between
// two ingests, e.g. a CI rerun picked up from `[tests] results_dir`) does
// not change the declared-entry count, so this gate skips the recompile;
// the sweep it hands over to re-derives each certificate's state from the
// runs now in the store, so the fresh run is not missed.
func certifyNextStep(ctx context.Context, st store.Store, service, watermark string) (certifyStep, error) {
	decls, err := st.CertifyDeclarationsForService(ctx, service)
	if err != nil {
		return certifyNothing, err
	}
	suites, err := st.TestSuiteStatuses(ctx, service)
	if err != nil {
		return certifyNothing, err
	}
	declared := len(decls)
	for _, s := range suites {
		if s.DeclaredCertifiesJSON != "" && s.DeclaredCertifiesJSON != "[]" {
			declared++ // at least one suite-level entry exists; exact count is compile's own job
		}
	}
	if declared == 0 {
		return certifyNothing, nil
	}

	existing, err := st.ClaimsForService(ctx, service, certify.Layer)
	if err != nil {
		return certifyNothing, err
	}
	if len(existing) == 0 {
		return certifyCompile, nil
	}
	for _, c := range existing {
		if c.CompiledAtWatermark != watermark {
			return certifyCompile, nil
		}
	}
	if len(existing) != len(decls) {
		return certifyCompile, nil
	}
	return certifySweep, nil
}
