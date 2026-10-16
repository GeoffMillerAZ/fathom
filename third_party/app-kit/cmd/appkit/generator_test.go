package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

const (
	genDefaultInput    = "cue/tokens/kit.cue"
	genDefaultTerminal = "appkit/cli/theme/palette_gen.go"
)

type genWorkspace struct {
	kitDir       string
	customInput  string
	customSource []byte
	baseline     map[string][]byte
	sentinel     []byte
}

// Each test has a kit-like working directory and a sibling consumer. No
// generator invocation in these tests can write the checkout's artifacts.
func newGenWorkspace(t *testing.T) genWorkspace {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("../..", genDefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	baseline := make(map[string][]byte)
	for _, path := range []string{
		"appkit/ui/tokens/theme.css",
		"appkit/ui/tokens/tokens.dtcg.json",
		"appkit/ui/tokens/names_gen.go",
		genDefaultTerminal,
	} {
		baseline[path], err = os.ReadFile(filepath.Join("../..", path))
		if err != nil {
			t.Fatal(err)
		}
	}

	customSource := bytes.Replace(source, []byte(`"appkit"`), []byte(`"consumer-fixture"`), 1)
	customSource = bytes.Replace(customSource, []byte(`oklch(0.50 0.19 262)`), []byte(`oklch(0.60 0.10 140)`), 1)
	if !bytes.Contains(customSource, []byte(`"consumer-fixture"`)) || !bytes.Contains(customSource, []byte(`oklch(0.60 0.10 140)`)) {
		t.Fatal("default token fixture changed; update the distinct consumer fixture")
	}

	root := t.TempDir()
	w := genWorkspace{
		kitDir:       filepath.Join(root, "app-kit"),
		customInput:  filepath.Join(root, "consumer", "web", "css", "tokens.cue"),
		customSource: customSource,
		baseline:     baseline,
		sentinel:     []byte("// kit terminal palette sentinel: must not be overwritten by a consumer CSS build\n"),
	}
	writeGenFixture(t, filepath.Join(w.kitDir, genDefaultInput), source)
	writeGenFixture(t, w.customInput, customSource)
	for path := range baseline {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(w.kitDir, path)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeGenFixture(t, filepath.Join(w.kitDir, genDefaultTerminal), w.sentinel)
	t.Chdir(w.kitDir)
	return w
}

func writeGenFixture(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertGenFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s content differs: got %d bytes, want %d", path, len(got), len(want))
	}
}

func TestGenConsumerCSSDoesNotOverwriteKitTerminal(t *testing.T) {
	for _, defaultPresent := range []bool{true, false} {
		name := "default-source-present"
		if !defaultPresent {
			name = "default-source-absent"
		}
		t.Run(name, func(t *testing.T) {
			w := newGenWorkspace(t)
			if !defaultPresent {
				if err := os.Remove(genDefaultInput); err != nil {
					t.Fatal(err)
				}
			}
			cssPath := filepath.Join(filepath.Dir(w.customInput), "theme.css")
			// Exact argument shape used by consumer ui:tokens tasks with go run -C ../app-kit.
			if err := genTokens([]string{"-tokens", w.customInput, "-css", cssPath, "-dtcg", "", "-go", ""}); err != nil {
				t.Fatal(err)
			}
			assertGenFile(t, genDefaultTerminal, w.sentinel)
			custom, err := tokens.Load(w.customSource)
			if err != nil {
				t.Fatal(err)
			}
			assertGenFile(t, cssPath, []byte(tokens.EmitThemeCSS(custom)))
			for path := range w.baseline {
				if path == genDefaultTerminal {
					continue
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("unrequested kit output %s exists or cannot be inspected: %v", path, err)
				}
			}
		})
	}
}

func TestGenDefaultInputAliasesKeepImplicitTerminal(t *testing.T) {
	for _, alias := range []string{"relative", "clean-relative", "absolute", "clean-absolute", "symlink"} {
		t.Run(alias, func(t *testing.T) {
			w := newGenWorkspace(t)
			input := genDefaultInput
			switch alias {
			case "clean-relative":
				input = "./cue/tokens/../tokens/kit.cue"
			case "absolute":
				input = filepath.Join(w.kitDir, genDefaultInput)
			case "clean-absolute":
				input = w.kitDir + "/cue/tokens/../tokens/kit.cue"
			case "symlink":
				input = "kit-alias.cue"
				if err := os.Symlink(genDefaultInput, input); err != nil {
					t.Skipf("symlinks unavailable on this test platform: %v", err)
				}
			}
			if err := genTokens([]string{"-tokens", input, "-css", "", "-dtcg", "", "-go", ""}); err != nil {
				t.Fatal(err)
			}
			assertGenFile(t, genDefaultTerminal, w.baseline[genDefaultTerminal])
		})
	}
}

func TestGenCustomInputIdentityDoesNotUsePathSpelling(t *testing.T) {
	for _, alias := range []string{"same-basename", "symlink-parent"} {
		t.Run(alias, func(t *testing.T) {
			w := newGenWorkspace(t)
			input := filepath.Join(filepath.Dir(w.kitDir), "consumer", genDefaultInput)
			if alias == "symlink-parent" {
				// Cleaning this path lexically produces the default input path, but
				// the OS resolves it to a different file through the directory symlink.
				otherDir := filepath.Join(filepath.Dir(w.customInput), "child")
				if err := os.MkdirAll(otherDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(otherDir, "cue/tokens/link"); err != nil {
					t.Skipf("symlinks unavailable on this test platform: %v", err)
				}
				writeGenFixture(t, filepath.Join(filepath.Dir(w.customInput), "kit.cue"), w.customSource)
				input = "cue/tokens/link/../kit.cue"
			} else {
				writeGenFixture(t, input, w.customSource)
			}
			if err := genTokens([]string{"-tokens", input, "-css", "", "-dtcg", "", "-go", ""}); err != nil {
				t.Fatal(err)
			}
			assertGenFile(t, genDefaultTerminal, w.sentinel)
		})
	}
}

func TestGenExplicitTerminalIsAlwaysHonored(t *testing.T) {
	for _, target := range []string{"custom-output", "default-output", "skip-custom", "skip-default"} {
		t.Run(target, func(t *testing.T) {
			w := newGenWorkspace(t)
			input := w.customInput
			output := filepath.Join(filepath.Dir(w.customInput), "palette.go")
			switch target {
			case "default-output":
				output = genDefaultTerminal
			case "skip-custom":
				output = ""
			case "skip-default":
				input, output = genDefaultInput, ""
			}
			if err := genTokens([]string{"-tokens", input, "-css", "", "-dtcg", "", "-go", "", "-terminal=" + output, "-terminalpkg", "consumer"}); err != nil {
				t.Fatal(err)
			}
			if output != "" {
				custom, err := tokens.Load(w.customSource)
				if err != nil {
					t.Fatal(err)
				}
				want, err := tokens.EmitTerminalPalette(custom, "consumer")
				if err != nil {
					t.Fatal(err)
				}
				assertGenFile(t, output, want)
			}
			if output != genDefaultTerminal {
				assertGenFile(t, genDefaultTerminal, w.sentinel)
			}
		})
	}
}

func TestGenExplicitTerminalWriteFailureIsNotSuppressed(t *testing.T) {
	w := newGenWorkspace(t)
	output := filepath.Join("missing-directory", "palette.go")
	err := genTokens([]string{"-tokens", w.customInput, "-css", "", "-dtcg", "", "-go", "", "-terminal", output})
	if err == nil || !strings.Contains(err.Error(), "gen: write "+output) {
		t.Fatalf("explicit terminal output failure = %v; want actionable write error", err)
	}
	assertGenFile(t, genDefaultTerminal, w.sentinel)
}

func TestGenDefaultOutputsMatchCommittedArtifactsAndAreDeterministic(t *testing.T) {
	w := newGenWorkspace(t)
	for run := 0; run < 2; run++ {
		if err := genTokens(nil); err != nil {
			t.Fatal(err)
		}
		for path, want := range w.baseline {
			assertGenFile(t, path, want)
		}
	}
}
