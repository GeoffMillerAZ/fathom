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

import "testing"

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
