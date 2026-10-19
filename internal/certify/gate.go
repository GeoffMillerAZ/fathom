package certify

// gate.go — the post-ingest certifications step (TC-8, O1270, O1271):
// whether a service's certifications layer needs nothing, a compile or a
// sweep after an ingest, and running that step. Both ingest entry points
// call AfterIngest, the CLI's plain `fathom ingest`
// (cmd/fathom/certify_compile.go) and the daemon's (pkg/client
// IngestService and IngestServiceAt), so the choice lives in one place.
import (
	"context"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// Step is what the post-ingest certifications pass did for one service:
// nothing, a full compile, a sweep of the certificates the layer already
// holds, or no sweep because the ingest changed nothing a sweep checks.
type Step int

const (
	StepNothing      Step = iota // nothing declared: the count read is the whole cost
	StepCompile                  // declarations or watermark moved: re-derive the layer
	StepSweep                    // compiled at this watermark: re-check what exists
	StepSweepSkipped             // compiled at this watermark, and the ingest read no file and picked up no run
)

// Outcome is AfterIngest's return: the step taken and what it wrote.
type Outcome struct {
	Step   Step
	Report Report
}

// AfterIngest picks the step for service (nextStep) and runs it against
// st, stamped with watermark: CompileCertifications when the declarations
// or the watermark moved, SweepCertifications when the layer is already
// compiled at this watermark, so a test that moved or went away, or a new
// attested run picked up at the same commit, still reaches the
// certificates. It runs synchronously inside the caller's ingest; the
// sweep's git reads stop when ctx ends. now stamps a compile.
//
// repoDir is the working tree the reach check runs `git diff` against
// (CV-5, design doc §1.1, O114), the root the caller is ingesting from.
//
// sweep gates the sweep step. It is false for an ingest that read no file
// and picked up no new test run, where nothing a sweep checks (a test or
// reached symbol moved or gone, a new attested run) can have changed; the
// Outcome then says StepSweepSkipped. Outcome.Step is set before the step
// runs, so a caller can count a sweep that then failed.
func AfterIngest(ctx context.Context, st store.Store, service, watermark, repoDir string, now time.Time, sweep bool) (Outcome, error) {
	step, err := nextStep(ctx, st, service, watermark)
	if err != nil {
		return Outcome{}, fmt.Errorf("hash-gate check: %w", err)
	}
	out := Outcome{Step: step}
	switch step {
	case StepNothing:
		return out, nil
	case StepSweep:
		if !sweep {
			out.Step = StepSweepSkipped
			return out, nil
		}
		out.Report, err = SweepCertifications(ctx, st, service, repoDir, ExecGitContext(ctx))
		if err != nil {
			return out, fmt.Errorf("sweep: %w", err)
		}
		return out, nil
	}
	out.Report, err = CompileCertifications(ctx, st, service, watermark, now, repoDir, ExecGit)
	if err != nil {
		return out, fmt.Errorf("compile: %w", err)
	}
	return out, nil
}

// nextStep reports what the certifications layer needs: StepNothing when
// nothing is declared at all (a compile would be a pure no-op, and no
// claim is read); StepSweep when the layer has already been compiled at
// least once, every existing claim already carries the CURRENT watermark,
// and the declared-entry count has changed neither up nor down since —
// the cheap proxy for "the declarations and watermark are unchanged";
// StepCompile otherwise.
//
// A NEW attested run ingested at the SAME watermark (no commit between
// two ingests, e.g. a CI rerun picked up from `[tests] results_dir`) does
// not change the declared-entry count, so this gate skips the recompile;
// the sweep it hands over to re-derives each certificate's state from the
// runs now in the store, so the fresh run is not missed.
func nextStep(ctx context.Context, st store.Store, service, watermark string) (Step, error) {
	decls, err := st.CertifyDeclarationsForService(ctx, service)
	if err != nil {
		return StepNothing, err
	}
	suites, err := st.TestSuiteStatuses(ctx, service)
	if err != nil {
		return StepNothing, err
	}
	declared := len(decls)
	for _, s := range suites {
		if s.DeclaredCertifiesJSON != "" && s.DeclaredCertifiesJSON != "[]" {
			declared++ // at least one suite-level entry exists; exact count is compile's own job
		}
	}
	if declared == 0 {
		return StepNothing, nil
	}

	existing, err := st.ClaimsForService(ctx, service, Layer)
	if err != nil {
		return StepNothing, err
	}
	if len(existing) == 0 {
		return StepCompile, nil
	}
	for _, c := range existing {
		if c.CompiledAtWatermark != watermark {
			return StepCompile, nil
		}
	}
	if len(existing) != len(decls) {
		return StepCompile, nil
	}
	return StepSweep, nil
}
