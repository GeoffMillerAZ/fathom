package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPluginRender pins IO-25 (vision 16 R1): the plugin is generated from
// the same embedded sources init installs — never hand-maintained — with
// all eight namespaced skills, the full hook template spelled via the
// shim-stable `fathom`, and the MCP registration.
func TestPluginRender(t *testing.T) {
	out := filepath.Join(t.TempDir(), "fathom-plugin")
	var stdout, stderr bytes.Buffer
	if code := cmdPluginRender([]string{"--out", out, "--marketplace"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit: %s", stderr.String())
	}

	var manifest map[string]any
	data, err := os.ReadFile(filepath.Join(out, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["name"] != "fathom" || manifest["version"] != pluginVersion {
		t.Errorf("manifest = %v", manifest)
	}

	skills, err := os.ReadDir(filepath.Join(out, "skills"))
	if err != nil || len(skills) != 9 {
		t.Fatalf("skills = %v (%v), want 9 (the pack including custodian)", skills, err)
	}

	var hooks struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	hd, err := os.ReadFile(filepath.Join(out, "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(hd, &hooks); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, groups := range hooks.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				total++
				if strings.Contains(h.Command, "/") {
					t.Errorf("hook command carries a path — must be shim-stable: %q", h.Command)
				}
				if !strings.HasPrefix(h.Command, "fathom ") {
					t.Errorf("command = %q", h.Command)
				}
			}
		}
	}
	if total != len(fathomHookSpecs) {
		t.Errorf("hook entries = %d, want the full template (%d)", total, len(fathomHookSpecs))
	}

	if _, err := os.Stat(filepath.Join(out, ".mcp.json")); err != nil {
		t.Error(".mcp.json missing")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(out), ".claude-plugin", "marketplace.json")); err != nil {
		t.Error("marketplace.json missing")
	}
}
