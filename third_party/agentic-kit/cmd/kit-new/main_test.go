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
	"errors"
	"io"
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

func TestScaffoldRefusesEveryExistingOutputBeforeWriting(t *testing.T) {
	for _, name := range []string{"go.mod", "go.work", "main.go", ".gitignore", "README.md", "workspace"} {
		for _, kind := range []string{"file", "directory", "symlink", "dangling-symlink"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, name)
				unrelated := filepath.Join(dir, "keep.txt")
				if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "file":
					if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				case "symlink", "dangling-symlink":
					target := unrelated
					if kind == "dangling-symlink" {
						target = filepath.Join(dir, "absent")
					}
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				code, _, stderr := runNew(t, "-name", "myagent", "-dir", dir)
				if code != 1 {
					t.Errorf("exit %d, want 1: %s", code, stderr)
				}
				after, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(before, after) {
					t.Errorf("existing output replaced: %s", path)
				}
				for _, output := range []string{"go.mod", "go.work", "main.go", ".gitignore", "README.md", "workspace"} {
					if output == name {
						continue
					}
					if _, err := os.Lstat(filepath.Join(dir, output)); !os.IsNotExist(err) {
						t.Errorf("created %s before refusing %s: %v", output, name, err)
					}
				}
				if got, err := os.ReadFile(unrelated); err != nil || string(got) != "keep" {
					t.Errorf("unrelated file changed: %q, %v", got, err)
				}
				if kind == "file" {
					got, err := os.ReadFile(path)
					if err != nil || string(got) != "original" {
						t.Errorf("existing bytes changed: %q, %v", got, err)
					}
				}
			})
		}
	}
}

func TestScaffoldExclusiveCreateAndOwnedRollback(t *testing.T) {
	t.Run("late collision", func(t *testing.T) {
		dir := t.TempDir()
		want := []byte("arrived after preflight")
		err := generateWithStep(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, func(name string) error {
			if name == "workspace" {
				return os.WriteFile(filepath.Join(dir, "go.mod"), want, 0o600)
			}
			return nil
		})
		if err == nil {
			t.Fatal("late collision succeeded")
		}
		got, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("late file changed: %q, %v", got, err)
		}
		if _, err := os.Lstat(filepath.Join(dir, "workspace")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("owned workspace survived: %v", err)
		}
	})
	t.Run("replacement and unrelated", func(t *testing.T) {
		dir := t.TempDir()
		unrelated := filepath.Join(dir, "keep")
		if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := generateWithStep(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, func(name string) error {
			if name != "main.go" {
				return nil
			}
			path := filepath.Join(dir, name)
			if err := os.Remove(path); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
				return err
			}
			return errors.New("injected write failure")
		})
		if err == nil {
			t.Fatal("injected failure succeeded")
		}
		for name, want := range map[string]string{"main.go": "replacement", "keep": "keep"} {
			got, readErr := os.ReadFile(filepath.Join(dir, name))
			if readErr != nil || string(got) != want {
				t.Errorf("%s changed: %q, %v", name, got, readErr)
			}
		}
		for _, name := range []string{"go.mod", "go.work", "workspace"} {
			if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("owned %s survived: %v", name, err)
			}
		}
	})
}

func TestScaffoldRollbackRetainsChangedOutputs(t *testing.T) {
	for _, stage := range []string{"same-byte replacement", "workspace replacement", "in-place edit", "chmod"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			target := "main.go"
			if stage == "workspace replacement" {
				target = "workspace"
			}
			var replacement []byte
			stop := errors.New("stop")
			err := generateWithStep(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, func(name string) error {
				if name != target {
					return nil
				}
				path := filepath.Join(dir, name)
				switch stage {
				case "same-byte replacement":
					var readErr error
					replacement, readErr = os.ReadFile(path)
					if readErr != nil {
						return readErr
					}
					if err := os.Remove(path); err != nil {
						return err
					}
					if err := os.WriteFile(path, replacement, 0o644); err != nil {
						return err
					}
				case "workspace replacement":
					if err := os.Remove(path); err != nil {
						return err
					}
					if err := os.Mkdir(path, 0o755); err != nil {
						return err
					}
				case "in-place edit":
					f, err := os.OpenFile(path, os.O_WRONLY, 0)
					if err != nil {
						return err
					}
					_, err = f.WriteAt([]byte("changed"), 0)
					return errors.Join(err, f.Close(), stop)
				case "chmod":
					// Windows observes only the owner-write bit, so clear it too.
					if err := os.Chmod(path, 0o400); err != nil {
						return err
					}
					t.Cleanup(func() {
						if err := os.Chmod(path, 0o600); err != nil {
							t.Errorf("restore output mode: %v", err)
						}
					})
				}
				return stop
			})
			if !errors.Is(err, stop) || !strings.Contains(err.Error(), "retained "+filepath.Join(dir, target)) {
				t.Fatalf("missing retained-path error: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(dir, target)); err != nil {
				t.Fatalf("changed output removed: %v", err)
			}
			if stage == "same-byte replacement" {
				got, err := os.ReadFile(filepath.Join(dir, target))
				if err != nil || !bytes.Equal(got, replacement) {
					t.Fatalf("replacement bytes changed: %v", err)
				}
			}
		})
	}
}

func TestScaffoldPinFailureRetainsUnknownOutput(t *testing.T) {
	for _, target := range []string{"workspace", "go.work"} {
		t.Run(target, func(t *testing.T) {
			dir := t.TempDir()
			failure := errors.New("pin unavailable")
			hooks := scaffoldHooks{}
			if target == "workspace" {
				hooks.openDirPin = func(*os.Root, string) (*os.Root, error) { return nil, failure }
			} else {
				hooks.openFilePin = func(root *os.Root, name string) (*os.File, error) {
					if name == target {
						return nil, failure
					}
					return root.Open(name)
				}
			}
			err := generateWithHooks(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, hooks)
			if !errors.Is(err, failure) || !strings.Contains(err.Error(), "retained "+filepath.Join(dir, target)) {
				t.Fatalf("missing retained-path error: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(dir, target)); err != nil {
				t.Fatalf("unknown output removed: %v", err)
			}
		})
	}
}

func TestScaffoldInvalidAndShortWriteCounts(t *testing.T) {
	for _, stage := range []string{"negative", "excess", "short"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			hooks := scaffoldHooks{write: func(f *os.File, data []byte) (int, error) {
				switch stage {
				case "negative":
					return -1, nil
				case "excess":
					return len(data) + 1, nil
				default:
					return f.Write(data[:len(data)-1])
				}
			}}
			err := generateWithHooks(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, hooks)
			if err == nil {
				t.Fatal("invalid write succeeded")
			}
			if stage == "short" {
				if !errors.Is(err, io.ErrShortWrite) {
					t.Fatalf("missing short write: %v", err)
				}
				if _, statErr := os.Lstat(filepath.Join(dir, "go.mod")); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("known short write survived: %v", statErr)
				}
			} else if _, statErr := os.Lstat(filepath.Join(dir, "go.mod")); statErr != nil {
				t.Fatalf("unknown write count removed: %v", statErr)
			}
		})
	}
}

func TestScaffoldPinsCloseAfterSuccessAndRollback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "rollback"}[fail], func(t *testing.T) {
			dir := t.TempDir()
			var files []*os.File
			var dirs []*os.Root
			hooks := scaffoldHooks{
				openFilePin: func(root *os.Root, name string) (*os.File, error) {
					f, err := root.Open(name)
					if err == nil {
						files = append(files, f)
					}
					return f, err
				},
				openDirPin: func(root *os.Root, name string) (*os.Root, error) {
					r, err := root.OpenRoot(name)
					if err == nil {
						dirs = append(dirs, r)
					}
					return r, err
				},
			}
			if fail {
				hooks.afterCreate = func(name string) error {
					if name == "main.go" {
						return errors.New("stop")
					}
					return nil
				}
			}
			err := generateWithHooks(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, hooks)
			if (err != nil) != fail {
				t.Fatalf("unexpected result: %v", err)
			}
			wantFiles := 5
			if fail {
				wantFiles = 3
			}
			if len(files) != wantFiles || len(dirs) != 1 {
				t.Fatalf("pins files=%d dirs=%d", len(files), len(dirs))
			}
			for _, f := range files {
				if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Errorf("file pin still open: %v", err)
				}
			}
			for _, r := range dirs {
				if _, err := r.Stat("."); !errors.Is(err, os.ErrClosed) {
					t.Errorf("directory pin still open: %v", err)
				}
			}
		})
	}
}

func TestScaffoldMetadataFailureRetainsUnknownIdentity(t *testing.T) {
	for _, target := range []string{"workspace", "go.work"} {
		t.Run(target, func(t *testing.T) {
			dir := t.TempDir()
			failure := errors.New("injected metadata failure")
			hooks := scaffoldHooks{}
			if target == "workspace" {
				hooks.workspaceInfo = func(*os.Root) (os.FileInfo, error) { return nil, failure }
			} else {
				calls := 0
				hooks.fileInfo = func(f *os.File) (os.FileInfo, error) {
					calls++
					if calls == 2 {
						return nil, failure
					}
					return f.Stat()
				}
			}
			err := generateWithHooks(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, hooks)
			if !errors.Is(err, failure) || !strings.Contains(err.Error(), "retained "+filepath.Join(dir, target)) {
				t.Fatalf("missing retained-path error: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(dir, target)); err != nil {
				t.Fatalf("unknown-identity output removed: %v", err)
			}
			for _, name := range []string{"go.mod", "go.work", "main.go", ".gitignore", "README.md", "workspace"} {
				if name == target {
					continue
				}
				if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("unexpected %s: %v", name, err)
				}
			}
		})
	}
}

func TestScaffoldWriteAndCloseFailureRollBackKnownOutputs(t *testing.T) {
	for _, stage := range []string{"write", "close"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			failure := errors.New("injected " + stage + " failure")
			hooks := scaffoldHooks{}
			calls := 0
			if stage == "write" {
				hooks.write = func(f *os.File, data []byte) (int, error) {
					calls++
					if calls == 2 {
						return 0, failure
					}
					return f.Write(data)
				}
			} else {
				hooks.close = func(f *os.File) error {
					calls++
					err := f.Close()
					if calls == 2 && err == nil {
						return failure
					}
					return err
				}
			}
			err := generateWithHooks(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, hooks)
			if !errors.Is(err, failure) {
				t.Fatalf("missing failure: %v", err)
			}
			for _, name := range []string{"go.mod", "go.work", "main.go", ".gitignore", "README.md", "workspace"} {
				if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("owned %s survived: %v", name, err)
				}
			}
		})
	}
}

func TestScaffoldSetupFailureReportsRetainedDestination(t *testing.T) {
	for _, stage := range []string{"parent", "dir-stat", "open-root"} {
		t.Run(stage, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "nested", "scaffold")
			failure := errors.New("injected " + stage + " failure")
			hooks := scaffoldHooks{}
			switch stage {
			case "parent":
				hooks.mkdirParent = func(parent string) error {
					if err := os.MkdirAll(parent, 0o755); err != nil {
						return err
					}
					return failure
				}
			case "dir-stat":
				hooks.dirInfo = func(string) (os.FileInfo, error) { return nil, failure }
			case "open-root":
				hooks.openRoot = func(string) (*os.Root, error) { return nil, failure }
			}
			err := generateWithHooks(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, hooks)
			if !errors.Is(err, failure) {
				t.Fatalf("missing failure: %v", err)
			}
			wantPath := dir
			if stage == "parent" {
				wantPath = filepath.Dir(dir)
			}
			if !strings.Contains(err.Error(), wantPath) {
				t.Errorf("missing absolute path %s: %v", wantPath, err)
			}
			if _, statErr := os.Lstat(wantPath); statErr != nil {
				t.Errorf("injected retained path missing: %v", statErr)
			}
		})
	}
}

func TestScaffoldCleanupFailureReportsRetainedPath(t *testing.T) {
	for _, stage := range []string{"output", "root-dir"} {
		t.Run(stage, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "scaffold")
			failure := errors.New("injected cleanup failure")
			hooks := scaffoldHooks{afterCreate: func(name string) error {
				if stage == "output" && name == "go.mod" {
					return errors.New("stop")
				}
				if stage == "root-dir" && name == "workspace" {
					return errors.New("stop")
				}
				return nil
			}}
			wantPath := dir
			if stage == "output" {
				wantPath = filepath.Join(dir, "go.mod")
				hooks.removeOutput = func(root *os.Root, name string) error {
					if name == "go.mod" {
						return failure
					}
					return root.Remove(name)
				}
			} else {
				hooks.removeDir = func(string) error { return failure }
			}
			err := generateWithHooks(scaffold{name: "myagent", dir: dir, kitVersion: "v0.0.0", kitPath: "../agentic-kit"}, hooks)
			if !errors.Is(err, failure) || !strings.Contains(err.Error(), "retained "+wantPath) {
				t.Fatalf("missing retained path %s: %v", wantPath, err)
			}
			if _, statErr := os.Lstat(wantPath); statErr != nil {
				t.Errorf("retained path missing: %v", statErr)
			}
		})
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
