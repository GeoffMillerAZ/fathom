package main

import (
	"bytes"
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCSSSetupGuideContract(t *testing.T) {
	g := cssSetupGuide()
	// The non-negotiable facts a consumer needs to get rendering right.
	for _, want := range []string{
		"No Node is required by this kit-owned build",
		"appkit gen", // theme layer generation
		"appkit css", // pinned-binary build
		"@import \"tailwindcss\"",
		"theme.css",   // the generated layer
		"@source",     // scanning requirement (the #1 inert-utility cause)
		"data-theme",  // light/dark contract
		"--ak-color-", // the variable layer roles resolve through
		"rounded-",    // radius vocabulary
		"font-",       // font vocabulary
		"declared React/client toolchain",
		"L3b/L4",
		"GOWORK=off go list -mod=readonly -m -f '{{.Dir}}' github.com/geoffmilleraz/app-kit",
		"including their rendering dependencies",
		"Tailwind itself can silently miss absent sources",
		"never check in absolute module-cache paths",
		"Local replacements are development-only evidence",
		"Preserve originals",
		"preserves original classes",
		"do not delete `dark:` or hover/focus/contrast distinctions automatically",
		"replacing Tailwind's default OS-based variant",
		"Build success is not source/asset proof",
		"Every scanned renderer file must also be an input to the incremental CSS task",
		"scanner manifest, generated theme CSS, input CSS and pinned compiler identity",
		"Add a new source file with a new utility class",
		"generated selector and browser-computed wrapping and contrast",
		"checksum is verified on every invocation",
		"@import \"tailwindcss\" source(none);",
		"disables implicit checkout scanning",
		"do not add those dependencies to the application merely to generate CSS",
		"font/radius aliases",
		"two isolated consumer roots with the same declared inputs",
		"Removing a selected source must fail a required-selector check",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("CSS setup guide missing %q", want)
		}
	}
	for _, stale := range []string{
		"No Node, ever", "appkit gen -tokens app/ui/tokens.cue -out", "task css:build",
		"drops `dark:`", "you get it for free", "map those to the nearest kit step",
		"../../../appkit/ui/motion/motion.css", "sha256-verifies once",
		"@import \"tailwindcss\";",
	} {
		if strings.Contains(g, stale) {
			t.Errorf("CSS setup guide retains misleading instruction %q", stale)
		}
	}
}

func TestCSSSetupGuideSelectedSources(t *testing.T) {
	if !strings.Contains(cssSetupGuide(), cssSetupSourceScript()) {
		t.Fatal("tested scanner preflight is not rendered in the guide")
	}
	for _, tc := range []struct {
		name, suffix string
		exists, pass bool
	}{
		{name: "selected_package", suffix: "module cache with spaces", exists: true, pass: true},
		{name: "missing_package", suffix: "module-cache"},
		{name: "quote_in_path", suffix: "module\"cache", exists: true},
		{name: "backslash_in_path", suffix: "module\\cache", exists: true},
		{name: "newline_in_path", suffix: "module\ncache", exists: true},
		{name: "unresolved_module"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "app/ui"), 0o700); err != nil {
				t.Fatal(err)
			}
			moduleDir := ""
			if tc.suffix != "" {
				moduleDir = filepath.Join(root, tc.suffix)
			}
			if tc.exists {
				if err := os.MkdirAll(filepath.Join(moduleDir, "appkit/ui/card"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", cssSetupSourceScript())
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "APPKIT_CSS_MODULE_DIR="+moduleDir)
			out, err := cmd.CombinedOutput()
			if tc.pass {
				if err != nil {
					t.Fatalf("selected source preflight failed: %v\n%s", err, out)
				}
				want := "@source \"" + filepath.ToSlash(moduleDir) + "/appkit/ui/card\";\n"
				if got := readFile(t, filepath.Join(root, "app/ui/kit.sources.css")); got != want {
					t.Fatalf("scanner fragment got %q, want %q", got, want)
				}
			} else {
				if err == nil {
					t.Fatal("invalid/missing source directory silently accepted")
				}
				if _, err := os.Stat(filepath.Join(root, "app/ui/kit.sources.css")); !os.IsNotExist(err) {
					t.Fatalf("failed preflight created scanner fragment: %v", err)
				}
			}
		})
	}
}

// TestCSSSetupGuideCoversMotion — the CSS setup guide is the agent-facing "how
// do I wire the kit's CSS in" answer, and the motion layer wires in the same
// way (an @import after the theme) but with one crucial difference: it needs no
// @source entry. Without this section an agent adapting a component finds the
// role utilities and never learns the motion layer exists, or wastes time
// hunting for the @source glob it does not need.
func TestCSSSetupGuideCoversMotion(t *testing.T) {
	g := cssSetupGuide()
	for _, want := range []string{
		"motion.css", // the import line
		"ak-motion",  // the class vocabulary
		"@source",    // the "no @source needed" clarification is stated here
		"RootClass",  // the Go helpers
		"appkit vendor -out app/ui/vendor/appkit-motion --include motion\n",
		"appkit vendor -out app/ui/vendor/appkit-motion --include motion --check",
		"kit.lock",
		"@import \"./vendor/appkit-motion/motion.css\"",
		"motion.CSSHandler()",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("CSS setup guide has no motion content: missing %q", want)
		}
	}
}

// TestCSSSetupGuideAdvertisesRealVocabulary guards drift: every color role,
// radius name, and font name the guide enumerates must come from the generated
// token vocabulary, and all of them must appear. If a role is renamed in CUE,
// the guide changes with it (it reads tokens.* directly) — this asserts the
// wiring stays intact.
func TestCSSSetupGuideAdvertisesRealVocabulary(t *testing.T) {
	g := cssSetupGuide()
	for _, r := range tokens.ColorNames {
		if !strings.Contains(g, "`"+r+"`") {
			t.Errorf("guide omits color role %q", r)
		}
	}
	for _, r := range tokens.RadiusNames {
		if !strings.Contains(g, "`rounded-"+r+"`") {
			t.Errorf("guide omits radius utility rounded-%s", r)
		}
	}
	for _, f := range tokens.FontNames {
		if !strings.Contains(g, "`font-"+f+"`") {
			t.Errorf("guide omits font utility font-%s", f)
		}
	}
}

// Execute the same typed arguments rendered into the guide, against the actual
// appkit executable. No Tailwind download or licensed/consumer input is needed.
func TestCSSSetupGuideGenerationRecipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	kitRoot, err := filepath.Abs(kitRootRel)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "appkit")
	env := append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=")
	build := exec.CommandContext(ctx, "go", "build", "-mod=readonly", "-o", bin, "./cmd/appkit")
	build.Dir, build.Env = kitRoot, env
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual appkit CLI: %v\n%s", err, out)
	}
	guide := cssSetupGuide()
	if !strings.Contains(guide, cssSetupGenCommand()+" || exit 1\n") || !strings.Contains(guide, "mkdir -p "+strings.Join(cssSetupOutputDirs(), " ")+" || exit 1\n") {
		t.Fatal("executable recipe and directory prerequisites are not rendered in the guide")
	}
	args := cssSetupGenArgs()
	if args[len(args)-2] != "-terminal" || args[len(args)-1] != "" || !strings.HasSuffix(cssSetupGenCommand(), "-terminal ''") {
		t.Fatal("web recipe must explicitly disable terminal output, including shell quoting")
	}
	for _, arg := range args {
		if arg == "-out" {
			t.Fatal("gen recipe contains the nonexistent -out flag")
		}
	}

	const light = "oklch(0.91 0.04 270)"
	const dark = "oklch(0.21 0.04 270)"
	fixture := tokens.Tokens{
		Name: "css-setup-fixture", Fonts: tokens.Fonts{Sans: "Fixture Sans, sans-serif", Mono: "monospace"},
		Radius: tokens.Radius{SM: "2px", MD: "5px", LG: "9px", XL: "13px", Full: "9999px"},
		Colors: map[string]tokens.ColorPair{},
	}
	for _, role := range tokens.ColorNames {
		fixture.Colors[role] = tokens.ColorPair{Light: light, Dark: dark}
	}
	fixture.Colors["setup-canary"] = tokens.ColorPair{Light: light, Dark: dark}
	source, err := json.Marshal(map[string]any{"tokens": fixture}) // JSON is valid CUE input.
	if err != nil {
		t.Fatal(err)
	}
	validated, err := tokens.Load(source)
	if err != nil || validated.Name != fixture.Name || validated.Colors["setup-canary"] != fixture.Colors["setup-canary"] {
		t.Fatalf("synthetic CUE fixture invalid or changed: %v", err)
	}
	want := map[string][]byte{
		"app/ui/tokens/theme.css":        []byte(tokens.EmitThemeCSS(validated)),
		"app/ui/tokens/tokens.dtcg.json": tokens.EmitDTCG(validated),
		"app/ui/tokens/names_gen.go":     tokens.EmitGoNames(validated, "tokens"),
	}
	for _, tc := range []struct {
		name    string
		body    []byte
		invalid bool
	}{
		{name: "valid", body: source},
		{name: "invalid_cue", body: []byte("tokens: {name: 42}"), invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range cssSetupOutputDirs() {
				if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "app/ui/tokens.cue"), tc.body, 0o600); err != nil {
				t.Fatal(err)
			}
			terminal := filepath.Join(root, "appkit/cli/theme/palette_gen.go")
			if err := os.MkdirAll(filepath.Dir(terminal), 0o700); err != nil {
				t.Fatal(err)
			}
			const sentinel = "// Existing terminal palette must remain untouched.\n"
			if err := os.WriteFile(terminal, []byte(sentinel), 0o600); err != nil {
				t.Fatal(err)
			}
			for run := 0; run < 2; run++ {
				cmd := exec.CommandContext(ctx, bin, cssSetupGenArgs()...)
				cmd.Dir, cmd.Env = root, env
				out, err := cmd.CombinedOutput()
				if tc.invalid {
					if err == nil {
						t.Fatal("invalid CUE unexpectedly generated artifacts")
					}
					if !strings.Contains(string(out), "token set does not satisfy the schema") {
						t.Fatalf("invalid fixture did not reach CUE validation: %v\n%s", err, out)
					}
				} else if err != nil {
					t.Fatalf("guide's appkit gen command failed: %v\n%s", err, out)
				}
				for path, expected := range want {
					body, err := os.ReadFile(filepath.Join(root, path))
					if tc.invalid {
						if !os.IsNotExist(err) {
							t.Errorf("invalid source emitted %s: %v", path, err)
						}
					} else if err != nil || !bytes.Equal(body, expected) {
						t.Errorf("run %d: %s differs from validated token output: %v", run, path, err)
					}
				}
				if body, err := os.ReadFile(terminal); err != nil || string(body) != sentinel {
					t.Fatalf("web generation changed default terminal palette: %v", err)
				}
			}
			if tc.invalid {
				return
			}
			css := readFile(t, filepath.Join(root, "app/ui/tokens/theme.css"))
			for _, value := range []string{"--ak-color-setup-canary: " + light, "--ak-color-setup-canary: " + dark, "--ak-radius-lg: 9px", "--radius-lg: var(--ak-radius-lg)"} {
				if !strings.Contains(css, value) {
					t.Errorf("generated CSS lost explicit fixture value %q", value)
				}
			}
			var dtcg struct {
				Name  string `json:"name"`
				Color map[string]struct {
					Value      string                       `json:"$value"`
					Extensions map[string]map[string]string `json:"$extensions"`
				} `json:"color"`
			}
			if err := json.Unmarshal([]byte(readFile(t, filepath.Join(root, "app/ui/tokens/tokens.dtcg.json"))), &dtcg); err != nil {
				t.Fatal(err)
			}
			if dtcg.Name != fixture.Name || dtcg.Color["setup-canary"].Value != light || dtcg.Color["setup-canary"].Extensions["com.appkit.scheme"]["dark"] != dark {
				t.Fatal("DTCG did not preserve the named fixture and both scheme values")
			}
			generated := readFile(t, filepath.Join(root, "app/ui/tokens/names_gen.go"))
			if _, err := parser.ParseFile(token.NewFileSet(), "names_gen.go", generated, parser.AllErrors); err != nil {
				t.Fatalf("generated role constants are not valid Go: %v", err)
			}
			if !strings.Contains(generated, `ColorSetupCanary = "setup-canary"`) {
				// gofmt aligns constants; compare words, not padding.
				if !strings.Contains(strings.Join(strings.Fields(generated), " "), `ColorSetupCanary = "setup-canary"`) {
					t.Error("generated Go constants omit the custom fixture role")
				}
			}
		})
	}
	t.Run("generation_failure_stops_css_build", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "app/ui/tokens"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "app/ui/tokens.cue"), []byte("tokens: {name: 42}"), 0o600); err != nil {
			t.Fatal(err)
		}
		const previousTheme = "/* Existing stale theme must not be rebuilt after invalid token input. */\n"
		theme := filepath.Join(root, "app/ui/tokens/theme.css")
		if err := os.WriteFile(theme, []byte(previousTheme), 0o600); err != nil {
			t.Fatal(err)
		}
		_, remainder, ok := strings.Cut(guide, "```sh\n")
		if !ok {
			t.Fatal("guide lacks executable build block")
		}
		block, _, ok := strings.Cut(remainder, "```\n")
		if !ok {
			t.Fatal("guide's build block is not closed")
		}
		// Generation is the real binary. Only attempted CSS invocation is replaced
		// with a marker, so this negative canary cannot fetch/run Tailwind.
		const dispatch = `appkit() {
  if [ "$1" = gen ]; then
    "$APPKIT_SETUP_TEST_BIN" "$@"
  elif [ "$1" = css ]; then
    printf '%s\n' 'CSS invoked after failed generation' > css-invoked
  else
    return 2
  fi
}
`
		cmd := exec.CommandContext(ctx, "sh", "-c", dispatch+block)
		cmd.Dir = root
		cmd.Env = append(env, "APPKIT_SETUP_TEST_BIN="+bin)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "token set does not satisfy the schema") {
			t.Fatalf("guide block did not fail at CUE validation: %v\n%s", err, out)
		}
		if _, err := os.Stat(filepath.Join(root, "css-invoked")); !os.IsNotExist(err) {
			t.Fatalf("guide continued to CSS build after generation failure: %v", err)
		}
		if got := readFile(t, theme); got != previousTheme {
			t.Fatal("failed token generation changed the previous theme")
		}
	})
}

func TestCSSSetupGuideMCPParity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv := mcp.NewServer(&mcp.Implementation{Name: "css-setup-test", Version: "test"}, nil)
	registerKitTools(srv, loadKitDocs(t.TempDir())) // No checkout docs or licensed inputs required.
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "css-setup-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_kit_css_setup", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("CSS setup tool failed: %v", result.GetError())
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != cssSetupGuide() {
		t.Fatal("MCP text differs from the tested CSS setup guide")
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var structured docOutput
	if err := json.Unmarshal(body, &structured); err != nil {
		t.Fatal(err)
	}
	if structured.Title != "app-kit CSS setup" || structured.Body != cssSetupGuide() {
		t.Fatal("MCP structured content differs from the tested CSS setup guide")
	}
}
