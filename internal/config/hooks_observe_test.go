package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHooksSeamsRejectInvalidObserveModesByKey(t *testing.T) {
	for _, key := range []string{"claim_check", "gate_preflight", "tests_to_run", "read_outline_hint", "prompt_definitions"} {
		for _, mode := range []string{`"on"`, `"Observe"`, "1"} {
			t.Run(key+"/"+mode, func(t *testing.T) {
				p := filepath.Join(t.TempDir(), "config.toml")
				writeTOML(t, p, "[hooks]\n"+key+" = "+mode+"\n")
				if _, err := Load("", p); err == nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("%s = %s: error=%v, want rejection naming %s", key, mode, err, key)
				}
			})
		}
	}
}

func TestHooksSeamsAcceptBooleansAndObserve(t *testing.T) {
	for _, tc := range []struct {
		key string
		on  func(Hooks) bool
	}{
		{"claim_check", func(h Hooks) bool { return h.ClaimCheck }},
		{"gate_preflight", func(h Hooks) bool { return h.GatePreflight }},
		{"tests_to_run", func(h Hooks) bool { return h.TestsToRun }},
		{"read_outline_hint", func(h Hooks) bool { return h.ReadOutlineHint }},
		{"prompt_definitions", func(h Hooks) bool { return h.PromptDefinitions }},
	} {
		for _, mode := range []string{"true", "false", `"observe"`} {
			t.Run(tc.key+"/"+mode, func(t *testing.T) {
				p := filepath.Join(t.TempDir(), "config.toml")
				writeTOML(t, p, "[hooks]\n"+tc.key+" = "+mode+"\n")
				loaded, err := Load("", p)
				if err != nil {
					t.Fatal(err)
				}
				if got := tc.on(loaded.Config.Hooks); got != (mode != "false") {
					t.Errorf("enabled = %v for %s", got, mode)
				}
				if got := loaded.Config.Hooks.Observes("hooks." + tc.key); got != (mode == `"observe"`) {
					t.Errorf("observes = %v for %s", got, mode)
				}
				if got := loaded.Provenance["hooks."+tc.key]; got.Source != SourceProject || got.Path != p {
					t.Errorf("provenance = %+v", got)
				}
			})
		}
	}
}

func TestHooksObserveLayerOverrides(t *testing.T) {
	dir := t.TempDir()
	user, project := filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml")
	for _, tc := range []struct {
		user, project string
		on, observe   bool
	}{
		{`"observe"`, "true", true, false},
		{`"observe"`, "false", false, false},
		{"true", `"observe"`, true, true},
		{"false", `"observe"`, true, true},
	} {
		writeTOML(t, user, "[hooks]\nclaim_check = "+tc.user+"\n")
		writeTOML(t, project, "[hooks]\nclaim_check = "+tc.project+"\n")
		loaded, err := Load(user, project)
		if err != nil {
			t.Fatal(err)
		}
		h := loaded.Config.Hooks
		if h.ClaimCheck != tc.on || h.Observes("hooks.claim_check") != tc.observe {
			t.Errorf("user %s, project %s: enabled=%v observe=%v", tc.user, tc.project, h.ClaimCheck, h.Observes("hooks.claim_check"))
		}
	}
}
