package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTestBindings mirrors writeGuardCue (edithook_pre_test.go) for
// .fathom/modules.cue.
func writeTestBindings(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modules.cue"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── binding_file / guard_file ─────────────────────────────────────────────

func TestCheckBindingFile_Absent(t *testing.T) {
	got := checkBindingFile(t.TempDir())
	if got.Status != doctorPass {
		t.Errorf("Status = %v, want pass (absence is the common case)", got.Status)
	}
}

func TestCheckBindingFile_Valid(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)
	got := checkBindingFile(root)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckBindingFile_Invalid_Loud(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "Not_Valid"}]`)
	got := checkBindingFile(root)
	if got.Status != doctorFail {
		t.Errorf("Status = %v, want fail (loud) for an invalid binding file", got.Status)
	}
	if got.Remedy == "" {
		t.Error("expected a remedy for an invalid binding file")
	}
}

func TestCheckGuardFile_Absent(t *testing.T) {
	got := checkGuardFile(t.TempDir())
	if got.Status != doctorPass {
		t.Errorf("Status = %v, want pass", got.Status)
	}
}

func TestCheckGuardFile_Invalid_Loud(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{}]`)
	got := checkGuardFile(root)
	if got.Status != doctorFail {
		t.Errorf("Status = %v, want fail (loud) for an invalid guard file", got.Status)
	}
}

// ─── binding_modules_mounted ────────────────────────────────────────────────

func TestCheckBindingModulesMounted_NoBindings_Pass(t *testing.T) {
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkBindingModulesMounted(hc, "http://unused", t.TempDir(), false)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, want pass (no bindings configured)", got.Status)
	}
}

func TestCheckBindingModulesMounted_DaemonDown_Skipped(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkBindingModulesMounted(hc, "http://unused", root, false)
	if got.Status != doctorSkipped {
		t.Errorf("Status = %v, want skipped (daemon down)", got.Status)
	}
}

func TestCheckBindingModulesMounted_AllMounted_Pass(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{modules: []fakeModuleRow{{module: "platform-standards", path: "/mods/platform-standards.sqlite"}}})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkBindingModulesMounted(hc, srv.URL, root, true)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckBindingModulesMounted_Unmounted_Loud(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{}) // nothing mounted
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkBindingModulesMounted(hc, srv.URL, root, true)
	if got.Status != doctorFail {
		t.Errorf("Status = %v, want fail (loud) for an unmounted bound module", got.Status)
	}
	if !strings.Contains(got.Reason, "platform-standards") {
		t.Errorf("Reason = %q, should name the unmounted module", got.Reason)
	}
}

// ─── binding_module_readiness ──────────────────────────────────────────────

func TestCheckBindingModuleReadiness_NotLocal_Skipped(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{modules: []fakeModuleRow{{module: "platform-standards", path: "/mods/platform-standards.sqlite"}}})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkBindingModuleReadiness(hc, srv.URL, root, true)
	if got.Status != doctorSkipped {
		t.Errorf("Status = %v, Reason = %q, want skipped (\"not checkable: module not local\")", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "not checkable") {
		t.Errorf("Reason = %q, want it to say \"not checkable\"", got.Reason)
	}
}

func TestCheckBindingModuleReadiness_LocalBelowServable_Warn(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, `bindings: [{module: "draft-module"}]`)

	// A local module store directory with a module.cue that will not load
	// (deliberately minimal/invalid) -- runDoctor reports verdict "none",
	// well below servable.
	modDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modDir, "module.cue"), []byte("name: \"draft-module\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(modDir, "draft-module.sqlite")

	srv := fakeDoctorDaemon(t, fakeDaemonCfg{modules: []fakeModuleRow{{module: "draft-module", path: storePath}}})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkBindingModuleReadiness(hc, srv.URL, root, true)
	if got.Status != doctorWarn {
		t.Errorf("Status = %v, Reason = %q, want warn (below servable)", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "draft-module") {
		t.Errorf("Reason = %q, should name the module", got.Reason)
	}
}

// ─── guard_dead_scopes ──────────────────────────────────────────────────────

func TestCheckGuardDeadScopes_NoGuards_Pass(t *testing.T) {
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkGuardDeadScopes(hc, "http://unused", t.TempDir(), "svc", false)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, want pass", got.Status)
	}
}

func TestCheckGuardDeadScopes_MatchesIndexedFile_Pass(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{scopes: ["internal/store/**"]}]`)
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{sources: []string{"internal/store/db.go", "cmd/main.go"}})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkGuardDeadScopes(hc, srv.URL, root, "svc", true)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckGuardDeadScopes_MatchesNothing_Warn(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{scopes: ["nonexistent/dir/**"]}]`)
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{sources: []string{"internal/store/db.go", "cmd/main.go"}})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkGuardDeadScopes(hc, srv.URL, root, "svc", true)
	if got.Status != doctorWarn {
		t.Errorf("Status = %v, Reason = %q, want warn (dead guard)", got.Status, got.Reason)
	}
}

func TestCheckGuardDeadScopes_CommandOnlyGuard_NotedNotFailed(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{command_scopes: ["terraform apply*"]}]`)
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{sources: []string{"cmd/main.go"}})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkGuardDeadScopes(hc, srv.URL, root, "svc", true)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, want pass (a command-only guard has no file scope to check)", got.Status)
	}
	if !strings.Contains(got.Reason, "not checkable") {
		t.Errorf("Reason = %q, should honestly note command guards aren't checkable this way", got.Reason)
	}
}

// ─── guard_include_modules_unbound ─────────────────────────────────────────

func TestCheckGuardIncludeModulesUnbound_NoIncludeModules_Pass(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{scopes: ["**/*.go"]}]`)
	got := checkGuardIncludeModulesUnbound(root)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckGuardIncludeModulesUnbound_WithBindings_Pass(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{scopes: ["**/*.go"], include_modules: true}]`)
	writeTestBindings(t, root, `bindings: [{module: "platform-standards"}]`)
	got := checkGuardIncludeModulesUnbound(root)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q", got.Status, got.Reason)
	}
}

func TestCheckGuardIncludeModulesUnbound_NoBindings_Warn(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{scopes: ["**/*.go"], include_modules: true}]`)
	got := checkGuardIncludeModulesUnbound(root)
	if got.Status != doctorWarn {
		t.Errorf("Status = %v, Reason = %q, want warn (include_modules with nothing bound)", got.Status, got.Reason)
	}
}

// ─── doctor JSON summary carries the warn count additively ────────────────

func TestDoctorExitCode_WarnDoesNotFailTheGate(t *testing.T) {
	checks := []doctorCheck{
		{Name: "a", Status: doctorPass},
		{Name: "b", Status: doctorWarn},
		{Name: "c", Status: doctorSkipped},
	}
	if code := doctorExitCode(checks); code != 0 {
		t.Errorf("doctorExitCode = %d, want 0 (warn must never fail the gate)", code)
	}
}
