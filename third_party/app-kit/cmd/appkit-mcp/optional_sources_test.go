package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
	"github.com/geoffmilleraz/app-kit/internal/twplus/normalize"
	"github.com/geoffmilleraz/app-kit/internal/twplus/parse"
)

// Exercise the real flag/env resolution, artifact loading, registration and stdio
// transport in a child process, isolated from the developer's licensed corpora.
func TestMCPStartupProcess(t *testing.T) {
	if os.Getenv("APPKIT_MCP_STARTUP_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"appkit-mcp"}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("appkit-mcp", flag.ExitOnError)
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func optionalSourceCommand(ctx context.Context, root string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestMCPStartupProcess$", "--"}, args...)...)
	cmd.Dir = root
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "APPKIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "APPKIT_MCP_STARTUP_HELPER=1")
	return cmd
}

func writeOptionalSourceFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := index.NewBuilder(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	comp, rows, err := normalize.Build(parse.Component{
		Path: []string{"Application UI", "Layout", "Dashboards", "Overview"},
		Raw: parse.RawComponent{Name: "Overview", Snippets: []parse.RawSnippet{
			{Code: `<div className="overview">Synthetic dashboard</div>`, Language: "jsx", Name: "react", Version: 3},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AddComponent(comp, rows); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Finalize(); err != nil {
		t.Fatal(err)
	}
}

func TestMCPOptionalSourcesStartup(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		docs, tailwindplus, matdash, marker bool
	}{
		{name: "guidance_without_licensed_data", docs: true},
		{name: "guidance_with_only_private_store_marker", docs: true, marker: true},
		{name: "both_sources_and_docs_absent"},
		{name: "matdash_only", docs: true, matdash: true},
		{name: "tailwindplus_only", docs: true, tailwindplus: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			var args []string
			if tc.docs {
				intent := filepath.Join(root, "docs", "intent", "README.md")
				if err := os.MkdirAll(filepath.Dir(intent), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(intent, []byte("# Synthetic kit intent\nChoose the consumer's applicable layers.\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				args = append(args, "-kit-root", root)
			}
			if tc.tailwindplus {
				writeOptionalSourceFixture(t, filepath.Join(root, defaultIndexRel))
			}
			if tc.marker {
				marker, err := os.ReadFile(filepath.Join("..", "..", designAssetsRel, "go.mod"))
				if err != nil {
					t.Fatal(err)
				}
				store := filepath.Join(root, designAssetsRel)
				if err := os.MkdirAll(store, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(store, "go.mod"), marker, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.matdash {
				writeOptionalSourceFixture(t, filepath.Join(root, defaultMatDashIndexRel))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := optionalSourceCommand(ctx, root, args...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			client := mcp.NewClient(&mcp.Implementation{Name: "optional-sources-test", Version: "test"}, nil)
			session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
			if err != nil {
				t.Fatalf("startup: %v\n%s", err, stderr.String())
			}
			defer session.Close()
			instructions := session.InitializeResult().Instructions
			if !tc.tailwindplus && !strings.Contains(instructions, "TailwindPlus corpus: unavailable") {
				t.Fatalf("instructions hide unavailable TailwindPlus: %s", instructions)
			}
			if !tc.matdash && !strings.Contains(instructions, "MatDash corpus: unavailable") {
				t.Fatalf("instructions hide unavailable MatDash: %s", instructions)
			}
			listed, err := session.ListTools(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			names := map[string]bool{}
			for _, tool := range listed.Tools {
				names[tool.Name] = true
				if strings.Contains(tool.Description, "React+Tailwind v4") || strings.Contains(tool.Description, "tailwind_version=4, mode=none") {
					t.Fatalf("stale MatDash version claim: %s", tool.Description)
				}
			}
			for _, name := range []string{"search_capabilities", "get_capability", "get_kit_guidance", "get_kit_decision", "search_components", "get_component", "list_components", "list_sections", "get_component_preview"} {
				if !names[name] {
					t.Errorf("expected tool %s unavailable", name)
				}
			}
			call := func(name string, args map[string]any, unavailable bool, want string) {
				t.Helper()
				result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
				if err != nil {
					t.Fatalf("%s protocol failure: %v", name, err)
				}
				if result.IsError != unavailable {
					t.Fatalf("%s isError=%v, want %v: %+v", name, result.IsError, unavailable, result)
				}
				var text strings.Builder
				for _, content := range result.Content {
					if c, ok := content.(*mcp.TextContent); ok {
						text.WriteString(c.Text)
					}
				}
				if !strings.Contains(text.String(), want) {
					t.Fatalf("%s missing %q: %s", name, want, text.String())
				}
				if !unavailable && result.StructuredContent == nil {
					t.Fatalf("%s omitted structured content", name)
				}
			}
			catalogResource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "kit://catalog"})
			if err != nil || len(catalogResource.Contents) == 0 || catalogResource.Contents[0].Text == "" {
				t.Fatalf("first-party catalog resource unavailable: %v, %+v", err, catalogResource)
			}
			call("search_capabilities", map[string]any{"query": "button"}, false, "")
			call("get_kit_guidance", nil, false, "app-kit")
			if tc.docs {
				call("get_kit_decision", map[string]any{"id": "intent"}, false, "Synthetic kit intent")
			}
			for _, source := range []struct {
				name      string
				available bool
				action    string
			}{
				{"tailwindplus", tc.tailwindplus, "appkit data build"},
				{"matdash", tc.matdash, "appkit data matdash-build"},
			} {
				want := source.action
				if source.available {
					want = "Overview"
				}
				call("search_components", map[string]any{"query": "overview", "source": source.name, "tailwind_version": 3}, !source.available, want)
				call("get_component", map[string]any{"full_name": "Application UI.Layout.Dashboards.Overview", "source": source.name, "tailwind_version": 3}, !source.available, want)
			}
			want := "appkit data build"
			if tc.tailwindplus {
				want = "Overview"
			}
			call("search_components", map[string]any{"query": "overview"}, !tc.tailwindplus, want)
			call("list_components", nil, !tc.tailwindplus, want)
			if tc.tailwindplus {
				want = "Application UI"
			}
			call("list_sections", nil, !tc.tailwindplus, want)
			call("get_component_preview", map[string]any{"full_name": "Overview"}, true, "appkit data previews")
			call("search_components", map[string]any{"query": "overview", "source": "typo"}, true, "unknown component source")
			call("get_component", map[string]any{"full_name": "Overview", "source": "typo"}, true, "unknown component source")
			// A handled source error must leave the session usable.
			call("get_kit_guidance", nil, false, "app-kit")
		})
	}
}

func TestMCPExplicitSourcePathFailures(t *testing.T) {
	for _, setting := range []struct{ flag, env, action string }{
		{"-index", "APPKIT_TWPLUS_INDEX", "appkit data build"},
		{"-matdash-index", "APPKIT_MATDASH_INDEX", "appkit data matdash-build"},
		{"-previews", "APPKIT_TWPLUS_PREVIEWS", "appkit data previews"},
		{"-embeddings", "APPKIT_TWPLUS_EMBEDDINGS", "appkit data embed"},
		{"-kit-root", "APPKIT_ROOT", "app-kit checkout directory"},
	} {
		for _, viaEnv := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/env=%v", setting.flag, viaEnv), func(t *testing.T) {
				root := t.TempDir()
				missing := filepath.Join(root, "missing.sqlite")
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := optionalSourceCommand(ctx, root, setting.flag, missing)
				if viaEnv {
					cmd = optionalSourceCommand(ctx, root)
					cmd.Env = append(cmd.Env, setting.env+"="+missing)
				}
				out, err := cmd.CombinedOutput()
				if err == nil {
					t.Fatalf("explicit missing path silently accepted: %s", out)
				}
				for _, want := range []string{missing, setting.flag, setting.action} {
					if !strings.Contains(string(out), want) {
						t.Errorf("error missing %q: %s", want, out)
					}
				}
			})
		}
	}
}

func TestOptionalSourceRejectsInvalidSQLite(t *testing.T) {
	for _, name := range []string{"empty", "corrupt", "directory"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.sqlite")
			if name == "directory" {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				var data []byte
				if name == "corrupt" {
					data = []byte("not a database")
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, explicit := range []bool{false, true} {
				ix, err := openOptionalArtifact(path, explicit, "test index", "-index", "appkit data build", openComponentIndex)
				if ix != nil || (err != nil) != explicit {
					t.Fatalf("explicit=%v: index=%v err=%v", explicit, ix, err)
				}
				pv, err := openOptionalArtifact(path, explicit, "test previews", "-previews", "appkit data previews", openPreviewStore)
				if pv != nil || (err != nil) != explicit {
					t.Fatalf("explicit=%v: previews=%v err=%v", explicit, pv, err)
				}
			}
		})
	}
	if got := serverInstructions(nil, nil, nil); !strings.Contains(got, "unavailable") {
		t.Fatalf("nil-source instructions: %s", got)
	}
}
