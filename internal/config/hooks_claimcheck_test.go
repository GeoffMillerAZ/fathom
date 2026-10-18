package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestConfig_HooksClaimCheckAndGatePreflightDefaultOff pins RO-38's and
// RO-40's keys: `[hooks] claim_check` and `[hooks] gate_preflight` are off
// until a layer sets them, a repository may set them (the scope registry's
// behaviour class, like test_run), and a non-bool is refused naming the key.
func TestConfig_HooksClaimCheckAndGatePreflightDefaultOff(t *testing.T) {
	dir := t.TempDir()
	loaded, err := Load(filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Config.Hooks.ClaimCheck || loaded.Config.Hooks.GatePreflight {
		t.Errorf("hooks = %+v with no layer setting them, want both off", loaded.Config.Hooks)
	}
	for _, k := range []string{"hooks.claim_check", "hooks.gate_preflight"} {
		if _, ok := loaded.Provenance[k]; ok {
			t.Errorf("%s has provenance with no layer setting it", k)
		}
		if !RepoMaySet(k) {
			t.Errorf("%s: a repository may not set it, want the behaviour class", k)
		}
	}

	projectPath := filepath.Join(dir, "on.toml")
	writeTOML(t, projectPath, "[hooks]\nclaim_check = true\ngate_preflight = true\n")
	loaded, err = Load("", projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.Config.Hooks.ClaimCheck || !loaded.Config.Hooks.GatePreflight {
		t.Errorf("hooks = %+v, want both on", loaded.Config.Hooks)
	}
	if info := loaded.Provenance["hooks.claim_check"]; info.Source != SourceProject || info.Path != projectPath {
		t.Errorf("claim_check provenance = %+v, want the project layer", info)
	}

	for key, body := range map[string]string{"claim_check": "claim_check = \"on\"\n", "gate_preflight": "gate_preflight = 1\n"} {
		bad := filepath.Join(dir, key+".toml")
		writeTOML(t, bad, "[hooks]\n"+body)
		if _, err := Load("", bad); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("Load(%s) = %v, want an error naming %s", body, err, key)
		}
	}
}
