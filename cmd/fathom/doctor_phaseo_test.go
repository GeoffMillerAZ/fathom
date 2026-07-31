package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
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

// ─── O22: the fathom-init scaffold itself must PASS, not FAIL ─────────────
//
// docs/dogfood-observations.md O22: every fresh `fathom init` wrote
// .fathom/guard.cue and .fathom/modules.cue as fully-commented scaffolds
// that then failed their own doctor checks. These pin the fix at the
// doctor layer using the ACTUAL ExampleTemplate() bytes init writes —
// mirroring guardspec/bindspec's own pinning tests one layer up.

func TestCheckBindingFile_ScaffoldEmpty_Pass(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, bindspec.ExampleTemplate())
	got := checkBindingFile(root)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q, want pass for init's own scaffold", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "scaffold") {
		t.Errorf("Reason = %q, want it to name the scaffolded-empty state (not just look identical to the plain valid-with-N-bindings pass)", got.Reason)
	}
}

func TestCheckGuardFile_ScaffoldEmpty_Pass(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, guardspec.ExampleTemplate())
	got := checkGuardFile(root)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q, want pass for init's own scaffold", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "scaffold") {
		t.Errorf("Reason = %q, want it to name the scaffolded-empty state", got.Reason)
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

// TestCheckBindingModulesMounted_ScaffoldEmpty_PassNotSkip is O22's
// dependent-check assertion: the init scaffold (0 bindings, loaded clean
// per the loader fix) must let this check run normally against an empty
// set and pass trivially — never SKIP as "binding_file is invalid".
func TestCheckBindingModulesMounted_ScaffoldEmpty_PassNotSkip(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, bindspec.ExampleTemplate())
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkBindingModulesMounted(hc, srv.URL, root, true)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q, want pass (trivially true over zero bindings), not skipped", got.Status, got.Reason)
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

// TestCheckBindingModuleReadiness_ScaffoldEmpty_Pass is O22's
// dependent-check assertion for binding_module_readiness: the init
// scaffold (0 bindings) must pass trivially, not SKIP as "binding_file is
// invalid".
func TestCheckBindingModuleReadiness_ScaffoldEmpty_Pass(t *testing.T) {
	root := t.TempDir()
	writeTestBindings(t, root, bindspec.ExampleTemplate())
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkBindingModuleReadiness(hc, srv.URL, root, true)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q, want pass, not skipped", got.Status, got.Reason)
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

// TestCheckGuardDeadScopes_ScaffoldEmpty_Pass is O22's dependent-check
// assertion for guard_dead_scopes: the init scaffold (0 guards) must pass
// trivially, not SKIP as "guard_file is invalid".
func TestCheckGuardDeadScopes_ScaffoldEmpty_Pass(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, guardspec.ExampleTemplate())
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{sources: []string{"cmd/main.go"}})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	got := checkGuardDeadScopes(hc, srv.URL, root, "svc", true)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q, want pass, not skipped", got.Status, got.Reason)
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

// TestCheckGuardIncludeModulesUnbound_ScaffoldEmpty_Pass is O22's
// dependent-check assertion for guard_include_modules_unbound: the init
// scaffold (0 guards) must pass trivially ("no guard sets include_modules"),
// not SKIP as "guard_file is invalid".
func TestCheckGuardIncludeModulesUnbound_ScaffoldEmpty_Pass(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, guardspec.ExampleTemplate())
	got := checkGuardIncludeModulesUnbound(root)
	if got.Status != doctorPass {
		t.Errorf("Status = %v, Reason = %q, want pass, not skipped", got.Status, got.Reason)
	}
}

// ─── O22 end-to-end: a real `fathom init` scaffold must clear the whole
// phase-O checklist with zero FAILs ─────────────────────────────────────
//
// This is the deliverable the mission called out as most important: not a
// hand-authored fixture, but writeFathomConfigScaffolds — the exact
// function `fathom init` calls (init.go) — writing to a scratch root, then
// every phase-O check run against that root exactly as `fathom doctor`
// would. Before the O22 fix this produced two FAILs (binding_file,
// guard_file) and four SKIPs; after it, every one of the six checks PASSes.
func TestPhaseOChecks_RealInitScaffold_ZeroFails(t *testing.T) {
	root := t.TempDir()
	var stdout strings.Builder
	if err := writeFathomConfigScaffolds(root, &stdout); err != nil {
		t.Fatalf("writeFathomConfigScaffolds: %v", err)
	}

	srv := fakeDoctorDaemon(t, fakeDaemonCfg{sources: []string{"cmd/main.go"}})
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}

	checks := []doctorCheck{
		checkBindingFile(root),
		checkGuardFile(root),
		checkBindingModulesMounted(hc, srv.URL, root, true),
		checkBindingModuleReadiness(hc, srv.URL, root, true),
		checkGuardDeadScopes(hc, srv.URL, root, "svc", true),
		checkGuardIncludeModulesUnbound(root),
	}
	for _, c := range checks {
		if c.Status == doctorFail {
			t.Errorf("%s: Status = fail, Reason = %q — a repo that just ran `fathom init` must never fail its own doctor check", c.Name, c.Reason)
		}
		if c.Status == doctorSkipped {
			t.Errorf("%s: Status = skipped, Reason = %q — an empty-but-valid scaffold should run each check normally (trivially passing), not skip it", c.Name, c.Reason)
		}
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
