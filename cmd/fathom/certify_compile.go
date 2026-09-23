package main

// certify_compile.go — wires internal/certify.CompileCertifications into
// both `fathom tests ingest` and plain `fathom ingest` for a Go service
// (vision 33 / ADR-043 stage 1, TC-6's own wiring instruction): the
// certifications layer recompiles whenever declarations or runs might
// have moved, hash-gated so a repeat ingest against an unchanged
// declaration/run/watermark set is a cheap no-op rather than a full
// re-derive.
import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// runCertifyCompile hash-gates and, when needed, runs
// certify.CompileCertifications for service against st, stamped with
// watermark. Prints a one-line summary to stdout unless quiet is true (the
// JSON-output callers render their own shape instead). Errors are
// surfaced but never abort the caller's own ingest — certification
// compilation is a best-effort enrichment pass over data the ingest
// already wrote successfully.
func runCertifyCompile(ctx context.Context, st store.Store, service, watermark string, stdout, stderr io.Writer, quiet bool) {
	needed, err := certifyNeedsCompile(ctx, st, service, watermark)
	if err != nil {
		fmt.Fprintf(stderr, "warning: certify: hash-gate check: %v\n", err)
		return
	}
	if !needed {
		return
	}
	report, err := certify.CompileCertifications(ctx, st, service, watermark, time.Now())
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

// certifyNeedsCompile reports whether the certifications layer needs
// recompiling: false when nothing is declared at all (a compile would be
// a pure no-op), or when the layer has already been compiled at least
// once, every existing claim already carries the CURRENT watermark, and
// the declared-entry count has changed neither up nor down since — the
// cheap proxy for "the declarations, runs and watermark are unchanged"
// the wiring instruction asks for.
//
// Known gap (documented, not silently swallowed): a NEW attested run
// ingested at the SAME watermark (no commit between two `tests ingest`
// calls, e.g. a CI rerun) does not by itself change the declared-entry
// count, so this gate can skip a recompile that would have picked up the
// fresh run. `fathom tests ingest` always calls CompileCertifications
// unconditionally on its own runs argument for exactly this reason — see
// cmdTestsIngest — so the gap only affects a plain `fathom ingest`'s
// `[tests] results_dir` pickup between two ingests of the same commit.
// SweepCertifications (TC-8) closes this without a full recompile once a
// caller wires it in; not required by this wiring instruction.
func certifyNeedsCompile(ctx context.Context, st store.Store, service, watermark string) (bool, error) {
	decls, err := st.CertifyDeclarationsForService(ctx, service)
	if err != nil {
		return false, err
	}
	suites, err := st.TestSuiteStatuses(ctx, service)
	if err != nil {
		return false, err
	}
	declared := len(decls)
	for _, s := range suites {
		if s.DeclaredCertifiesJSON != "" && s.DeclaredCertifiesJSON != "[]" {
			declared++ // at least one suite-level entry exists; exact count is compile's own job
		}
	}
	if declared == 0 {
		return false, nil
	}

	existing, err := st.ClaimsForService(ctx, service, certify.Layer)
	if err != nil {
		return false, err
	}
	if len(existing) == 0 {
		return true, nil
	}
	for _, c := range existing {
		if c.CompiledAtWatermark != watermark {
			return true, nil
		}
	}
	if len(existing) != len(decls) {
		return true, nil
	}
	return false, nil
}
