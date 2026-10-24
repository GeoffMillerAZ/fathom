package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// RT-30 (design §1): [tenancy] is a machine contract — read at user scope
// and from env only. A repository's [tenancy] is ignored and named, and
// [daemon].memory_limit keeps its meaning (the GC hint) until [tenancy]
// says otherwise, the conflict named.
func TestBudgetConfigUserScopeOnly(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "repo", ".fathom", "config.toml")
	writeTOML(t, userPath, `
[tenancy]
budget = "2GiB"
shed_at = 0.7
exit_enabled = true
gomaxprocs = 3

[tenancy.host]
refuse_on_swap_growth = "1GiB"
swap_growth_window = "2m"

[tenancy.unmanaged]
budget = "256MiB"
ttl = "4h"

[tenancy.breaker]
safe_after = 4
`)
	// A cloned repository trying to raise the laptop's ceiling.
	writeTOML(t, projectPath, `
[tenancy]
budget = "16GiB"
exit_enabled = false
`)
	loaded, err := Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("a repository [tenancy] must validate (and be ignored), not fail the load: %v", err)
	}
	cfg, notes, err := ResolveTenancyConfig(loaded, envOf(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Budget != 2*tenancy.GiB || cfg.ExitDisabled {
		t.Fatalf("the user budget stands and the repository cannot switch exit off: %+v", cfg)
	}
	if cfg.ShedAt != 0.7 || cfg.GOMAXPROCS != 3 {
		t.Fatalf("user keys: %+v", cfg)
	}
	if cfg.Host.RefuseOnSwapGrowth != tenancy.GiB || cfg.Host.SwapGrowthWindow != 2*time.Minute {
		t.Fatalf("host keys: %+v", cfg.Host)
	}
	if cfg.Unmanaged.Budget != 256*tenancy.MiB || cfg.Unmanaged.TTL != 4*time.Hour || cfg.Breaker.SafeAfter != 4 {
		t.Fatalf("nested tables: %+v", cfg)
	}
	if !loaded.ProjectTenancyIgnored {
		t.Fatalf("Loaded must record the ignored repository [tenancy] for doctor's tenancy_scope")
	}
	if !hasNote(notes, "ignored", projectPath) {
		t.Fatalf("the ignored repository [tenancy] is named with its path: %v", notes)
	}
	if _, ok := loaded.Provenance["tenancy.budget"]; !ok || loaded.Provenance["tenancy.budget"].Source != SourceUser {
		t.Fatalf("tenancy provenance must be the user layer: %+v", loaded.Provenance["tenancy.budget"])
	}

	// Env wins over the user layer.
	cfg, _, err = ResolveTenancyConfig(loaded, envOf(map[string]string{
		"FATHOM_TENANCY_BUDGET": "1GiB", "FATHOM_TENANCY_ADMISSION": "off", "FATHOM_TENANCY_EXIT": "off",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EnvBudget != tenancy.GiB || cfg.Admission != "off" || !cfg.ExitDisabled {
		t.Fatalf("env overrides: %+v", cfg)
	}
	if _, _, err := ResolveTenancyConfig(loaded, envOf(map[string]string{"FATHOM_TENANCY_BUDGET": "lots"})); err == nil {
		t.Fatalf("an unparseable FATHOM_TENANCY_BUDGET is a named error, never a silent default")
	}

	// Migration: only [daemon].memory_limit set → it stays the GC hint,
	// named as superseded; the budget stays auto.
	onlyOld := filepath.Join(dir, "old.toml")
	writeTOML(t, onlyOld, "[daemon]\nmemory_limit = \"3GiB\"\n")
	loaded, err = Load(onlyOld, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg, notes, _ = ResolveTenancyConfig(loaded, envOf(nil))
	if cfg.GCHint != "3GiB" || cfg.Budget != 0 {
		t.Fatalf("memory_limit migrates to the GC hint, budget auto: %+v", cfg)
	}
	if !hasNote(notes, "superseded", "gc_hint") {
		t.Fatalf("the migration is named: %v", notes)
	}
	// FATHOM_MEMORY_LIMIT, the old env, migrates the same way.
	cfg, _, _ = ResolveTenancyConfig(loaded, envOf(map[string]string{"FATHOM_MEMORY_LIMIT": "5GiB"}))
	if cfg.GCHint != "5GiB" {
		t.Fatalf("FATHOM_MEMORY_LIMIT migrates: %+v", cfg)
	}

	// Both set → [tenancy] wins and the conflict is named.
	both := filepath.Join(dir, "both.toml")
	writeTOML(t, both, "[daemon]\nmemory_limit = \"3GiB\"\n\n[tenancy]\ngc_hint = \"budget\"\n")
	loaded, err = Load(both, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg, notes, _ = ResolveTenancyConfig(loaded, envOf(nil))
	if cfg.GCHint != "budget" {
		t.Fatalf("[tenancy].gc_hint wins: %+v", cfg)
	}
	if !hasNote(notes, "conflict", "memory_limit") {
		t.Fatalf("the conflict is named: %v", notes)
	}
}

func hasNote(notes []string, subs ...string) bool {
	for _, n := range notes {
		ok := true
		for _, s := range subs {
			if !strings.Contains(n, s) {
				ok = false
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// Review finding 6: a malformed GC hint — [tenancy].gc_hint, the migrated
// [daemon].memory_limit or FATHOM_MEMORY_LIMIT — is an error naming its
// source, as the old soft limit's was, never a silent fallback to the exit
// mark.
func TestMalformedGCHintIsNamedError(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.toml")
	writeTOML(t, empty, "")
	loaded, err := Load(empty, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"3 gigs", "0GiB", "-1GiB"} {
		_, _, err := ResolveTenancyConfig(loaded, envOf(map[string]string{"FATHOM_MEMORY_LIMIT": v}))
		if err == nil || !strings.Contains(err.Error(), "FATHOM_MEMORY_LIMIT") {
			t.Fatalf("FATHOM_MEMORY_LIMIT=%q: err %v, want a named error", v, err)
		}
	}
	for _, v := range []string{"off", "3GiB", "512MiB"} {
		if _, _, err := ResolveTenancyConfig(loaded, envOf(map[string]string{"FATHOM_MEMORY_LIMIT": v})); err != nil {
			t.Fatalf("FATHOM_MEMORY_LIMIT=%q is valid: %v", v, err)
		}
	}
	bad := filepath.Join(dir, "bad.toml")
	writeTOML(t, bad, "[tenancy]\ngc_hint = \"lots\"\n")
	if loaded, err := Load(bad, ""); err == nil {
		if _, _, err := ResolveTenancyConfig(loaded, envOf(nil)); err == nil || !strings.Contains(err.Error(), "gc_hint") {
			t.Fatalf("[tenancy].gc_hint = lots: err %v, want a named error", err)
		}
	}
	for _, v := range []string{"exit", "budget", "off", "2GiB"} {
		ok := filepath.Join(dir, "ok.toml")
		writeTOML(t, ok, "[tenancy]\ngc_hint = \""+v+"\"\n")
		loaded, err := Load(ok, "")
		if err != nil {
			t.Fatalf("gc_hint %q: load: %v", v, err)
		}
		if _, _, err := ResolveTenancyConfig(loaded, envOf(nil)); err != nil {
			t.Fatalf("gc_hint %q is valid: %v", v, err)
		}
	}
}

// Review finding 9: the ladder's marks are ordered — a refuse mark below
// the shed mark (explicit or against the 0.75 default) is a named error.
func TestTenancyMarksOrdered(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{"[tenancy]\nshed_at = 1.0\nrefuse_at = 0.5\n", false},
		{"[tenancy]\nrefuse_at = 0.5\n", false}, // default shed 0.75
		{"[tenancy]\nshed_at = 0.9\n", true},    // default refuse 1.0
		{"[tenancy]\nshed_at = 0.8\nrefuse_at = 0.8\n", true},
	} {
		p := filepath.Join(dir, "c.toml")
		writeTOML(t, p, tc.body)
		loaded, err := Load(p, "")
		if err != nil {
			if tc.ok {
				t.Fatalf("%q: load: %v", tc.body, err)
			}
			continue // the schema refused it: named at load
		}
		_, _, err = ResolveTenancyConfig(loaded, envOf(nil))
		if tc.ok && err != nil {
			t.Fatalf("%q is ordered: %v", tc.body, err)
		}
		if !tc.ok && (err == nil || !strings.Contains(err.Error(), "refuse_at")) {
			t.Fatalf("%q: err %v, want a named ordering error", tc.body, err)
		}
	}
}

// RT-192: [tenancy.host] allowance tells unset (the 0.6 × shed_at default,
// a zero field) from an explicit 0 (the old rule, AllowanceOff).
func TestTenancyHostAllowanceTellsUnsetFromZero(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		body string
		want float64
	}{
		{"[tenancy]\nshed_at = 0.75\n", 0},
		{"[tenancy.host]\nallowance = 0\n", tenancy.AllowanceOff},
		{"[tenancy.host]\nallowance = 0.0\n", tenancy.AllowanceOff},
		{"[tenancy.host]\nallowance = 0.3\n", 0.3},
	} {
		p := filepath.Join(dir, "c.toml")
		writeTOML(t, p, tc.body)
		loaded, err := Load(p, "")
		if err != nil {
			t.Fatalf("%q: load: %v", tc.body, err)
		}
		cfg, _, err := ResolveTenancyConfig(loaded, envOf(nil))
		if err != nil || cfg.Host.Allowance != tc.want {
			t.Fatalf("%q: allowance %v err %v, want %v", tc.body, cfg.Host.Allowance, err, tc.want)
		}
	}
}

func TestTenancyBudgetEnvAcceptsPlainBytes(t *testing.T) {
	loaded := Loaded{}
	for env, want := range map[string]int64{"536870912": 536870912, " 1024 ": 1024, "1GiB": tenancy.GiB, "0": 0} {
		cfg, _, err := ResolveTenancyConfig(loaded, envOf(map[string]string{"FATHOM_TENANCY_BUDGET": env}))
		if err != nil {
			t.Fatalf("FATHOM_TENANCY_BUDGET=%q: %v", env, err)
		}
		if cfg.EnvBudget != want {
			t.Errorf("FATHOM_TENANCY_BUDGET=%q: EnvBudget = %d, want %d", env, cfg.EnvBudget, want)
		}
	}
	for _, bad := range []string{"-1", "1.5", "lots", "5 MB"} {
		_, _, err := ResolveTenancyConfig(loaded, envOf(map[string]string{"FATHOM_TENANCY_BUDGET": bad}))
		if err == nil || !strings.Contains(err.Error(), "byte count") || !strings.Contains(err.Error(), "IEC") {
			t.Errorf("FATHOM_TENANCY_BUDGET=%q: err = %v, want one naming both accepted forms", bad, err)
		}
	}
}
