package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/testevidence"
)

// cmdTestsIngest implements "fathom tests ingest" (vision 33 / ADR-043
// stage 0, TC-4): loads a service's declared tests/*.cue manifests plus
// attested #TestResult envelopes and writes them into the additive
// test_suites/test_runs/test_cases/certify_declarations store tables
// (internal/testevidence.IngestTestEvidence, IngestCertifyMarkers).
// Store-direct, mirroring cmdTestsCoverage's own "works without the
// daemon" discipline.
//
//	fathom tests ingest --manifest <dir> --results <file|dir> [--service S] [--store DB]
//
// --manifest names the service's repo ROOT (its tests/*.cue live under
// <dir>/tests/, and its `// certifies:` markers are scanned across the
// whole of <dir>) — default ".", the common case of running from inside
// the service's own checkout. "no manifests found" and "no runs
// ingested" are named skips printed to stdout, never a silent clean
// table (design doc §2 / TC-4).
func cmdTestsIngest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("tests ingest", stderr)
	manifestDir := fs.String("manifest", ".", "service repo root: tests/*.cue live under <dir>/tests/, and `// certifies:` markers are scanned under <dir>")
	resultsPath := fs.String("results", "", "attested #TestResult envelope file, or a directory of *.json envelopes")
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name, the same guess the Claude Code hooks use)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: tests ingest requires --service (could not guess one from the current directory)")
			return 1
		}
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	ctx := context.Background()

	watermark := testsIngestWatermark(ctx, st, svc)

	manifests, manifestWarnings, err := testevidence.LoadManifests(*manifestDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: load manifests from %s: %v\n", *manifestDir, err)
		return 1
	}
	if len(manifests) == 0 {
		fmt.Fprintf(stdout, "tests ingest: no manifests found under %s/tests\n", *manifestDir)
	}
	for _, w := range manifestWarnings {
		fmt.Fprintf(stdout, "tests ingest: manifest warning: %s\n", w)
	}

	var runs []testevidence.Run
	if *resultsPath != "" {
		runs, err = testevidence.LoadEnvelopes(*resultsPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: load envelopes from %s: %v\n", *resultsPath, err)
			return 1
		}
	}
	if len(runs) == 0 {
		fmt.Fprintln(stdout, "tests ingest: no runs ingested")
	}

	report, err := testevidence.IngestTestEvidence(ctx, st, svc, watermark, manifests, runs)
	if err != nil {
		fmt.Fprintf(stderr, "error: ingest test evidence: %v\n", err)
		return 1
	}

	markerReport, err := testevidence.IngestCertifyMarkers(ctx, st, svc, watermark, *manifestDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: scan certifies markers: %v\n", err)
		return 1
	}
	for _, w := range markerReport.Warnings {
		fmt.Fprintf(stdout, "tests ingest: malformed certifies marker: %s\n", w)
	}

	fmt.Fprintf(stdout, "tests ingest: %d suite(s), %d run(s), %d case(s) (%d joined), %d declared certification(s)\n",
		report.SuitesWritten, report.RunsWritten, report.CasesWritten, report.CasesJoined, markerReport.Declared)

	// The certifications layer recompiles unconditionally at the end of
	// `tests ingest` (vision 33 / ADR-043 stage 1, TC-6's own wiring
	// instruction) — this verb's whole job is to pick up fresh
	// declarations and/or a fresh attested run, so it never skips the
	// recompile the way the hash-gated plain-`ingest` path
	// (runCertifyCompile, cmd/fathom/certify_compile.go) can.
	if certReport, cerr := certify.CompileCertifications(ctx, st, svc, watermark, time.Now()); cerr != nil {
		fmt.Fprintf(stderr, "warning: certify: compile: %v\n", cerr)
	} else if certReport.Declared > 0 {
		fmt.Fprintf(stdout, "certify: %d declared, %d claim(s) compiled\n", certReport.Declared, certReport.ClaimsWritten)
		for _, w := range certReport.Skipped {
			fmt.Fprintf(stderr, "warning: certify: %s\n", w)
		}
	}
	return 0
}

// testsIngestWatermark is the service watermark test-evidence rows are
// stamped with: the service's already-recorded IndexedCommit (set by a
// prior `fathom ingest`) when one exists, so test evidence can later be
// compared against the code state it was actually observed against;
// otherwise a process-time fallback, since test evidence may be ingested
// before the service has ever been code-indexed at all.
func testsIngestWatermark(ctx context.Context, st store.Store, service string) string {
	meta, found, err := st.GetServiceMeta(ctx, service)
	if err == nil && found && meta.IndexedCommit != "" {
		return meta.IndexedCommit
	}
	return fmt.Sprintf("tests-ingest:%d", time.Now().UnixNano())
}
