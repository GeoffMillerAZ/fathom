package scenariobench

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goModuleFixture writes a tiny, self-contained Go module with two tests —
// TestPass (always passes) and TestFail (always fails) — so RunOracle can
// be exercised against a real `go test` run with no network and no other
// dependency on this repo's own module.
func goModuleFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module scenariooraclefixture\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(dir, "thing.go"), "package thing\n\nfunc Add(a, b int) int { return a + b }\n")
	writeTestFile(t, filepath.Join(dir, "thing_test.go"), `package thing

import "testing"

func TestPass(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Fatal("bad")
	}
}

func TestFail(t *testing.T) {
	t.Fatal("this test always fails")
}
`)
	return dir
}

func TestRunOracle_AllPass(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{FailToPass: []string{"TestPass"}, PassToPass: []string{"TestPass"}}}

	result, err := RunOracle(context.Background(), wt, task)
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if !result.FailToPassOK {
		t.Error("expected FailToPassOK=true")
	}
	if !result.PassToPassOK {
		t.Error("expected PassToPassOK=true")
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if result.TestOutputRef == "" {
		t.Error("expected a TestOutputRef")
	}
	raw, err := os.ReadFile(result.TestOutputRef)
	if err != nil || !strings.Contains(string(raw), "PASS: TestPass") {
		t.Errorf("expected the output file to record TestPass's PASS line: err=%v content=%q", err, raw)
	}
}

func TestRunOracle_FailToPassFails(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{FailToPass: []string{"TestFail"}, PassToPass: []string{"TestPass"}}}

	result, err := RunOracle(context.Background(), wt, task)
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if result.FailToPassOK {
		t.Error("expected FailToPassOK=false (TestFail always fails)")
	}
	if !result.PassToPassOK {
		t.Error("expected PassToPassOK=true (TestPass still passes)")
	}
	if result.Passed {
		t.Error("expected Passed=false")
	}
}

func TestRunOracle_Coverage(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{FailToPass: []string{"TestPass"}, Coverage: true}}

	result, err := RunOracle(context.Background(), wt, task)
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if result.CoverProfile == "" {
		t.Fatal("expected a CoverProfile path")
	}
	if _, err := os.Stat(result.CoverProfile); err != nil {
		t.Errorf("expected the coverage profile to exist: %v", err)
	}
}

func TestRunOracle_Detectors(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}

	t.Run("expected exit satisfied", func(t *testing.T) {
		task := Task{Oracle: Oracle{Detectors: []Detector{{Name: "file-exists", Command: "test -f thing.go", ExpectExit: 0}}}}
		result, err := RunOracle(context.Background(), wt, task)
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if len(result.Detectors) != 1 || result.Detectors[0].Violated {
			t.Errorf("expected a satisfied, non-violated detector: %+v", result.Detectors)
		}
		if !result.Passed {
			t.Error("expected Passed=true (no named tests, no violation)")
		}
	})

	t.Run("violated detector fails the oracle", func(t *testing.T) {
		task := Task{Oracle: Oracle{Detectors: []Detector{{Name: "no-such-file", Command: "test -f does-not-exist.go", ExpectExit: 0}}}}
		result, err := RunOracle(context.Background(), wt, task)
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if len(result.Detectors) != 1 || !result.Detectors[0].Violated {
			t.Errorf("expected a violated detector: %+v", result.Detectors)
		}
		if result.Passed {
			t.Error("expected Passed=false when a detector is violated")
		}
	})
}

func TestRunOracle_NoNamedTestsIsVacuouslyOK(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{}}
	result, err := RunOracle(context.Background(), wt, task)
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if !result.FailToPassOK || !result.PassToPassOK || !result.Passed {
		t.Errorf("expected a vacuous pass with no named tests and no detectors: %+v", result)
	}
}
