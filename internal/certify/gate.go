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
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Step is what the post-ingest certifications pass did for one service:
// nothing, a full compile, a sweep of the certificates the layer already
// holds, or no sweep because the ingest changed nothing a sweep checks.
type Step int

const (
	StepNothing      Step = iota // nothing declared: the count read is the whole cost
	StepCompile                  // declarations or watermark moved: re-derive, then sweep existing claims
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
// A compile preserves existing test-file citations and always sweeps them
// afterward, so changed declarations or a changed watermark cannot hide a
// moved or vanished test. sweep gates only the already-compiled step.
// It is false for an ingest that read no file
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
	previous, err := st.ClaimsForService(ctx, service, Layer)
	if err != nil {
		return out, fmt.Errorf("compile citations: %w", err)
	}
	out.Report, err = compileCertifications(ctx, st, service, watermark, now, repoDir, ExecGitContext(ctx), previous)
	if err != nil {
		return out, fmt.Errorf("compile: %w", err)
	}
	if len(previous) > 0 {
		report, err := SweepCertifications(ctx, st, service, repoDir, ExecGitContext(ctx))
		if err != nil {
			return out, fmt.Errorf("sweep after compile: %w", err)
		}
		out.Report.Skipped = append(out.Report.Skipped, report.Skipped...)
		sort.Strings(out.Report.Skipped)
	}
	return out, nil
}

// nextStep reports what the certifications layer needs: StepNothing when
// nothing is declared at all (a compile would be a pure no-op, and no
// claim is read); StepSweep when every existing claim carries the current
// watermark and belongs to the declared set, and every unclaimed declaration
// is still unresolvable; StepCompile otherwise. Duplicate markers share a
// claim ID. Suite-only claims conservatively recompile because this gate
// does not expand suite declarations into test keys.
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
	declaredIDs := make(map[string]bool, len(decls))
	for _, d := range decls {
		declaredIDs[core.ClaimID(Layer, Kind, d.Kind+":"+d.TargetID, d.TestSymbolKey)] = true
	}
	claimed := make(map[string]bool, len(existing))
	for _, c := range existing {
		if c.CompiledAtWatermark != watermark {
			return StepCompile, nil
		}
		if !declaredIDs[c.ClaimID] {
			return StepCompile, nil
		}
		claimed[c.ClaimID] = true
	}
	if len(existing) == 0 && declared > len(decls) {
		return StepCompile, nil
	}
	// Only unclaimed declarations need resolution. SymbolSourcesForNames
	// uses symbol IN over the existing (service_id, symbol) index, without
	// ObservationsForSymbol's leading-wildcard GLOB. Require the full key
	// so the same test name in another package cannot resolve a declaration.
	unclaimedKeys := make(map[string]bool)
	nameSet := make(map[string]bool)
	for _, d := range decls {
		if claimed[core.ClaimID(Layer, Kind, d.Kind+":"+d.TargetID, d.TestSymbolKey)] {
			continue
		}
		unclaimedKeys[d.TestSymbolKey] = true
		name := d.TestSymbolKey[strings.LastIndex(d.TestSymbolKey, ".")+1:]
		nameSet[name] = true
	}
	if len(nameSet) == 0 {
		return StepSweep, nil
	}
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)
	rows, err := st.SymbolSourcesForNames(ctx, service, names)
	if err != nil {
		return StepNothing, err
	}
	for _, row := range rows {
		key := core.SymbolKey(service, core.QualifiedPackage(row.SourceRef, row.Package), row.Symbol)
		if unclaimedKeys[key] {
			return StepCompile, nil
		}
	}
	return StepSweep, nil
}
