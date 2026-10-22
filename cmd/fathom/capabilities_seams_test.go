package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapabilitiesExposeOptInSeams(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOME", t.TempDir())
	root := t.TempDir()
	var out, stderr bytes.Buffer
	// No daemon is needed to list local switches.
	if code := cmdCapabilities([]string{"--repo", root, "--url", "http://127.0.0.1:1", "--json"}, &out, &stderr); code != 0 {
		t.Fatalf("code %d: %s", code, &stderr)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	rows, _ := m["hook_seams"].([]any)
	for _, id := range []string{"readouts", "claim_check", "gate_preflight", "tests_to_run", "read_outline_hint", "prompt_definitions"} {
		found := false
		for _, r := range rows {
			v := r.(map[string]any)
			if v["id"] == id {
				found = true
				if !strings.Contains(v["command"].(string), "capabilities set "+id) {
					t.Errorf("no set command for %s: %v", id, v)
				}
			}
		}
		if !found {
			t.Errorf("missing seam %s", id)
		}
	}
}

func TestCapabilitiesSetOptInSeamChangesRealConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOME", t.TempDir())
	root := t.TempDir()
	doctorWriteFixture(t, root, ".fathom/config.toml", "# preserve me\n[hooks]\n# another field\ntest_run = true\n")
	for _, value := range []string{"on", "off"} {
		var out, stderr bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", root, "claim_check", value}, &out, &stderr); code != 0 {
			t.Fatalf("set %s: %s", value, &stderr)
		}
		if got := hooksKeyOn(root, hookOptInClaimCheck); got != (value == "on") {
			t.Fatalf("actual hook switch=%v for %s", got, value)
		}
	}
	b, err := os.ReadFile(filepath.Join(root, ".fathom/config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "# preserve me") || !strings.Contains(string(b), "test_run = true") {
		t.Fatalf("unrelated config lost: %s", b)
	}
}

func TestDoctorFlagsDoubleHookRegistration(t *testing.T) {
	root := doctorHookFixture(t, "fathom prompt-hook")
	b, err := os.ReadFile(filepath.Join(root, ".claude/settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	doctorWriteFixture(t, os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json", string(b))
	found := false
	for _, row := range doctorRepoChecks(root, "claude") {
		if row.Name == "duplicate_hook_registration" {
			found = true
			if row.Status != doctorWarn || !strings.Contains(row.Reason, "PostToolUse") || row.Remedy == "" {
				t.Fatalf("double scope = %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("double registration is invisible")
	}
}

func TestCapabilitiesSeamCommandsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOME", t.TempDir())
	root := t.TempDir()
	for _, row := range hookSeams(root) {
		args := append([]string{"set", "--repo", root}, strings.Fields(strings.TrimPrefix(row.Command, "fathom capabilities set "))...)
		var out, stderr bytes.Buffer
		if code := cmdCapabilities(args, &out, &stderr); code != 0 {
			t.Fatalf("%s: %s", row.Command, &stderr)
		}
	}
	rows := hookSeams(root)
	for _, row := range rows {
		if row.State != "on" {
			t.Fatalf("actual control not enabled: %+v", row)
		}
	}
}
