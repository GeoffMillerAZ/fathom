// Package testisolate is the one test entry point for every package that
// reaches a daemon URL, state-directory or call-log default (ADR-047,
// docs/design/responsible-tenant.md §10.4, invariant I52): a test binary
// runs with an isolated HOME, state dir, XDG state home, call log and
// thread log, FATHOM_HTTP pointed at an unroutable address and
// FATHOM_TEST=1 — or it does not run at all (exit 2). Before this, each
// package's TestMain made its own temp HOME and silently kept the real one
// when MkdirTemp failed, and cmd/fathom's tests appended rows to the
// operator's call log through the live daemon (INC-23).
//
// A package opts in with a testmain_test.go:
//
//	func TestMain(m *testing.M) { testisolate.Main(m) }
//
// TestEveryPackageWithDaemonDefaultsIsIsolated keeps the list honest.
package testisolate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ModelCLINames are the harness/model CLI executables no test may run for
// real (TQ-22): IsolateAndRun puts a refusing stub for each first on PATH.
// TestModelCLINamesCoverHarnesses pins the list against internal/harness.
var ModelCLINames = []string{"agent", "claude", "codex", "copilot", "cursor-agent", "gemini"}

// EnvStubLog names the file the stubs append "<name> <args>" lines to.
const EnvStubLog = "FATHOM_TEST_STUB_LOG"

// StubExitCode is what a stub exits with: distinct from a model CLI's own
// codes so a test that reached one is recognisable.
const StubExitCode = 97

// UnroutableURL is the daemon URL every test sees unless it names its own:
// port 1 on loopback refuses at once, so a stray RPC fails fast and never
// reaches the operator's daemon.
const UnroutableURL = "http://127.0.0.1:1"

// EnvTest marks a process (and every child it execs with os.Environ()) as a
// test: the URL backstops and the X-Fathom-Caller test flag read it, since
// testing.Testing() is false in a child process.
const EnvTest = "FATHOM_TEST"

// IsolatedVars are the variables IsolateAndRun sets, in the order it sets
// them. FATHOM_STATE_DIR and FATHOM_CALL_LOG are UNSET rather than pinned:
// an inherited value would point at the operator's state or log, and one
// pinned path would fold every repository's state (and every service's
// call log) into one — the code under test keys both per repository under
// XDG_STATE_HOME, which is isolated instead.
//
// FATHOM_HOME (PE-26) is UNSET too: it would otherwise win over the isolated
// HOME and send every per-user path (ledgers, stamps, daemon records) to
// wherever the surrounding gate or scratch run pointed it. A test that
// exercises the override sets it itself with t.Setenv.
var IsolatedVars = []string{
	"HOME", "FATHOM_STATE_DIR", "XDG_STATE_HOME", "FATHOM_CALL_LOG",
	"FATHOM_THREADS", "FATHOM_HTTP", "FATHOM_HOME", EnvTest,
	// TQ-22: PATH gets the stub dir PREPENDED (git and go keep working);
	// the model-CLI overrides are unset so they cannot point past the stubs.
	"PATH", EnvStubLog, "FATHOM_CLAUDE_BIN", "FATHOM_CODEX_BIN",
}

// UnderTest reports whether this process is a test binary or was started
// by one (FATHOM_TEST=1).
func UnderTest() bool {
	return testing.Testing() || os.Getenv(EnvTest) == "1"
}

// DaemonURLBackstop is the default daemon URL for a resolver whose
// FATHOM_HTTP is unset: the unroutable URL under test, else def.
func DaemonURLBackstop(def string) string {
	return backstop(testing.Testing(), os.Getenv(EnvTest), def)
}

func backstop(inTestBinary bool, envTest, def string) string {
	if inTestBinary || envTest == "1" {
		return UnroutableURL
	}
	return def
}

// Runner is what a TestMain hands over; *testing.M satisfies it.
type Runner interface{ Run() int }

// Options injects the seams IsolateAndRun's own tests need; the zero value
// is production.
type Options struct {
	// MkdirTemp makes the isolated root (default os.MkdirTemp).
	MkdirTemp func(dir, pattern string) (string, error)
	// Setup redirects a package's own caches (package-level vars) under
	// root; an error refuses the run like any other isolation failure.
	Setup func(root string) error
	// Stderr receives the refusal (default os.Stderr).
	Stderr io.Writer
}

// Main isolates, runs the suite and exits with its code — or 2 when it
// cannot isolate. setup, when given, redirects the package's own caches.
func Main(m Runner, setup ...func(root string) error) {
	o := Options{}
	if len(setup) > 0 {
		o.Setup = func(root string) error {
			for _, s := range setup {
				if err := s(root); err != nil {
					return err
				}
			}
			return nil
		}
	}
	os.Exit(IsolateAndRun(m, o))
}

// IsolateAndRun builds the isolated environment, runs m, removes the root
// and returns m's code; it returns 2 without running m when any step of
// the isolation fails, so a suite never falls back to the real HOME.
func IsolateAndRun(m Runner, o Options) int {
	if o.MkdirTemp == nil {
		o.MkdirTemp = os.MkdirTemp
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	refuse := func(err error) int {
		fmt.Fprintf(o.Stderr, "testisolate: refusing to run tests without isolation (ADR-047 §10.4): %v\n", err)
		return 2
	}
	root, err := o.MkdirTemp("", "fathom-test-")
	if err != nil {
		return refuse(fmt.Errorf("making the isolated root: %w", err))
	}
	if root == "" {
		return refuse(fmt.Errorf("making the isolated root: empty path"))
	}
	defer os.RemoveAll(root)

	dirs := map[string]string{
		"HOME":           filepath.Join(root, "home"),
		"XDG_STATE_HOME": filepath.Join(root, "xdg-state"),
	}
	for _, k := range []string{"HOME", "XDG_STATE_HOME"} {
		if err := os.MkdirAll(dirs[k], 0o755); err != nil {
			return refuse(fmt.Errorf("making %s: %w", k, err))
		}
	}
	values := map[string]string{
		"HOME":              dirs["HOME"],
		"FATHOM_STATE_DIR":  "",
		"XDG_STATE_HOME":    dirs["XDG_STATE_HOME"],
		"FATHOM_CALL_LOG":   "",
		"FATHOM_THREADS":    filepath.Join(root, "home", ".fathom", "threads.jsonl"),
		"FATHOM_HTTP":       UnroutableURL,
		"FATHOM_HOME":       "",
		EnvTest:             "1",
		EnvStubLog:          filepath.Join(root, "stub-calls.log"),
		"FATHOM_CLAUDE_BIN": "",
		"FATHOM_CODEX_BIN":  "",
	}
	binDir, err := writeModelCLIStubs(root)
	if err != nil {
		return refuse(err)
	}
	values["PATH"] = binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	for _, k := range IsolatedVars {
		if values[k] == "" {
			if err := os.Unsetenv(k); err != nil {
				return refuse(fmt.Errorf("unsetting %s: %w", k, err))
			}
			continue
		}
		if err := os.Setenv(k, values[k]); err != nil {
			return refuse(fmt.Errorf("setting %s: %w", k, err))
		}
	}
	if o.Setup != nil {
		if err := o.Setup(root); err != nil {
			return refuse(fmt.Errorf("package setup: %w", err))
		}
	}
	return m.Run()
}

// writeModelCLIStubs makes <root>/bin holding one refusing stub per
// ModelCLINames entry and returns the directory.
func writeModelCLIStubs(root string) (string, error) {
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", fmt.Errorf("making the stub dir: %w", err)
	}
	for _, name := range ModelCLINames {
		body := "#!/bin/sh\n" +
			"echo \"" + name + " $*\" >> \"$" + EnvStubLog + "\"\n" +
			"echo \"testisolate: a test tried to run the real " + name + "\" >&2\n" +
			"exit " + fmt.Sprint(StubExitCode) + "\n"
		if strings.ContainsAny(name, " \"'$`\\") {
			return "", fmt.Errorf("model CLI name %q is not a plain word", name)
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			return "", fmt.Errorf("writing the %s stub: %w", name, err)
		}
	}
	return bin, nil
}
