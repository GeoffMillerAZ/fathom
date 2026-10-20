package main

// FD-34 (docs/plans/fathom-dir.md): the config.toml switches whose field is
// not user-only surface as capability ids, sourced from the same booleans;
// capabilities.cue wins on conflict and doctor names it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

func writeFathomFile(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func reportFor(t *testing.T, reports []capability.Report, id string) capability.Report {
	t.Helper()
	for _, r := range reports {
		if r.Capability.ID == id {
			return r
		}
	}
	t.Fatalf("no capability %q in the registry", id)
	return capability.Report{}
}

func TestCapabilitiesSurfaceConfigSwitches(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	// auto_refresh is user-only: even set in the repository's file it must
	// not surface (A2, I64, SEC-2).
	root := t.TempDir()
	writeFathomFile(t, root, "config.toml", "[hooks]\ntest_run = true\npin_check = \"off\"\n[guards]\nenforce = false\n[daemon]\nauto_refresh = true\n")

	reg := capability.Default()
	if _, ok := reg.Get("auto-refresh"); ok {
		t.Fatal("auto-refresh must not be a capability id")
	}
	res, errs := loadCapabilityConfig(reg, root)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	reports := reg.DetectAll(capability.Probe{}, res)
	for id, want := range map[string]bool{"test-run-hook": true, "pin-check": false, "guard-enforce": false} {
		r := reportFor(t, reports, id)
		if r.Enabled != want || r.Scope != capability.ScopeConfig {
			t.Errorf("%s: enabled=%v scope=%q, want enabled=%v scope=%q", id, r.Enabled, r.Scope, want, capability.ScopeConfig)
		}
	}

	// Unset switches fall back to the built-in default, from the same booleans.
	bare := t.TempDir()
	res, _ = loadCapabilityConfig(reg, bare)
	reports = reg.DetectAll(capability.Probe{}, res)
	for id, want := range map[string]bool{"test-run-hook": false, "pin-check": true, "guard-enforce": true} {
		if r := reportFor(t, reports, id); r.Enabled != want || r.Scope != capability.ScopeBuiltIn {
			t.Errorf("%s unset: enabled=%v scope=%q, want enabled=%v built-in", id, r.Enabled, r.Scope, want)
		}
	}
}

func TestCapabilitiesCueWinsOverConfigAndDoctorNamesConflict(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	root := t.TempDir()
	writeFathomFile(t, root, "config.toml", "[hooks]\npin_check = \"off\"\n")
	writeFathomFile(t, root, "capabilities.cue", "capabilities: {\"pin-check\": on: true}\n")

	reg := capability.Default()
	res, errs := loadCapabilityConfig(reg, root)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	r := reportFor(t, reg.DetectAll(capability.Probe{}, res), "pin-check")
	if !r.Enabled || r.Scope != capability.ScopeProject {
		t.Fatalf("pin-check enabled=%v scope=%q, want capabilities.cue (on) to win", r.Enabled, r.Scope)
	}
	c := checkCapabilities("", root, "")
	if !strings.Contains(c.Reason, "pin-check") || !strings.Contains(c.Reason, "capabilities.cue wins") {
		t.Fatalf("doctor reason does not name the conflict: %s", c.Reason)
	}
}
