package designadapt

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Keep native HTTP safety in the ordinary source-only gate. This uses an authored
// synthetic form, real model/view helpers and real handler tests, never paid files.
// It proves handler semantics, not source styling or actual browser behavior.
func TestCatalystSyntheticHTTPFixture(t *testing.T) {
	generated, err := adaptCatalystForm([]byte(syntheticCatalystForm()))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd/server"), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, body []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.templ", []byte(generated))
	write("go.mod", []byte("module example.test/p10\n\ngo 1.26.2\n\nrequire github.com/a-h/templ v0.3.1020\n"))
	write("cmd/server/appearance.go", []byte("package main\nconst sourceErrorClass = \"synthetic-error\"\n"))
	for name, source := range map[string]string{
		"go.sum":    filepath.Join(root, "go.sum"),
		"policy.go": "catalyst_model.go.txt", "view.go": "catalyst_view.go.txt",
		"catalyst_native.go": "catalyst_native.go.txt", "catalyst_native.css": "catalyst_native.css",
		"cmd/server/server.go":      filepath.Join(root, "docs/assessments/fixtures/catalyst-settings-adapter/server.go.txt"),
		"cmd/server/server_test.go": filepath.Join(root, "docs/assessments/fixtures/catalyst-settings-adapter/server_test.go.txt"),
		"cmd/server/controller.mjs": "catalyst_enhance.mjs",
	} {
		code, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		write(name, code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	run := func(cwd string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("source-only native fixture: %v\n%s", err, output)
		}
	}
	run(dir, "tool", "-modfile", filepath.Join(root, "go.mod"), "templ", "generate", "-f", "settings.templ", "-include-version=false")
	generatedGo, err := os.ReadFile(filepath.Join(dir, "settings_templ.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkCatalystGeneratedLocations(generatedGo); err != nil {
		t.Fatal(err)
	}
	run(dir, "test", "-race", "-count=1", "./...")
}
