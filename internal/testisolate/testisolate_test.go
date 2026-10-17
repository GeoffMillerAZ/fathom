package testisolate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

type runnerFunc func() int

func (f runnerFunc) Run() int { return f() }

// isolatedVars are restored by t.Setenv after each case, since
// IsolateAndRun mutates the process environment the way TestMain does.
func saveEnv(t *testing.T) {
	t.Helper()
	for _, k := range IsolatedVars {
		t.Setenv(k, os.Getenv(k))
	}
}

// TestIsolateAndRun_ExitsWhenTempUnavailable (RT-16, §10.4): when the
// isolated root cannot be made, the suite never runs against the real
// HOME — the entry point exits 2 and names why.
func TestIsolateAndRun_ExitsWhenTempUnavailable(t *testing.T) {
	saveEnv(t)
	realHome := os.Getenv("HOME")
	var stderr bytes.Buffer
	ran := false
	code := IsolateAndRun(runnerFunc(func() int { ran = true; return 0 }), Options{
		MkdirTemp: func(string, string) (string, error) { return "", errors.New("no space left on device") },
		Stderr:    &stderr,
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if ran {
		t.Fatal("the suite ran although isolation failed")
	}
	if os.Getenv("HOME") != realHome {
		t.Fatalf("HOME changed to %q on a failed isolation", os.Getenv("HOME"))
	}
	if !strings.Contains(stderr.String(), "no space left on device") {
		t.Fatalf("stderr does not name the reason: %q", stderr.String())
	}

	// A failing per-package setup is the same refusal.
	stderr.Reset()
	ran = false
	code = IsolateAndRun(runnerFunc(func() int { ran = true; return 0 }), Options{
		MkdirTemp: func(string, string) (string, error) { return t.TempDir(), nil },
		Setup:     func(string) error { return errors.New("cache dir unwritable") },
		Stderr:    &stderr,
	})
	if code != 2 || ran {
		t.Fatalf("setup failure: code=%d ran=%v, want 2/false", code, ran)
	}
}

// TestIsolateAndRun_IsolatesEnvironment: the suite sees an isolated HOME,
// state dir, XDG state, call log, the unroutable daemon URL and
// FATHOM_TEST=1; the root is removed afterwards and the suite's code is
// returned.
func TestIsolateAndRun_IsolatesEnvironment(t *testing.T) {
	saveEnv(t)
	t.Setenv("FATHOM_HTTP", "http://localhost:7581")
	t.Setenv("FATHOM_STATE_DIR", "/real/operator/state")
	t.Setenv("FATHOM_CALL_LOG", "/real/operator/mcp-calls.jsonl")
	t.Setenv("FATHOM_HOME", "/real/operator/fathom-home")
	parent := t.TempDir()
	var root string
	seen := map[string]string{}
	var setupRoot string
	code := IsolateAndRun(runnerFunc(func() int {
		for _, k := range IsolatedVars {
			seen[k] = os.Getenv(k)
		}
		return 7
	}), Options{
		MkdirTemp: func(_, pattern string) (string, error) {
			d, err := os.MkdirTemp(parent, pattern)
			root = d
			return d, err
		},
		Setup: func(r string) error { setupRoot = r; return nil },
	})
	if code != 7 {
		t.Fatalf("exit code = %d, want the suite's 7", code)
	}
	if setupRoot != root {
		t.Fatalf("setup got %q, want root %q", setupRoot, root)
	}
	if seen["FATHOM_HTTP"] != UnroutableURL {
		t.Errorf("FATHOM_HTTP = %q, want %q (an inherited live URL must be replaced)", seen["FATHOM_HTTP"], UnroutableURL)
	}
	if seen[EnvTest] != "1" {
		t.Errorf("%s = %q, want 1", EnvTest, seen[EnvTest])
	}
	for _, k := range []string{"FATHOM_STATE_DIR", "FATHOM_CALL_LOG", "FATHOM_HOME"} {
		if seen[k] != "" {
			t.Errorf("%s = %q, want unset (it resolves under the isolated XDG_STATE_HOME)", k, seen[k])
		}
	}
	for _, k := range []string{"HOME", "XDG_STATE_HOME", "FATHOM_THREADS"} {
		if !strings.HasPrefix(seen[k], root+string(filepath.Separator)) {
			t.Errorf("%s = %q, not under the isolated root %q", k, seen[k], root)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("isolated root %s not removed after the run (err=%v)", root, err)
	}
}

// TestBackstopFromEnvInChildProcess: testing.Testing() is false in a child
// process, so the env flag alone must carry the backstop.
func TestBackstopFromEnvInChildProcess(t *testing.T) {
	if got := backstop(false, "", "http://localhost:7581"); got != "http://localhost:7581" {
		t.Fatalf("not under test: %q, want the default", got)
	}
	if got := backstop(false, "1", "http://localhost:7581"); got != UnroutableURL {
		t.Fatalf("FATHOM_TEST=1: %q, want %q", got, UnroutableURL)
	}
	if got := backstop(true, "", "http://localhost:7581"); got != UnroutableURL {
		t.Fatalf("testing.Testing(): %q, want %q", got, UnroutableURL)
	}
}

// TestIsolateAndRun_StubsModelCLIs (TQ-22): the suite's PATH resolves every
// declared model-CLI name inside the isolated root; running one exits 97,
// names the refusal and is recorded in the stub log; FATHOM_CLAUDE_BIN and
// FATHOM_CODEX_BIN are unset; git still resolves (PATH is prepended).
func TestIsolateAndRun_StubsModelCLIs(t *testing.T) {
	saveEnv(t)
	t.Setenv("FATHOM_CLAUDE_BIN", "/real/claude")
	t.Setenv("FATHOM_CODEX_BIN", "/real/codex")
	parent := t.TempDir()
	var root string
	var failures []string
	code := IsolateAndRun(runnerFunc(func() int {
		for _, k := range []string{"FATHOM_CLAUDE_BIN", "FATHOM_CODEX_BIN"} {
			if v := os.Getenv(k); v != "" {
				failures = append(failures, k+" = "+v+", want unset")
			}
		}
		if _, err := exec.LookPath("git"); err != nil {
			failures = append(failures, "git no longer resolves: "+err.Error())
		}
		logPath := os.Getenv(EnvStubLog)
		if !strings.HasPrefix(logPath, root+string(filepath.Separator)) {
			failures = append(failures, EnvStubLog+" = "+logPath+", not under the root")
		}
		for _, name := range ModelCLINames {
			p, err := exec.LookPath(name)
			if err != nil || !strings.HasPrefix(p, filepath.Join(root, "bin")+string(filepath.Separator)) {
				failures = append(failures, name+" resolves to "+p+" ("+fmt.Sprint(err)+"), want inside the root")
				continue
			}
			var stderr bytes.Buffer
			cmd := exec.Command(name, "--version", "x y")
			cmd.Stderr = &stderr
			err = cmd.Run()
			var ee *exec.ExitError
			if !errors.As(err, &ee) || ee.ExitCode() != StubExitCode {
				failures = append(failures, name+": exit err = "+fmt.Sprint(err)+", want 97")
			}
			if want := "testisolate: a test tried to run the real " + name; !strings.Contains(stderr.String(), want) {
				failures = append(failures, name+": stderr "+stderr.String()+" lacks "+want)
			}
		}
		raw, _ := os.ReadFile(logPath)
		for _, name := range ModelCLINames {
			if !strings.Contains(string(raw), name+" --version x y") {
				failures = append(failures, name+" not recorded in the stub log: "+string(raw))
			}
		}
		return 0
	}), Options{
		MkdirTemp: func(_, pattern string) (string, error) {
			d, err := os.MkdirTemp(parent, pattern)
			root = d
			return d, err
		},
	})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	for _, f := range failures {
		t.Error(f)
	}
}

// TestModelCLINamesCoverHarnesses: every harness the registry knows that is
// driven through a CLI has its binary name declared (so adding a harness
// without declaring its CLI fails here).
func TestModelCLINamesCoverHarnesses(t *testing.T) {
	declared := map[string]bool{}
	for _, n := range ModelCLINames {
		declared[n] = true
	}
	for _, d := range harness.All() {
		if d.Label == "agents" { // the generic pull tier has no binary
			continue
		}
		if !declared[d.Label] && !declared[d.Label+"-agent"] {
			t.Errorf("harness %q has no model-CLI name declared in ModelCLINames", d.Label)
		}
	}
}

// TestIsolateAndRun_UnsetsTheDecisionKillButRemembersIt (KS-1): an
// operator who exports FATHOM_DECIDE=off, the documented kill, still gets a
// deterministic suite (a test that opens a fake port is not closed by the
// shell it ran from), and a harness that must honour the kill (the
// decideexp E1 harness, EXP-3) reads the inherited value from InheritedDecide.
func TestIsolateAndRun_UnsetsTheDecisionKillButRemembersIt(t *testing.T) {
	saveEnv(t)
	t.Setenv("FATHOM_DECIDE", "off")
	t.Cleanup(func() { inheritedDecide = "" })
	var seen string
	var present bool
	code := IsolateAndRun(runnerFunc(func() int {
		seen, present = os.LookupEnv("FATHOM_DECIDE")
		return 0
	}), Options{MkdirTemp: func(_, pattern string) (string, error) { return os.MkdirTemp(t.TempDir(), pattern) }})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if present {
		t.Errorf("FATHOM_DECIDE = %q inside the suite, want unset", seen)
	}
	if got := InheritedDecide(); got != "off" {
		t.Errorf("InheritedDecide() = %q, want the operator's off", got)
	}
}
