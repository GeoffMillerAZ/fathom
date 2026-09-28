package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	if err != nil || len(skills) != 10 {
		t.Fatalf("skills = %v (%v), want 10 (the pack including custodian+findings)", skills, err)
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

// TestPluginRenderCarriesDefaultSurface pins RH-5 (docs/plans/reach.md,
// docs/design/reach.md §4): the rendered plugin documents the SAME default
// surface and opt-in groups the registry actually serves at
// POST /rpc/agent — read from internal/mcp.DefaultSurfaceTools/
// ExtendedSurfaceTools, never hand-copied — so an installed plugin cannot
// drift from what its own daemon answers. The Claude Code manifest's open
// `metadata` field carries the structured data (the Agent Plugins 1.0
// plugin.json is a CLOSED schema with no metadata field — schema/plugin/
// schema.cue — so that dialect cannot carry it); both MCP registration
// files (mcp.json and .mcp.json) carry the default-surface tool count in
// a header on the fathom server entry, the one open field #HTTPServer
// admits in both dialects (schema/plugin/schema.cue's #HTTPServer).
func TestPluginRenderCarriesDefaultSurface(t *testing.T) {
	wantDefault, wantGroups := renderPluginSurface()
	if len(wantDefault) == 0 {
		t.Fatal("renderPluginSurface returned no default-surface tools")
	}
	if len(wantGroups) == 0 {
		t.Fatal("renderPluginSurface returned no groups")
	}

	out := filepath.Join(t.TempDir(), "fathom-plugin")
	var stdout, stderr bytes.Buffer
	if code := cmdPluginRender([]string{"--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit: %s", stderr.String())
	}

	var claudeManifest struct {
		Metadata struct {
			DefaultSurface []string            `json:"defaultSurface"`
			SurfaceGroups  map[string][]string `json:"surfaceGroups"`
		} `json:"metadata"`
	}
	data, err := os.ReadFile(filepath.Join(out, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &claudeManifest); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(claudeManifest.Metadata.DefaultSurface, wantDefault) {
		t.Errorf("metadata.defaultSurface = %v, want %v", claudeManifest.Metadata.DefaultSurface, wantDefault)
	}
	if !reflect.DeepEqual(claudeManifest.Metadata.SurfaceGroups, wantGroups) {
		t.Errorf("metadata.surfaceGroups = %v, want %v", claudeManifest.Metadata.SurfaceGroups, wantGroups)
	}

	wantHeader := strings.Join(wantDefault, ",")
	for _, rel := range []string{"mcp.json", ".mcp.json"} {
		var doc struct {
			MCPServers map[string]struct {
				Headers map[string]string `json:"headers"`
			} `json:"mcpServers"`
		}
		raw, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		got := doc.MCPServers["fathom"].Headers["X-Fathom-Default-Surface"]
		if got != wantHeader {
			t.Errorf("%s: X-Fathom-Default-Surface header = %q, want %q", rel, got, wantHeader)
		}
	}
}

// TestPluginTemplateDigestPinned pins the rendered tree's content digest
// (docs/design/reach.md §4: "pluginVersion is driven by a digest of the
// rendered tree"). A change to any skill, hook spec, or the default
// surface/groups this render stamps into the manifests moves the digest;
// when that happens deliberately, regenerate this pin AND bump
// pluginVersion in the same change — the two travel together, so a
// template change can never ship silently unversioned.
func TestPluginTemplateDigestPinned(t *testing.T) {
	const pinnedForVersion = pluginVersion
	got, err := pluginTemplateDigest()
	if err != nil {
		t.Fatal(err)
	}
	if got != pluginTemplateDigestPin {
		t.Errorf("pluginTemplateDigest() = %s, want the pin %s (pinned for pluginVersion %s) — if this template change is deliberate, update pluginTemplateDigestPin AND bump pluginVersion together",
			got, pluginTemplateDigestPin, pinnedForVersion)
	}
}
