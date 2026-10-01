// guardsfathom_probe_test.go — wave N14 item 2's mechanical proof that
// testdata/scenariobench/guards-fathom.json's new
// "debug-dump-default-under-workspace" detector actually discriminates,
// the same shape internal/scenariobench/guardsm1's guardsm1_probe_test.go
// already proves for guards-m1.json (RC-50): guards-pilot-6's second
// reading found that the task's ORIGINAL oracle — reusing the pre-existing
// TestDefaultStorePathNeverTheCwd verbatim — never exercises a brand-new
// verb's own default path at all, so two real sessions hardcoded
// ~/.fathom/debug unconditionally and both scored PASS. This file applies
// the corpus's own violation_probe (the naive, hardcoded-home fix) to a
// disposable copy of the subject tree and checks the new detector goes red,
// then applies a compliant fix (routes through the shared workspacePath
// helper, cmd/fathom/defaultpaths.go) and checks the same detector goes
// green.
package scenariobench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGuardsI31DebugDumpDetectorFiresAndClears is wave N14 item 2's
// anchor. It uses the archiveHEAD/applyCommands/runProbeDetector/
// taskDetector helpers at the end of this file (the guardsm1 package keeps
// its own copies) rather than reimplementing them per test.
func TestGuardsI31DebugDumpDetectorFiresAndClears(t *testing.T) {
	c := loadGuardsCorpus(t) // guards-fathom.json
	var task Task
	found := false
	for _, tk := range c.Tasks {
		if tk.ID == "guards-i31-workspace-artifact" {
			task, found = tk, true
			break
		}
	}
	if !found {
		t.Fatal("guards-i31-workspace-artifact not found in guards-fathom.json")
	}
	if task.ViolationProbe == nil || len(task.ViolationProbe.Commands) == 0 {
		t.Fatal("task guards-i31-workspace-artifact: no violation_probe")
	}
	detector := taskDetector(t, task, "debug-dump-default-under-workspace")

	dir := archiveHEAD(t, moduleRoot)

	// The naive, VIOLATING fix from the corpus's own violation_probe:
	// hardcodes ~/.fathom/debug unconditionally, exactly like the two real
	// guards-pilot-6 sessions the second reading found.
	applyCommands(t, dir, task.ViolationProbe.Commands)
	if exit, out := runProbeDetector(t, dir, detector.Command); exit == 0 {
		t.Fatalf("detector %q must be RED on the naive hardcoded-~/.fathom/debug fix (%s):\n%s", detector.Name, task.ViolationProbe.Description, out)
	} else {
		t.Logf("%s: detector %q correctly red (exit %d) on the naive violation", task.ID, detector.Name, exit)
	}

	// The compliant fix: route the SAME default path through workspacePath
	// (cmd/fathom/defaultpaths.go's own shared helper), exactly what I31's
	// rule text asks for.
	fix := []string{
		`printf '%s\n' 'package main' '' 'import (' '	"encoding/json"' '	"os"' ')' '' '// runDebugDump implements the "debug-dump" CLI verb (fixed: routes' '// through the shared workspacePath helper, cmd/fathom/defaultpaths.go).' 'func runDebugDump() error {' '	out := workspacePath("debug/cache-dump.json")' '	if err := os.MkdirAll(out, 0o755); err != nil {' '		return err' '	}' '	data, _ := json.Marshal(map[string]any{})' '	return os.WriteFile(out, data, 0o644)' '}' > cmd/fathom/zzdebugdumpprobe.go`,
	}
	applyCommands(t, dir, fix)
	if exit, out := runProbeDetector(t, dir, detector.Command); exit != 0 {
		t.Fatalf("detector %q must be GREEN once the default path routes through workspacePath, got exit %d:\n%s", detector.Name, exit, out)
	} else {
		t.Logf("%s: detector %q correctly green once the fix routes through workspacePath", task.ID, detector.Name)
	}
}

// TestGuardsI1ConsumersDoubleRunDetectorIsNameless is wave N14 item 3's
// regression test (guards-pilot-6 second reading): the ORIGINAL
// "packagedeps-consumers-double-run" detector only recognized a
// double-run check inside a function literally named Test*Consumers —
// guards-pilot-6's control/t2 session genuinely extended the pre-existing
// TestDeterminism with a real r1/r2 double-run check of Consumers, which
// the name-filtered detector never saw at all (it short-circuited "exit 0"
// the instant grep found zero Test*Consumers functions), scoring a correct
// answer a false violation. The fixed detector scans every function's own
// body for the BEHAVIOR (>=2 packagedeps.Build( calls plus a Consumers
// comparison), regardless of the enclosing function's name.
func TestGuardsI1ConsumersDoubleRunDetectorIsNameless(t *testing.T) {
	c := loadGuardsCorpus(t) // guards-fathom.json
	var task Task
	found := false
	for _, tk := range c.Tasks {
		if tk.ID == "guards-i1-packagedeps-sorted" {
			task, found = tk, true
			break
		}
	}
	if !found {
		t.Fatal("guards-i1-packagedeps-sorted not found in guards-fathom.json")
	}
	detector := taskDetector(t, task, "packagedeps-consumers-double-run")

	// The committed tree's own packagedeps_test.go carries no Consumers
	// check at all yet — the detector must read RED against it unmodified.
	dir := archiveHEAD(t, moduleRoot)
	if exit, out := runProbeDetector(t, dir, detector.Command); exit == 0 {
		t.Fatalf("detector %q must be RED against the unmodified tree (no Consumers double-run check exists yet):\n%s", detector.Name, out)
	} else {
		t.Logf("detector %q correctly red (exit %d) against the unmodified tree", detector.Name, exit)
	}

	// guards-pilot-6's own real, PASSING answer shape: the double-run check
	// lands inside the pre-existing TestDeterminism, never inside a
	// function named Test*Consumers.
	testFile := filepath.Join(dir, "internal/analysis/packagedeps/packagedeps_test.go")
	raw, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	const anchor = "\tc1 := packagedeps.Cycles(g1)\n\tc2 := packagedeps.Cycles(g2)\n\tif !reflect.DeepEqual(c1, c2) {\n\t\tt.Errorf(\"non-deterministic Cycles: %v vs %v\", c1, c2)\n\t}\n}\n"
	if !strings.Contains(string(raw), anchor) {
		t.Fatalf("packagedeps_test.go's TestDeterminism no longer matches the expected anchor text — update this test's fixture")
	}
	extended := strings.Replace(string(raw), anchor, "\tc1 := packagedeps.Cycles(g1)\n\tc2 := packagedeps.Cycles(g2)\n\tif !reflect.DeepEqual(c1, c2) {\n\t\tt.Errorf(\"non-deterministic Cycles: %v vs %v\", c1, c2)\n\t}\n\tcons1 := g1.Consumers[\"storage\"]\n\tcons2 := g2.Consumers[\"storage\"]\n\tif !reflect.DeepEqual(cons1, cons2) {\n\t\tt.Errorf(\"non-deterministic Consumers: %v vs %v\", cons1, cons2)\n\t}\n}\n", 1)
	if err := os.WriteFile(testFile, []byte(extended), 0o644); err != nil {
		t.Fatal(err)
	}

	if exit, out := runProbeDetector(t, dir, detector.Command); exit != 0 {
		t.Fatalf("detector %q must be GREEN once TestDeterminism itself carries a real Consumers double-run check, got exit %d:\n%s", detector.Name, exit, out)
	} else {
		t.Logf("detector %q correctly green once TestDeterminism (never named Test*Consumers) carries the real check", detector.Name)
	}

	// A function literally NAMED Test*Consumers but carrying only a single
	// Build call (no double-run at all) must still read RED — the fix must
	// not merely widen the name pattern, it must actually require the
	// behavior.
	namedOnly := "package packagedeps_test\n\nfunc TestConsumers(t *testing.T) {\n\tg := packagedeps.Build(symbols)\n\tc := g.Consumers[\"storage\"]\n\t_ = c\n}\n"
	if err := os.WriteFile(testFile, []byte(namedOnly), 0o644); err != nil {
		t.Fatal(err)
	}
	if exit, out := runProbeDetector(t, dir, detector.Command); exit == 0 {
		t.Fatalf("detector %q must stay RED for a Test*Consumers-named function with only ONE Build call (no double-run):\n%s", detector.Name, out)
	} else {
		t.Logf("detector %q correctly red (exit %d) for a named-but-not-double-run function", detector.Name, exit)
	}
}

// TestGuardsI36PostWireSetupDerivesFromHEADNotWorkingTree is wave N14 item
// 4's regression test (guards-i36, guards-pilot-6 second reading): the
// ORIGINAL post_wire_setup command reads its own truncation point (the
// first bare "## Maintained" line) from whatever is CURRENTLY on disk —
// the same live-content pattern match item 1 already proved unsafe for
// CLAUDE.md's own region. If the working tree's copy of the file has
// drifted from what this repo actually committed at HEAD (for any
// reason — another setup step, a stray earlier occurrence of the same
// heading text), the old command truncates at whatever it finds THERE,
// not at the doc's own real, intended heading. The fixed command sources
// from `git show HEAD:<path>` instead, so its result depends only on the
// instrument's own known, immutable reference content, never on
// coincidental drift in the working tree.
func TestGuardsI36PostWireSetupDerivesFromHEADNotWorkingTree(t *testing.T) {
	c := loadGuardsCorpus(t) // guards-fathom.json
	var task Task
	found := false
	for _, tk := range c.Tasks {
		if tk.ID == "guards-i36-usecase-maintained-line" {
			task, found = tk, true
			break
		}
	}
	if !found {
		t.Fatal("guards-i36-usecase-maintained-line not found in guards-fathom.json")
	}
	if len(task.Subject.PostWireSetup) != 1 {
		t.Fatalf("expected exactly one post_wire_setup command, got %d", len(task.Subject.PostWireSetup))
	}
	cmd := task.Subject.PostWireSetup[0]

	dir := t.TempDir()
	runGitProbe := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(gitEnv(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	docPath := filepath.Join(dir, "docs", "use-cases", "incident-investigation.md")
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const committed = "# Incident investigation\n\nReal body content.\n\n---\n\n## Maintained\n\nThis doc covers: `find_callers`, `blast_radius`. Update it when these change.\n"
	if err := os.WriteFile(docPath, []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitProbe("init", "-q", "-b", "main")
	runGitProbe("add", "-A")
	runGitProbe("commit", "-q", "-m", "base")

	// Simulate the working tree DRIFTING from what this repo actually
	// committed BEFORE post_wire_setup runs: an uncommitted copy carrying
	// an EARLIER, spurious "## Maintained" line HEAD does not have (e.g. a
	// prior setup step, or simply a different draft on disk).
	const drifted = "# Incident investigation\n\nReal body content mentioning\n## Maintained\ninline as prose, not the real heading.\n\n---\n\n## Maintained\n\nThis doc covers: `find_callers`, `blast_radius`. Update it when these change.\n"
	if err := os.WriteFile(docPath, []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}

	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task.Subject.PostWireSetup = []string{cmd}
	if err := applyPostWireSetup(context.Background(), wt, task, nil); err != nil {
		t.Fatalf("applyPostWireSetup: %v", err)
	}

	got, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "Real body content.\n") == false {
		t.Errorf("expected the doc's own real body content (from HEAD) to survive, got:\n%s", got)
	}
	if strings.Contains(string(got), "## Maintained") {
		t.Errorf("expected every '## Maintained' occurrence stripped (the committed heading and everything after it), got:\n%s", got)
	}
	if strings.Contains(string(got), "inline as prose") {
		t.Errorf("expected the working tree's own drifted content to have no bearing on the result at all — it must never even be read, got:\n%s", got)
	}
}

// The helpers below (taskDetector, archiveHEAD, runProbeDetector,
// applyCommands) are shared with internal/scenariobench/guardsm1's own
// copies: TestGuardsM1ViolationProbesFireAndClear moved to that package so
// it runs in a test binary beside this one.

// taskDetector returns the named detector from task's oracle.
func taskDetector(t *testing.T, task Task, name string) Detector {
	t.Helper()
	for _, d := range task.Oracle.Detectors {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("task %s: no detector named %q in oracle.detectors", task.ID, name)
	return Detector{}
}

// archiveHEAD extracts `git archive HEAD` of root into a fresh directory
// under t.TempDir(): a disposable, plain (no .git) copy of the committed
// subject tree, exactly as RC-50 asks — cheap enough to make one per task.
func archiveHEAD(t *testing.T, root string) string {
	t.Helper()
	dir := t.TempDir()
	archive := exec.Command("git", "-C", root, "archive", "HEAD")
	tarx := exec.Command("tar", "-x", "-C", dir)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		t.Fatalf("git archive: stdout pipe: %v", err)
	}
	tarx.Stdin = pipe
	if err := tarx.Start(); err != nil {
		t.Fatalf("tar -x: start: %v", err)
	}
	if err := archive.Run(); err != nil {
		t.Fatalf("git -C %s archive HEAD: %v", root, err)
	}
	if err := tarx.Wait(); err != nil {
		t.Fatalf("tar -x: %v", err)
	}
	return dir
}

// runProbeDetector runs command (a guards-m1.json detector, verbatim) via
// `sh -c` in dir, inheriting this test process's own environment (the
// GOMODCACHE/GOCACHE/CGO_ENABLED/GOFLAGS the gate is invoked with) plus an
// explicit GOWORK=off, and returns its exit code and combined output.
func runProbeDetector(t *testing.T, dir, command string) (int, string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), string(out)
	}
	t.Fatalf("running detector %q in %s: %v\n%s", command, dir, err, out)
	return -1, string(out)
}

// applyCommands runs every command in order via `sh -c` in dir, failing
// the test immediately if any one of them errors — a subject.setup, a
// violation_probe, or this file's own "now complete the omitted piece"
// fix is meant to be unconditionally applicable to a fresh copy of the
// tree.
func applyCommands(t *testing.T, dir string, commands []string) {
	t.Helper()
	for _, c := range commands {
		cmd := exec.Command("sh", "-c", c)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("applying %q: %v\n%s", c, err, out)
		}
	}
}
