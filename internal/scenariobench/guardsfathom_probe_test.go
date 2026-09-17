// guardsfathom_probe_test.go — wave N14 item 2's mechanical proof that
// testdata/scenariobench/guards-fathom.json's new
// "debug-dump-default-under-workspace" detector actually discriminates,
// the same shape guardsm1_probe_test.go already proves for guards-m1.json
// (RC-50): guards-pilot-6's second reading found that the task's ORIGINAL
// oracle — reusing the pre-existing TestDefaultStorePathNeverTheCwd
// verbatim — never exercises a brand-new verb's own default path at all,
// so two real sessions hardcoded ~/.fathom/debug unconditionally and both
// scored PASS. This file applies the corpus's own violation_probe (the
// naive, hardcoded-home fix) to a disposable copy of the subject tree and
// checks the new detector goes red, then applies a compliant fix (routes
// through the shared workspacePath helper, cmd/fathom/defaultpaths.go)
// and checks the same detector goes green.
package scenariobench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGuardsI31DebugDumpDetectorFiresAndClears is wave N14 item 2's
// anchor. It reuses guardsm1_probe_test.go's archiveHEAD/applyCommands/
// runProbeDetector/taskDetector helpers (same package) rather than
// reimplementing them.
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
