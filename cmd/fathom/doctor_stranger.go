// doctor_stranger.go — SR-53 (docs/plans/surface-reckoning.md, design §5
// row SR-53, F-51, G16, docs/design/reach.md §5 "The stranger's five
// minutes"): `fathom doctor --stranger` times the SAME "install to first
// proven answer" path scripts/stranger-check.sh already exercises (clone
// the tracked tree to scratch, build, `fathom init`, ingest a fixture,
// verify a two-row plan) against G16's 5-minute bar, folded into a native
// doctor row instead of a shell script nobody's `fathom doctor` run
// exercises or gates on. `doorStrangerCheck` itself is a pure,
// clock-injected verdict over caller-supplied steps (never a bare
// time.Now() — the determinism contract, internal/AGENTS.md) so the
// pass/fail boundary is provable without a single real timed operation;
// doctorStrangerRow supplies the real step, invoked via exec.CommandContext
// (contexted, never a bare exec.Command — the tenancy-lint discipline
// every process spawn in this codebase keeps) and bounded well past the
// bar so a genuinely hung script still reports FAIL rather than blocking
// doctor forever.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// strangerCheckBar is G16's own bar (docs/design/reach.md §3 item 3:
// "install to first proven answer in at most 5 minutes").
const strangerCheckBar = 5 * time.Minute

// strangerCheckTimeout bounds the real script's subprocess — generous
// past the bar itself, so a run that is merely SLOW still completes and
// reports an honest over-bar FAIL, while a genuinely hung script is cut
// off rather than blocking `fathom doctor --stranger` forever.
const strangerCheckTimeout = 10 * time.Minute

// strangerStep is one named, timed leg of the stranger's path — Run does
// the real work; doctorStrangerCheck itself performs no I/O, so the
// caller's step is what actually builds/ingests/verifies.
type strangerStep struct {
	Name string
	Run  func() error
}

// doctorStrangerCheck is SR-53's anchor (F-51): runs steps in order,
// timed by the injected clock now (called once before the first step and
// again either on the first failing step or after the last one — never
// per-step, so a passing multi-step run costs exactly two clock reads).
// FAIL names the first failing step, its error, and the elapsed time so
// far — never claiming the bar was met when a step never finished. With
// every step passing, FAIL names the elapsed time against bar when it is
// exceeded (a strict >, so a run finishing at EXACTLY the bar passes);
// otherwise PASS, naming the elapsed time.
func doctorStrangerCheck(steps []strangerStep, bar time.Duration, now func() time.Time) doctorCheck {
	const name = "stranger"
	start := now()
	for _, step := range steps {
		if err := step.Run(); err != nil {
			elapsed := now().Sub(start)
			return doctorCheck{
				Name: name, Status: doctorFail,
				Reason: fmt.Sprintf("stranger check failed at %q after %s: %v", step.Name, elapsed.Round(time.Second), err),
				Remedy: "scripts/stranger-check.sh (run directly for the full per-step timing breakdown)",
			}
		}
	}
	elapsed := now().Sub(start)
	if elapsed > bar {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("install to first proven answer took %s, over the %s bar (G16)", elapsed.Round(time.Second), bar),
			Remedy: "scripts/stranger-check.sh (run directly for the full per-step timing breakdown; profile the slow leg)",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("install to first proven answer in %s (bar: %s, G16)", elapsed.Round(time.Second), bar)}
}

// doctorStrangerRow builds and runs the real stranger check against root.
// Skipped, never failed, when root carries no scripts/stranger-check.sh
// (C4-5, this file's package doc comment: an uncheckable state is
// reported skipped with a reason, never a guessed pass or an unfair
// fail) — --stranger is a fathom-development check timing fathom's OWN
// install path; an adopting repo has no fauxtown fixture or
// stranger-check.sh of its own for this row to time.
func doctorStrangerRow(root string) doctorCheck {
	const name = "stranger"
	script := filepath.Join(root, "scripts", "stranger-check.sh")
	if fi, err := os.Stat(script); err != nil || fi.IsDir() {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: "no scripts/stranger-check.sh at this root — --stranger only applies to fathom's own repository",
		}
	}
	steps := []strangerStep{
		{Name: "stranger-check.sh", Run: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), strangerCheckTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, script)
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%v: %s", err, truncateBytes(string(out), 2000))
			}
			return nil
		}},
	}
	return doctorStrangerCheck(steps, strangerCheckBar, time.Now)
}
