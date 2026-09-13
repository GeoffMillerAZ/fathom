// RunOracle (SB-16) decides success after the last session, in the
// worktree, by the driver — never by the agent (vision 27 §2). It runs the
// subject's own test command once, separately, over fail-to-pass and
// pass-to-pass so each can be reported independently, with a coverage
// profile when the task asks for one, then runs every detector.
package scenariobench

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// defaultTestCommand is used when Subject.TestCommand is unset (types.go).
const defaultTestCommand = "go test -run '{{tests}}' ./..."

// goTestPassRE / goTestFailRE match `go test -v`'s per-test result lines:
// "--- PASS: TestName (0.00s)" / "--- FAIL: TestName (0.00s)".
var (
	goTestPassRE = regexp.MustCompile(`(?m)^\s*--- PASS:\s+(\S+)`)
	goTestFailRE = regexp.MustCompile(`(?m)^\s*--- FAIL:\s+(\S+)`)
)

// RunOracle runs task's oracle against wt: the test command over the union
// of fail-to-pass and pass-to-pass test names (with -coverprofile when
// Oracle.Coverage), decided per-test from `go test -v`'s own PASS/FAIL
// lines so FailToPassOK and PassToPassOK are independently meaningful, and
// then every detector. Output (test stdout, the coverage profile) is
// written under wt.CellDir and referenced from the result.
func RunOracle(ctx context.Context, wt Worktree, task Task) (OracleResult, error) {
	var result OracleResult

	names := append(append([]string(nil), task.Oracle.FailToPass...), task.Oracle.PassToPass...)
	if len(names) > 0 {
		passed, outputRef, coverRef, err := runOracleTests(ctx, wt, task, names)
		if err != nil {
			return OracleResult{}, fmt.Errorf("running oracle tests: %w", err)
		}
		result.TestOutputRef = outputRef
		result.CoverProfile = coverRef
		result.FailToPassOK = allPassed(passed, task.Oracle.FailToPass)
		result.PassToPassOK = allPassed(passed, task.Oracle.PassToPass)
	} else {
		// No named tests: nothing to fail, vacuously OK.
		result.FailToPassOK = true
		result.PassToPassOK = true
	}

	for _, d := range task.Oracle.Detectors {
		dr, err := runDetector(ctx, wt, d)
		if err != nil {
			return OracleResult{}, fmt.Errorf("running detector %s: %w", d.Name, err)
		}
		result.Detectors = append(result.Detectors, dr)
	}

	result.Passed = result.FailToPassOK && result.PassToPassOK && !anyViolated(result.Detectors)
	return result, nil
}

func allPassed(passed map[string]bool, names []string) bool {
	for _, n := range names {
		if !passed[n] {
			return false
		}
	}
	return true
}

func anyViolated(drs []DetectorResult) bool {
	for _, d := range drs {
		if d.Violated {
			return true
		}
	}
	return false
}

// runOracleTests runs the subject's test command once (-v appended for
// per-test parsing, plus -coverprofile when asked), over the regex-OR of
// names, in wt.Path.
func runOracleTests(ctx context.Context, wt Worktree, task Task, names []string) (passed map[string]bool, outputRef, coverRef string, err error) {
	tmpl := task.Subject.TestCommand
	if tmpl == "" {
		tmpl = defaultTestCommand
	}
	testsArg := strings.Join(names, "|")
	cmdStr := strings.ReplaceAll(tmpl, "{{tests}}", testsArg) + " -v"
	if task.Oracle.Coverage {
		coverRef = filepath.Join(wt.CellDir, "coverage.out")
		cmdStr += " -coverprofile=" + coverRef
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = wt.Path
	out, runErr := cmd.CombinedOutput() // a failing test set is a normal, expected exit != 0

	outputRef = filepath.Join(wt.CellDir, "oracle-test-output-"+shortHash(cmdStr)[:8]+".txt")
	if werr := os.WriteFile(outputRef, out, 0o644); werr != nil {
		return nil, "", "", fmt.Errorf("writing test output %s: %w", outputRef, werr)
	}

	passed = map[string]bool{}
	for _, m := range goTestPassRE.FindAllStringSubmatch(string(out), -1) {
		passed[m[1]] = true
	}
	for _, m := range goTestFailRE.FindAllStringSubmatch(string(out), -1) {
		passed[m[1]] = false
	}
	// A test command that ran but produced no per-test lines at all (a
	// non-Go test runner, or a build failure before any test ran) is
	// distinguishable from "every named test passed": every requested name
	// with no PASS/FAIL line stays unset (false via the map's zero value),
	// so allPassed correctly reports not-OK rather than vacuously OK.
	_ = runErr
	return passed, outputRef, coverRef, nil
}

func runDetector(ctx context.Context, wt Worktree, d Detector) (DetectorResult, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", d.Command)
	cmd.Dir = wt.Path
	out, err := cmd.CombinedOutput()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			return DetectorResult{}, fmt.Errorf("running %q: %w", d.Command, err)
		}
	}
	logPath := filepath.Join(wt.CellDir, "detector-"+sanitizeName(d.Name)+".log")
	_ = os.WriteFile(logPath, out, 0o644)
	return DetectorResult{Name: d.Name, Exit: exit, Violated: exit != d.ExpectExit}, nil
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}
