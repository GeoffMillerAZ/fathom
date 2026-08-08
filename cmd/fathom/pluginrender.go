package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/buildinfo"
)

// fathom plugin render (vision 16 R1, plan IO-25): generate the Claude Code
// plugin directory — skills (all eight, namespaced /fathom:<name>), the
// full hook template as plugin hooks, and the MCP registration — from the
// SAME embedded sources init installs. A build artifact, never
// hand-maintained: a hand-edited plugin is the schema-copy bug class, so
// re-rendering overwrites unconditionally (the output dir belongs to the
// generator). Distribution: commit the rendered tree to a marketplace repo;
// bump pluginVersion to ship an update (the reconcile channel).

// pluginVersion is the plugin's semver — bumped deliberately per template/
// skill change that should propagate to installed fleets; the embedded
// fathom build version rides in metadata for provenance.
const pluginVersion = "0.1.0"

func cmdPluginRender(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plugin render", stderr)
	out := fs.String("out", "fathom-plugin", "output directory (created/overwritten — it belongs to the generator)")
	mcpURL := fs.String("mcp-url", "http://localhost:7575/rpc/agent", "daemon endpoint the plugin registers")
	marketplace := fs.Bool("marketplace", false, "also write .claude-plugin/marketplace.json beside the plugin dir")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// plugin.json
	manifest := map[string]any{
		"name":        "fathom",
		"displayName": "fathom",
		"version":     pluginVersion,
		"description": "Governed code intelligence: index-first discovery, obligations at edit time, done-means-proven gates — skills, hooks, and the MCP registration in one install.",
		"metadata":    map[string]any{"fathomVersion": buildinfo.Version()},
	}
	if err := writeJSON(filepath.Join(*out, ".claude-plugin", "plugin.json"), manifest); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// skills/
	for _, name := range skillsdist.Names {
		content, err := skillsdist.FS.ReadFile(name + "/SKILL.md")
		if err != nil {
			fmt.Fprintf(stderr, "error: embedded skill %q: %v\n", name, err)
			return 1
		}
		dest := filepath.Join(*out, "skills", name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	// hooks/hooks.json — the SAME template init writes, spelled via the
	// shim-stable `fathom` (never a checkout path).
	hooksObj := map[string]any{}
	for _, spec := range fathomHookSpecs {
		entry := map[string]any{
			"type":    "command",
			"command": "fathom " + spec.sub,
			"timeout": spec.timeout,
		}
		group := map[string]any{"hooks": []any{entry}}
		if spec.matcher != "" {
			group["matcher"] = spec.matcher
		}
		arr, _ := hooksObj[spec.event].([]any)
		hooksObj[spec.event] = append(arr, group)
	}
	if err := writeJSON(filepath.Join(*out, "hooks", "hooks.json"), map[string]any{"hooks": hooksObj}); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// .mcp.json — the daemon registration.
	if err := writeJSON(filepath.Join(*out, ".mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			"fathom": map[string]any{"type": "http", "url": *mcpURL},
		},
	}); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *marketplace {
		mp := map[string]any{
			"name":  "fathom-plugins",
			"owner": map[string]any{"name": "fathom"},
			"plugins": []any{map[string]any{
				"name":        "fathom",
				"source":      "./" + filepath.Base(*out),
				"description": manifest["description"],
				"version":     pluginVersion,
			}},
		}
		if err := writeJSON(filepath.Join(filepath.Dir(*out), ".claude-plugin", "marketplace.json"), mp); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "plugin rendered: %s (v%s, %d skills, %d hook groups, MCP → %s) — validate with `claude plugin validate %s`\n",
		*out, pluginVersion, len(skillsdist.Names), len(fathomHookSpecs), *mcpURL, *out)
	return 0
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
