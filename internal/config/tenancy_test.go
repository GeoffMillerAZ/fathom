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
