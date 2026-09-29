package testrun

import "testing"

func exit(n int) *int { return &n }

// TestTestRunVerdictUnknownWithoutFullSummary pins TR-4 (design section
// 3): the result is three-way. Pass only when the runner's own final
// summary covers every package the output shows and the output is the
// whole run (not piped through a filter, not cut by the harness);
// otherwise unknown — a hook reading stdin never sees the exit code on
// PostToolUse (TR-V1). The exit code is used only when the hook input
// carries it (PostToolUseFailure's "Exit code N").
func TestTestRunVerdictUnknownWithoutFullSummary(t *testing.T) {
	pass := fixture(t, "go-plain-pass.txt")
	passJSON := fixture(t, "go-json-pass.jsonl")
	fail := fixture(t, "go-plain-fail.txt")
	lines := splitLines(passJSON)
	cutJSON := joinLines(lines[:len(lines)-3])
	running := "=== RUN   TestSlow\n    slow_test.go:9: still going\n"

	cases := []struct {
		name   string
		output string
		ev     Evidence
		want   Verdict
	}{
		{"a whole passing run", pass, Evidence{}, Pass},
		{"a whole passing -json run", passJSON, Evidence{}, Pass},
		{"a passing run seen through a pipe", pass, Evidence{Filtered: true}, Unknown},
		{"a passing run the harness cut", pass, Evidence{Truncated: true}, Unknown},
		{"a -json stream missing a package's final event", cutJSON, Evidence{}, Unknown},
		{"output that stops mid-test", running, Evidence{}, Unknown},
		{"output no parser recognises", "3 passed, 0 failed\n", Evidence{}, Unknown},
		{"failures", fail, Evidence{}, Fail},
		{"failures seen through a pipe", fail, Evidence{Filtered: true}, Fail},
		{"a carried non-zero exit with nothing parsed", "error: something went wrong\n", Evidence{ExitCode: exit(2)}, Fail},
		{"a carried non-zero exit on a pipe is the filter's", "3 passed\n", Evidence{ExitCode: exit(1), Filtered: true}, Unknown},
		{"a carried zero exit proves a cut run passed", running, Evidence{ExitCode: exit(0)}, Pass},
		{"a carried zero exit on a pipe proves nothing", running, Evidence{ExitCode: exit(0), Filtered: true}, Unknown},
		{"a carried zero exit on unrecognised output", "3 passed\n", Evidence{ExitCode: exit(0)}, Unknown},
		{"failures beat a carried zero exit", fail, Evidence{ExitCode: exit(0)}, Fail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := testRunVerdict(ParseTestRun(tc.output), tc.ev); got != tc.want {
				t.Errorf("verdict = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("Analyze is the parse plus the verdict", func(t *testing.T) {
		res := Analyze(fail, Evidence{})
		if res.Verdict != Fail || res.Run.Runner != RunnerGo || len(res.Run.Failures) != 5 {
			t.Errorf("Analyze = %q, %q, %d failures", res.Verdict, res.Run.Runner, len(res.Run.Failures))
		}
	})
}
