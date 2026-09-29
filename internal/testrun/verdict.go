package testrun

// Verdict is the three-way result of a test run (design section 3).
type Verdict string

// The three results. Unknown is the honest answer whenever the output
// cannot prove a pass and shows no failure.
const (
	Pass    Verdict = "pass"
	Fail    Verdict = "fail"
	Unknown Verdict = "unknown"
)

// Evidence is what the hook knows about a run besides its output text.
type Evidence struct {
	// ExitCode is the command's exit status, set only when the hook input
	// carries it (Claude Code: PostToolUseFailure's leading "Exit code N",
	// TR-V1). nil on PostToolUse, which never carries one.
	ExitCode *int
	// Filtered: the output went through a pipe (`| tail -30`), so the text
	// is a filtered view and the shell's exit status is the filter's.
	Filtered bool
	// Truncated: the harness cut the output (persisted the rest elsewhere).
	Truncated bool
}

// Result is a parsed run and its verdict.
type Result struct {
	Run     Run
	Verdict Verdict
}

// Analyze is tier 0 end to end: the parse and the three-way verdict.
func Analyze(output string, ev Evidence) Result {
	r := ParseTestRun(output)
	return Result{Run: r, Verdict: testRunVerdict(r, ev)}
}

// testRunVerdict is the three-way result (TR-4):
//
//   - fail: the output shows a failure (a failing test, a panic, a build
//     error, a failed package, a JUnit failure or error), or the hook input
//     carries a non-zero exit status of an unfiltered command;
//   - pass: no failure, a recognised runner, an unfiltered output, and
//     either the runner's final summary covers every package the output
//     shows in an output the harness did not cut, or the hook input
//     carries exit status 0;
//   - unknown: everything else.
func testRunVerdict(r Run, ev Evidence) Verdict {
	if r.Failed() {
		return Fail
	}
	if ev.ExitCode != nil && *ev.ExitCode != 0 && !ev.Filtered {
		return Fail
	}
	if r.Runner == RunnerNone || r.Runner == "" || ev.Filtered {
		return Unknown
	}
	if ev.ExitCode != nil && *ev.ExitCode == 0 {
		return Pass
	}
	if r.Complete && !ev.Truncated {
		return Pass
	}
	return Unknown
}
