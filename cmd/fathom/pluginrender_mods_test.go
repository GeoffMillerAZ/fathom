package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// scoutFrontmatter splits a rendered agent file into its frontmatter lines (key: value) and its body.
func scoutFrontmatter(t *testing.T, content string) (map[string]string, string) {
	t.Helper()
	if !strings.HasPrefix(content, "---\n") {
		t.Fatalf("agent file does not open with frontmatter:\n%s", content)
	}
	head, body, ok := strings.Cut(strings.TrimPrefix(content, "---\n"), "\n---\n")
	if !ok {
		t.Fatalf("agent frontmatter is not closed:\n%s", content)
	}
	fields := map[string]string{}
	for _, line := range strings.Split(head, "\n") {
		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			t.Fatalf("frontmatter line %q is not key: value", line)
		}
		fields[k] = v
	}
	return fields, body
}

// TestPluginRenderEmitsScoutAgentFromTheDefaultSurface pins the scout (docs/design/claude-code-mods.md §7.2): render
// writes agents/fathom-scout.md whose tools are the default surface's tools by their plugin-scoped names, read from
// the registry (never hand-copied), with omitClaudeMd and low effort, and a body of routing plus the answer contract.
func TestPluginRenderEmitsScoutAgentFromTheDefaultSurface(t *testing.T) {
	out := renderPluginTree(t)
	data, err := os.ReadFile(filepath.Join(out, "agents", "fathom-scout.md"))
	if err != nil {
		t.Fatal(err)
	}
	fields, body := scoutFrontmatter(t, string(data))

	defaultTools := mcp.DefaultSurfaceTools()
	if len(defaultTools) != 8 {
		t.Fatalf("default surface = %v, want the eight tools", defaultTools)
	}
	var want []string
	for _, tool := range defaultTools {
		want = append(want, pluginScopedToolPrefix+tool)
	}
	if got := strings.Split(fields["tools"], ", "); !reflect.DeepEqual(got, want) {
		t.Errorf("tools = %v, want the default surface by plugin-scoped name %v", got, want)
	}
	if fields["name"] != "fathom-scout" {
		t.Errorf("name = %q", fields["name"])
	}
	if fields["description"] == "" {
		t.Error("description is empty: Claude Code picks an agent by it")
	}
	if fields["omitClaudeMd"] != "true" || fields["effort"] != "low" {
		t.Errorf("omitClaudeMd = %q, effort = %q; want true and low", fields["omitClaudeMd"], fields["effort"])
	}
	if len(fields) != 5 {
		t.Errorf("frontmatter keys = %v, want name, description, tools, omitClaudeMd, effort", fields)
	}

	// Routing: one line per default tool, each naming the tool.
	for _, tool := range defaultTools {
		if !strings.Contains(body, "- `"+tool+"`: ") {
			t.Errorf("body carries no routing line for %s:\n%s", tool, body)
		}
	}
	// The answer contract.
	for _, clause := range []string{"At most 40 lines", "`file:line` receipt", "freshness"} {
		if !strings.Contains(body, clause) {
			t.Errorf("body lacks the answer contract's %q:\n%s", clause, body)
		}
	}

	// A default tool render cannot route fails the render instead of shipping a scout that never names it.
	if _, err := renderScoutAgent(append(append([]string(nil), defaultTools...), "no_such_tool")); err == nil {
		t.Error("renderScoutAgent accepted a tool with no routing line")
	}
}

// TestPluginRenderSettingsHoldsOnlySubagentStatusLine pins the core plugin's settings.json (docs/design/
// claude-code-mods.md §7.3): it holds subagentStatusLine alone, a command that runs `fathom statusline --subagent`
// when the binary is on PATH and prints `fathom: not on PATH` when it is not, spelled with no path.
func TestPluginRenderSettingsHoldsOnlySubagentStatusLine(t *testing.T) {
	out := renderPluginTree(t)
	data, err := os.ReadFile(filepath.Join(out, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]map[string]string
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json: %v\n%s", err, data)
	}
	if len(settings) != 1 {
		t.Fatalf("settings.json keys = %v, want subagentStatusLine alone", settings)
	}
	line, ok := settings["subagentStatusLine"]
	if !ok {
		t.Fatalf("settings.json = %s, want subagentStatusLine", data)
	}
	const want = "command -v fathom >/dev/null 2>&1 && fathom statusline --subagent || echo 'fathom: not on PATH'"
	if !reflect.DeepEqual(line, map[string]string{"type": "command", "command": want}) {
		t.Errorf("subagentStatusLine = %v, want type command running %q", line, want)
	}
	if strings.Contains(strings.ReplaceAll(line["command"], ">/dev/null", ""), "/") {
		t.Errorf("status-line command carries a path, must be shim-stable: %q", line["command"])
	}
}
