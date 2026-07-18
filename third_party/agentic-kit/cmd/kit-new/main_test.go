package main

// Tests for kit-new: golden-file tests for both governed and minimal modes,
// and an integration test that generates a real module and runs `go build`.
//
// The integration test (TestIntegrationBuild) shells to the go tool — this is
// acceptable in cmd/ tests. It is gated by testing.Short() and sets:
//   - GOWORK=off (the generated dir has its own go.work but we test via replace)
//   - GOTOOLCHAIN=local (avoids any toolchain download)
//   - GOFLAGS unset (workspace mode conflicts with -mod flag)
//
// No network is required: the kit replace points at THIS checkout and the kit
// has no external Go dependencies.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// noIO discards all output in tests — we inspect return codes and files.
func runNew(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// ---- golden-file tests ------------------------------------------------------

// TestGoldenGoverned generates a governed module and compares every output
// file against the committed golden in testdata/golden/governed/.
func TestGoldenGoverned(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runNew(
		t,
		"-name", "myagent",
		"-dir", dir,
		"-module", "github.com/me/myagent",
		"-kit-version", "v0.0.0",
		"-kit-path", "../agentic-kit",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	goldenDir := filepath.Join("testdata", "golden", "governed")
	checkGoldenFiles(t, dir, goldenDir, []goldenFile{
		{generated: "go.mod", golden: "go.mod"},
		{generated: "go.work", golden: "go.work"},
		{generated: "main.go", golden: "main.go"},
		{generated: ".gitignore", golden: "gitignore.txt"},
		{generated: "README.md", golden: "README.md"},
	})
}

// TestGoldenMinimal generates a minimal module and compares output files.
func TestGoldenMinimal(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runNew(
		t,
		"-name", "myagent",
		"-dir", dir,
		"-module", "github.com/me/myagent",
		"-kit-version", "v0.0.0",
		"-kit-path", "../agentic-kit",
		"-minimal",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	goldenDir := filepath.Join("testdata", "golden", "minimal")
	checkGoldenFiles(t, dir, goldenDir, []goldenFile{
		{generated: "go.mod", golden: "go.mod"},
		{generated: "go.work", golden: "go.work"},
		{generated: "main.go", golden: "main.go"},
		{generated: ".gitignore", golden: "gitignore.txt"},
		{generated: "README.md", golden: "README.md"},
	})
}

type goldenFile struct {
	generated string // path relative to the generated dir
	golden    string // filename in the golden dir
}

// checkGoldenFiles asserts that every generated file matches its golden.
// If UPDATE_GOLDEN=1, the golden files are overwritten instead (for updates).
func checkGoldenFiles(t *testing.T, genDir, goldenDir string, files []goldenFile) {
	t.Helper()
	update := os.Getenv("UPDATE_GOLDEN") == "1"

	for _, f := range files {
		t.Run(f.generated, func(t *testing.T) {
			genPath := filepath.Join(genDir, f.generated)
			got, err := os.ReadFile(genPath)
			if err != nil {
				t.Fatalf("read generated %s: %v", genPath, err)
			}

			goldenPath := filepath.Join(goldenDir, f.golden)
			if update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("update golden %s: %v", goldenPath, err)
				}
				t.Logf("updated golden: %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with UPDATE_GOLDEN=1 to create)", goldenPath, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("generated %s does not match golden %s\n--- got ---\n%s\n--- want ---\n%s",
					f.generated, f.golden, got, want)
			}
		})
	}
}

// ---- content-shape tests ----------------------------------------------------

// TestGovernedContentShape checks that the governed main.go contains all the
// critical imports and identifiers that make it a fully wired governed agent.
func TestGovernedContentShape(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runNew(
		t,
		"-name", "myagent",
		"-dir", dir,
		"-module", "github.com/me/myagent",
		"-kit-version", "v0.0.0",
		"-kit-path", "../agentic-kit",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	main, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(main)

	requiredImports := []string{
		`"github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"`,
		`"github.com/geoffmilleraz/agentic-kit/foundations/security/injection-detector"`,
		`"github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"`,
		`"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"`,
		`"github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker"`,
		`"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/instrument"`,
	}
	for _, imp := range requiredImports {
		if !strings.Contains(src, imp) {
			t.Errorf("governed main.go missing import %q", imp)
		}
	}

	requiredIdents := []string{
		"BudgetMaxUSDPerDay",
		"budgetguard.New",
		"injectiondetector.NewDetector",
		"investigationjournal.NewJournalWithClock",
		"investigationjournal.JSONLSink",
		"journal.AttachSink",
		"sink.NewJSONLSink",
		"costtracker.New",
		"llminstrument.WithCost",
		"llminstrument.WithTelemetry",
		"llmclient.WithRetry",
		"agentharness.InjectionBlock",
		"journal.jsonl",
	}
	for _, ident := range requiredIdents {
		if !strings.Contains(src, ident) {
			t.Errorf("governed main.go missing %q", ident)
		}
	}
}

// TestMinimalContentShape checks that the minimal main.go has the opt-out
// comment block and does NOT wire the governance stack.
func TestMinimalContentShape(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runNew(
		t,
		"-name", "myagent",
		"-dir", dir,
		"-module", "github.com/me/myagent",
		"-kit-version", "v0.0.0",
		"-kit-path", "../agentic-kit",
		"-minimal",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	main, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(main)

	// Must have the loud opt-out comment.
	if !strings.Contains(src, "GOVERNANCE OPTED OUT via -minimal") {
		t.Error("minimal main.go missing opt-out comment block")
	}

	// Must NOT actually import the governance packages (the comment block names
	// them, but no import statement should reference them).
	governanceImportPaths := []string{
		`"github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"`,
		`"github.com/geoffmilleraz/agentic-kit/foundations/security/injection-detector"`,
		`"github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"`,
	}
	for _, imp := range governanceImportPaths {
		if strings.Contains(src, imp) {
			t.Errorf("minimal main.go should not have import statement %q", imp)
		}
	}
}

// TestDefaultModulePath verifies that -module is optional and defaults to
// "github.com/<name>/<name>".
func TestDefaultModulePath(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runNew(
		t,
		"-name", "coolbot",
		"-dir", dir,
		"-kit-version", "v0.0.0",
		"-kit-path", "../agentic-kit",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gomod), "module github.com/coolbot/coolbot") {
		t.Errorf("go.mod missing default module path:\n%s", gomod)
	}
}

// TestNameSubstitution verifies that the agent name is substituted everywhere.
func TestNameSubstitution(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runNew(
		t,
		"-name", "myrocket",
		"-dir", dir,
		"-module", "github.com/me/myrocket",
		"-kit-version", "v0.0.0",
		"-kit-path", "../agentic-kit",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	main, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(main)

	// Name appears in the binary header comment and flag set name.
	if !strings.Contains(src, "Command myrocket") {
		t.Error("main.go missing 'Command myrocket'")
	}
	if !strings.Contains(src, `flag.NewFlagSet("myrocket"`) {
		t.Error("main.go missing flag.NewFlagSet for 'myrocket'")
	}
}

// TestWorkspaceDir verifies that the workspace/ directory is created.
func TestWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runNew(
		t,
		"-name", "myagent",
		"-dir", dir,
		"-module", "github.com/me/myagent",
		"-kit-version", "v0.0.0",
		"-kit-path", "../agentic-kit",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	ws := filepath.Join(dir, "workspace")
	st, err := os.Stat(ws)
	if err != nil {
		t.Fatalf("workspace/ not created: %v", err)
	}
	if !st.IsDir() {
		t.Error("workspace/ is not a directory")
	}
}

// ---- usage error tests ------------------------------------------------------

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"missing name", []string{"-dir", "/tmp/x"}, 2},
		{"missing dir", []string{"-name", "foo"}, 2},
		{"name with slash", []string{"-name", "foo/bar", "-dir", "/tmp/x"}, 2},
		{"name with space", []string{"-name", "foo bar", "-dir", "/tmp/x"}, 2},
		{"unknown flag", []string{"-definitely-not-a-flag"}, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := runNew(t, tc.args...)
			if code != tc.want {
				t.Errorf("exit %d, want %d; stderr: %s", code, tc.want, stderr)
			}
		})
	}
}

// TestRefusesExistingGoMod verifies that kit-new refuses to clobber a module.
func TestRefusesExistingGoMod(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runNew(
		t,
		"-name", "myagent",
		"-dir", dir,
		"-module", "github.com/me/myagent",
		"-kit-version", "v0.0.0",
		"-kit-path", "../agentic-kit",
	)
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "go.mod already exists") {
		t.Errorf("stderr missing 'go.mod already exists': %s", stderr)
	}
}

// TestKitVersionFlag verifies the -kit-version flag appears in go.mod.
func TestKitVersionFlag(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runNew(
		t,
		"-name", "myagent",
		"-dir", dir,
		"-module", "github.com/me/myagent",
		"-kit-version", "v0.3.0",
		"-kit-path", "../agentic-kit",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gomod), "v0.3.0") {
		t.Errorf("go.mod does not contain kit version v0.3.0:\n%s", gomod)
	}
}

// ---- integration build test -------------------------------------------------

// TestIntegrationBuild generates a module into t.TempDir(), then runs
// `go build` in it with an absolute replace path pointing at THIS checkout.
// Guards:
//   - testing.Short() skip (shells to go tool)
//   - GOWORK=off (avoid workspace mode conflicts)
//   - GOTOOLCHAIN=local (avoid toolchain downloads)
//   - no network needed: the kit replace is a local path and kit has no deps
func TestIntegrationBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: skipping go build test in short mode")
	}

	// Resolve the kit repo root from this file's location.
	// __file__ is this test file; the kit root is ../../.. from cmd/kit-new/.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine this file's path")
	}
	kitRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	for _, tc := range []struct {
		name    string
		minimal bool
	}{
		{"governed", false},
		{"minimal", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			args := []string{
				"-name", "myagent",
				"-dir", dir,
				"-module", "github.com/me/myagent",
				"-kit-version", "v0.0.0",
				"-kit-path", kitRoot,
			}
			if tc.minimal {
				args = append(args, "-minimal")
			}

			code, _, stderr := runNew(t, args...)
			if code != 0 {
				t.Fatalf("generation failed (exit %d): %s", code, stderr)
			}

			// `go build` in the generated dir.
			// GOWORK=off: use the go.mod replace, not the workspace.
			// GOTOOLCHAIN=local: no downloads.
			cmd := exec.Command("go", "build", ".")
			cmd.Dir = dir
			cmd.Env = append(
				filterEnv(os.Environ(), "GOWORK", "GOFLAGS"),
				"GOWORK=off",
				"GOTOOLCHAIN=local",
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go build failed:\n%s", out)
			}
		})
	}
}

// filterEnv returns a copy of env with all entries whose key is in exclude removed.
func filterEnv(env []string, exclude ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, ex := range exclude {
			if strings.HasPrefix(e, ex+"=") {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, e)
		}
	}
	return out
}
