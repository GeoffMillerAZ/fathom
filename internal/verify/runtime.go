package verify

// runtime.go — runtime_pass (TC-29; I8 and ADR-023 as amended by ADR-067,
// 2026-10-10). "Static" means verify never executes a test or a command;
// it may read attested evidence. runtime_pass reads the newest attested
// run in test_runs (a test-kit envelope `fathom tests ingest` wrote,
// internal/testevidence) for a row's certifying tests: Pass or Fail with
// the run's commit and age, Unknown only when no run exists. It is
// informational: it never changes a row's status, never names a gate
// failure and never becomes a SARIF refusal, until a reading shows that
// attested runs agree with local reruns.

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const runtimeCriterion = "runtime_pass"

// AttestedRun is the newest attested run of one certifying test: a fact a
// producer outside the index attested and fathom ingested, never a run
// verify started.
type AttestedRun struct {
	// Test is the test the run executed, as the evidence names it (a
	// canonical key or a bare name).
	Test string
	// Status is the case's own status: "pass" | "fail" | "skip" | "flake".
	Status   string
	Attempts int
	// Commit is the commit the run was attested against; "" when the
	// envelope bound it to none. Dirty is true when the working tree
	// carried uncommitted edits at that moment.
	Commit string
	Dirty  bool
	// At is when the run started (its envelope's timing.startedAt), else
	// when it was ingested; zero when neither is known.
	At time.Time
	// Run is the run's digest, Suite the suite it belongs to.
	Run   string
	Suite string
}

// RuntimeEvidence is an optional capability: an Evidence that can read the
// attested runs in test_runs, read-only. VerifyPlan type-asserts for it;
// an Evidence without it leaves runtime_pass Unknown with that reason,
// never a fabricated verdict.
type RuntimeEvidence interface {
	// NewestAttestedRuns returns, for each of tests (bare names or
	// canonical keys) that has an attested run, the newest one, keyed by
	// the name as given. A name with no run is absent. An error means
	// the read itself failed, which runtime_pass names (I38).
	NewestAttestedRuns(tests []string) (map[string]AttestedRun, error)
	// Now is the clock a run's age is read against: injected, so
	// VerifyPlan itself never reads one.
	Now() time.Time
}

// runtimeTests names a row's certifying tests and where they came from:
// the tests certified_by names, else the reachable_from entry when it is
// a test, else the tests that statically reach the row's symbol.
func runtimeTests(e Expect, reaching []string) ([]string, string) {
	switch {
	case len(e.CertifiedBy) > 0:
		return e.CertifiedBy, "certified_by"
	case isTestName(e.ReachableFrom):
		return []string{e.ReachableFrom}, "reachable_from"
	case len(reaching) > 0:
		return reaching, "the tests that statically reach it"
	}
	return nil, ""
}

// isTestName reports whether name (bare, qualified or a canonical key)
// names a Test* function.
func isTestName(name string) bool {
	return strings.HasPrefix(bareTestName(name), "Test")
}

// bareTestName reduces a canonical key ("svc::dir/pkg.TestX") or a
// qualified name ("pkg.TestX") to the test's bare name.
func bareTestName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		name = name[i+2:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// evalRuntimePass is runtime_pass: it reads the newest attested run of
// tests (runtimeTests) through ev's RuntimeEvidence capability. Pass for a
// run that passed (a flake passed on a retry), Fail for one that failed,
// Unknown when no run exists, the newest run skipped the test, or ev reads
// no attested runs. The caller appends it without touching the row's
// status (I8).
func evalRuntimePass(tests []string, from string, ev Evidence) Criterion {
	re, ok := ev.(RuntimeEvidence)
	if !ok {
		return Criterion{Name: runtimeCriterion, Status: Unknown,
			Detail: "no attested-run evidence on this evidence source: verify never runs a test"}
	}
	if len(tests) == 0 {
		return Criterion{Name: runtimeCriterion, Status: Unknown,
			Detail: "no certifying test: the row names none (certified_by, a Test* reachable_from) and no Test* statically reaches it"}
	}
	runs, err := re.NewestAttestedRuns(tests)
	if err != nil {
		return Criterion{Name: runtimeCriterion, Status: Unknown, Detail: "the attested-run read failed: " + err.Error()}
	}
	if len(runs) == 0 {
		names := make([]string, len(tests))
		for i, t := range tests {
			names[i] = bareTestName(t)
		}
		return Criterion{Name: runtimeCriterion, Status: Unknown, Detail: fmt.Sprintf(
			"no attested run in test_runs for %s (%s); ingest a test-kit envelope with `fathom tests ingest --results`",
			strings.Join(dedupSorted(names), ", "), from)}
	}
	newest := newestRun(runs)
	name := bareTestName(newest.Test)
	where := runWhere(newest, re.Now())
	switch newest.Status {
	case "pass":
		return Criterion{Name: runtimeCriterion, Status: Pass, Detail: fmt.Sprintf("%s passed in its newest attested run %s", name, where)}
	case "flake":
		return Criterion{Name: runtimeCriterion, Status: Pass, Detail: fmt.Sprintf("%s passed after %d attempts (flake) in its newest attested run %s", name, newest.Attempts, where)}
	case "fail":
		return Criterion{Name: runtimeCriterion, Status: Fail, Detail: fmt.Sprintf("%s failed in its newest attested run %s; informational, the row's status is unchanged (I8)", name, where)}
	}
	return Criterion{Name: runtimeCriterion, Status: Unknown, Detail: fmt.Sprintf("%s's newest attested run recorded %q, not a pass or a fail %s", name, newest.Status, where)}
}

// newestRun picks the newest of runs; a tie prefers a failure, then the
// first test name, so the answer is deterministic and never hides a fail.
func newestRun(runs map[string]AttestedRun) AttestedRun {
	keys := make([]string, 0, len(runs))
	for k := range runs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	best := runs[keys[0]]
	for _, k := range keys[1:] {
		r := runs[k]
		switch {
		case r.At.After(best.At):
			best = r
		case r.At.Equal(best.At) && r.Status == "fail" && best.Status != "fail":
			best = r
		}
	}
	return best
}

// runWhere renders a run's binding: its digest and suite, its commit (or
// that it was bound to none) and its age against now.
func runWhere(r AttestedRun, now time.Time) string {
	var parts []string
	if r.Run != "" {
		id := strings.TrimPrefix(r.Run, "sha256:")
		if len(id) > 12 {
			id = id[:12]
		}
		run := "run " + id
		if r.Suite != "" {
			run += ", suite " + r.Suite
		}
		parts = append(parts, run)
	}
	switch {
	case r.Commit == "":
		parts = append(parts, "unbound to a commit")
	case r.Dirty:
		parts = append(parts, "at commit "+shortCommit(r.Commit)+" (dirty tree)")
	default:
		parts = append(parts, "at commit "+shortCommit(r.Commit))
	}
	if r.At.IsZero() {
		parts = append(parts, "age unknown")
	} else {
		parts = append(parts, ageText(now.Sub(r.At))+" old")
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func shortCommit(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}

// ageText renders d coarsely: days and hours, hours and minutes, or
// minutes. A run stamped after now (a clock skew) reads as under a minute.
func ageText(d time.Duration) string {
	if d < time.Minute {
		return "under a minute"
	}
	days, hours, mins := int(d/(24*time.Hour)), int(d%(24*time.Hour)/time.Hour), int(d%time.Hour/time.Minute)
	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	case hours > 0 && mins > 0:
		return fmt.Sprintf("%dh%dm", hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", mins)
}

func dedupSorted(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	n := 0
	for i, s := range out {
		if i == 0 || s != out[n-1] {
			out[n] = s
			n++
		}
	}
	return out[:n]
}

// informational reports whether a criterion never decides a row: its
// status is never the gate's reason and never a SARIF refusal.
// runtime_pass (I8) and self_declared (I45) are both reported and never
// gate.
func informational(name string) bool {
	return name == runtimeCriterion || name == "self_declared"
}
