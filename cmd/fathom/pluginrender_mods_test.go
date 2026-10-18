package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
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

// marketplaceTree reads every file under root's .claude-plugin/ and plugins/fathom/, keyed by slash path relative to
// root, skipping the type files Claude Code writes into a loaded plugin's .claude-plugin/types/ (gitignored).
func marketplaceTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, top := range []string{".claude-plugin", filepath.Join("plugins", "fathom")} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "types" && filepath.Base(filepath.Dir(path)) == ".claude-plugin" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)] = data
			return nil
		})
		if err != nil {
			t.Fatalf("%s: %v", top, err)
		}
	}
	return files
}

// TestCommittedMarketplaceMatchesAFreshRender pins the committed marketplace (docs/design/claude-code-mods.md §7.4,
// §9): `plugin render --out <root>/plugins/fathom --marketplace <root>` reproduces the repository's own
// .claude-plugin/marketplace.json and plugins/fathom/ byte for byte, so the committed tree is never hand-maintained.
// The build-version metadata is normalized at the source: a marketplace render stamps no build version, and the
// committed tree carries no machine path either.
func TestCommittedMarketplaceMatchesAFreshRender(t *testing.T) {
	tmp := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := cmdPluginRender([]string{"--out", filepath.Join(tmp, "plugins", "fathom"), "--marketplace", tmp}, &stdout, &stderr); code != 0 {
		t.Fatalf("plugin render exit %d: %s", code, stderr.String())
	}
	fresh := marketplaceTree(t, tmp)
	committed := marketplaceTree(t, filepath.Join("..", ".."))

	const regen = "regenerate with `go run ./cmd/fathom plugin render --out plugins/fathom --marketplace .` from the repository root"
	for path, want := range fresh {
		got, ok := committed[path]
		switch {
		case !ok:
			t.Errorf("%s: rendered, not committed; %s", path, regen)
		case !bytes.Equal(got, want):
			t.Errorf("%s: committed bytes differ from a fresh render; %s", path, regen)
		}
	}
	for path := range committed {
		if _, ok := fresh[path]; !ok {
			t.Errorf("%s: committed, not rendered; %s", path, regen)
		}
	}

	var mp struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(fresh[".claude-plugin/marketplace.json"], &mp); err != nil {
		t.Fatalf("marketplace.json: %v", err)
	}
	if mp.Name != "fathom-plugins" || len(mp.Plugins) != 1 || mp.Plugins[0].Name != "fathom" || mp.Plugins[0].Source != "./plugins/fathom" {
		t.Errorf("marketplace = %+v, want fathom-plugins listing fathom at ./plugins/fathom", mp)
	}

	home, _ := os.UserHomeDir()
	for path, data := range committed {
		for _, leak := range []string{"fathomVersion", tmp, "/Users/", "/home/", home} {
			if leak != "" && bytes.Contains(data, []byte(leak)) {
				t.Errorf("%s carries %q: a committed tree holds no build version and no machine or home path", path, leak)
			}
		}
	}
}

// TestPluginRenderMarketplaceRefusesAnOutOutsideItsRoot: a source must be relative to the manifest's root.
func TestPluginRenderMarketplaceRefusesAnOutOutsideItsRoot(t *testing.T) {
	tmp := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdPluginRender([]string{"--out", filepath.Join(tmp, "elsewhere"), "--marketplace", filepath.Join(tmp, "repo")}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "inside") {
		t.Errorf("exit %d, stderr %q; want a refusal naming that --out must sit inside the root", code, stderr.String())
	}
}
