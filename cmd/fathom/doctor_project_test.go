package main

import (
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

func writeDoctorProject(t *testing.T, files map[projectdir.File]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(projectdir.Under(root), 0o755); err != nil {
		t.Fatal(err)
	}
	for f, body := range files {
		if err := os.WriteFile(f.Path(root), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// FD-6 (docs/plans/fathom-dir.md): one doctor row reports every #Project
// key (file, status, entry count) and any unknown key; the existing guard,
// modules, capabilities and concerns rows keep their names and verdicts.
func TestDoctorProjectRowNamesEveryKey(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())

	good := writeDoctorProject(t, map[projectdir.File]string{
		projectdir.Capabilities: "budgets: {list: 900, map: 400}\n",
		projectdir.Guard:        "guards: [{scopes: [\"internal/**\"], note: \"n\"}]\n",
		projectdir.Modules:      "bindings: [{module: \"payments\"}]\n",
		projectdir.Concerns:     "concerns: {}\n",
		"extra.cue":             "made_up: true\n",
	})
	c := checkProjectKeys(good)
	if c.Name != "project" {
		t.Fatalf("row name %q, want project", c.Name)
	}
	for _, key := range []string{"bindings", "budgets", "capabilities", "concerns", "guards", "layer_rules", "learning", "packs", "risk"} {
		if !strings.Contains(c.Reason, key+":") {
			t.Errorf("reason does not name key %q: %s", key, c.Reason)
		}
	}
	for _, want := range []string{"guards: guard.cue ok (1)", "budgets: capabilities.cue ok (2)", "bindings: modules.cue ok (1)", "layer_rules: unset", "made_up: extra.cue"} {
		if !strings.Contains(c.Reason, want) {
			t.Errorf("reason lacks %q: %s", want, c.Reason)
		}
	}
	if c.Status != doctorWarn {
		t.Errorf("an unknown key is a warning: status %s (%s)", c.Status, c.Reason)
	}
	for _, row := range []doctorCheck{checkGuardFile(good), checkBindingFile(good), checkConcerns(good)} {
		if row.Status != doctorPass {
			t.Errorf("existing row %s changed verdict on a valid project: %s (%s)", row.Name, row.Status, row.Reason)
		}
	}

	// A file that does not parse, a key its reader refuses: FAIL, naming
	// both; every other key is still reported.
	bad := writeDoctorProject(t, map[projectdir.File]string{
		projectdir.Capabilities: "capabilities: {nope: {on: true}}\n",
		projectdir.Guard:        "guards: [{note: \"governs nothing\"}]\n",
		projectdir.Modules:      "bindings: [\n",
		projectdir.Concerns:     "concerns: {}\n",
	})
	c = checkProjectKeys(bad)
	if c.Status != doctorFail {
		t.Fatalf("status %s, want fail: %s", c.Status, c.Reason)
	}
	for _, want := range []string{"modules.cue", "guards: guard.cue invalid (1)", "capabilities: capabilities.cue invalid (1)", "concerns: concerns.cue ok (0)"} {
		if !strings.Contains(c.Reason, want) {
			t.Errorf("reason lacks %q: %s", want, c.Reason)
		}
	}
	wantRows := map[string]doctorStatus{"guard_file": doctorFail, "binding_file": doctorFail, "capabilities": doctorFail, "concerns": doctorPass}
	for _, row := range []doctorCheck{checkGuardFile(bad), checkBindingFile(bad), checkCapabilities("", bad, ""), checkConcerns(bad)} {
		if want, ok := wantRows[row.Name]; !ok || row.Status != want {
			t.Errorf("existing row %s: status %s, want %s (%s)", row.Name, row.Status, want, row.Reason)
		}
	}

	// No .fathom: a passing row that names the absence.
	if c := checkProjectKeys(t.TempDir()); c.Status != doctorPass {
		t.Errorf("absent project: status %s (%s)", c.Status, c.Reason)
	}
}
