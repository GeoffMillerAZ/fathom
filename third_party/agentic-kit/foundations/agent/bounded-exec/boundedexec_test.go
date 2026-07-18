// Tests for boundedexec deliberately deviate from the repo's
// injected-clock / no-sleep rule (see the package doc): a subprocess
// runner can only be exercised honestly with real subprocesses and real
// short timeouts. Helpers use /bin/sh -c and /usr/bin/env; everything
// skips on windows or when those binaries are absent.
package boundedexec

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	shPath  = "/bin/sh"
	envPath = "/usr/bin/env"
)

// requireTool skips the test on windows or when the helper binary used by
// the test is not present on this host.
func requireTool(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("posix shell helpers; skipping on windows")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s not available: %v", path, err)
	}
}

func mustRunner(t *testing.T, cfg Config) *Runner {
	t.Helper()
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New(%+v): %v", cfg, err)
	}
	return r
}

// generousCfg bounds tests that should finish on their own well before the
// deadline.
func generousCfg() Config {
	return Config{
		Timeout:        10 * time.Second,
		MaxStdoutBytes: 1 << 16,
		MaxStderrBytes: 1 << 16,
	}
}

func TestNewValidation(t *testing.T) {
	t.Parallel()
	valid := generousCfg()
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid", func(*Config) {}, false},
		{"valid with nil env and empty dir", func(c *Config) { c.Env = nil; c.Dir = "" }, false},
		{"zero timeout", func(c *Config) { c.Timeout = 0 }, true},
		{"negative timeout", func(c *Config) { c.Timeout = -time.Second }, true},
		{"zero stdout cap", func(c *Config) { c.MaxStdoutBytes = 0 }, true},
		{"negative stdout cap", func(c *Config) { c.MaxStdoutBytes = -1 }, true},
		{"zero stderr cap", func(c *Config) { c.MaxStderrBytes = 0 }, true},
		{"negative stderr cap", func(c *Config) { c.MaxStderrBytes = -5 }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := valid
			tt.mutate(&cfg)
			r, err := New(cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("New(%+v) = %v, want error", cfg, r)
				}
				return
			}
			if err != nil {
				t.Fatalf("New(%+v): unexpected error %v", cfg, err)
			}
		})
	}
}

func TestEmptyPath(t *testing.T) {
	t.Parallel()
	r := mustRunner(t, generousCfg())
	if _, err := r.Run(context.Background(), Spec{}); err == nil {
		t.Fatal("Run with empty Path: want error, got nil")
	}
}

func TestStartErrorNonexistentBinary(t *testing.T) {
	t.Parallel()
	r := mustRunner(t, generousCfg())
	_, err := r.Run(context.Background(), Spec{Path: "/nonexistent/boundedexec-no-such-binary"})
	if err == nil {
		t.Fatal("Run with nonexistent binary: want error, got nil")
	}
}

func TestEchoRoundTrip(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, generousCfg())

	res, err := r.Run(context.Background(), Spec{
		Path: shPath,
		Args: []string{"-c", "printf hello; printf world >&2"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := string(res.Stdout); got != "hello" {
		t.Errorf("Stdout = %q, want %q", got, "hello")
	}
	if got := string(res.Stderr); got != "world" {
		t.Errorf("Stderr = %q, want %q", got, "world")
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if res.StdoutTruncated || res.StderrTruncated {
		t.Errorf("truncation flags = %v/%v, want false/false", res.StdoutTruncated, res.StderrTruncated)
	}
	if res.TimedOut {
		t.Error("TimedOut = true, want false")
	}
	if res.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", res.Duration)
	}
}

func TestExitCodes(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, generousCfg())

	tests := []struct {
		script string
		want   int
	}{
		{"exit 0", 0},
		{"exit 1", 1},
		{"exit 3", 3},
		{"exit 42", 42},
	}
	for _, tt := range tests {
		t.Run(tt.script, func(t *testing.T) {
			t.Parallel()
			res, err := r.Run(context.Background(), Spec{Path: shPath, Args: []string{"-c", tt.script}})
			if err != nil {
				t.Fatalf("Run: %v (non-zero exits are data, not errors)", err)
			}
			if res.ExitCode != tt.want {
				t.Errorf("ExitCode = %d, want %d", res.ExitCode, tt.want)
			}
			if res.TimedOut {
				t.Error("TimedOut = true, want false")
			}
		})
	}
}

func TestStdin(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, generousCfg())

	const payload = "ping over stdin\nsecond line\n"
	res, err := r.Run(context.Background(), Spec{
		Path:  shPath,
		Args:  []string{"-c", "cat"},
		Stdin: []byte(payload),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := string(res.Stdout); got != payload {
		t.Errorf("Stdout = %q, want %q", got, payload)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestOutputCapsAndChildNotBlocked(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)

	const (
		stdoutCap = 1000
		stderrCap = 500
		// outLine/errLine are 33 bytes each with the newline; 5000
		// iterations writes ~165 KB per stream — far beyond both the caps
		// and the kernel pipe buffer, so completion proves the child was
		// drained, not blocked.
		outLine = "abcdefghijklmnopqrstuvwxyz012345\n"
		errLine = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345\n"
		iters   = "5000"
	)
	script := "n=0; while [ $n -lt " + iters + " ]; do printf '" +
		strings.TrimSuffix(outLine, "\n") + "\\n'; printf '" +
		strings.TrimSuffix(errLine, "\n") + "\\n' >&2; n=$((n+1)); done"

	r := mustRunner(t, Config{
		Timeout:        10 * time.Second,
		MaxStdoutBytes: stdoutCap,
		MaxStderrBytes: stderrCap,
	})
	res, err := r.Run(context.Background(), Spec{Path: shPath, Args: []string{"-c", script}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 || res.TimedOut {
		t.Fatalf("child did not complete cleanly: ExitCode=%d TimedOut=%v (blocked on a full pipe?)",
			res.ExitCode, res.TimedOut)
	}
	if len(res.Stdout) != stdoutCap {
		t.Errorf("len(Stdout) = %d, want exactly %d", len(res.Stdout), stdoutCap)
	}
	if len(res.Stderr) != stderrCap {
		t.Errorf("len(Stderr) = %d, want exactly %d", len(res.Stderr), stderrCap)
	}
	if !res.StdoutTruncated {
		t.Error("StdoutTruncated = false, want true")
	}
	if !res.StderrTruncated {
		t.Error("StderrTruncated = false, want true")
	}
	// First-N semantics: the kept bytes must be the prefix of the stream.
	wantOut := strings.Repeat(outLine, stdoutCap/len(outLine)+1)[:stdoutCap]
	if got := string(res.Stdout); got != wantOut {
		t.Errorf("Stdout kept wrong bytes:\n got %q\nwant %q", got, wantOut)
	}
	wantErr := strings.Repeat(errLine, stderrCap/len(errLine)+1)[:stderrCap]
	if got := string(res.Stderr); got != wantErr {
		t.Errorf("Stderr kept wrong bytes:\n got %q\nwant %q", got, wantErr)
	}
}

func TestExactCapNotTruncated(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, Config{Timeout: 10 * time.Second, MaxStdoutBytes: 5, MaxStderrBytes: 5})
	res, err := r.Run(context.Background(), Spec{Path: shPath, Args: []string{"-c", "printf abcde"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := string(res.Stdout); got != "abcde" {
		t.Errorf("Stdout = %q, want %q", got, "abcde")
	}
	if res.StdoutTruncated {
		t.Error("StdoutTruncated = true for exactly-cap-sized output, want false")
	}
}

func TestTimeoutKillsSleepingChild(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, Config{
		Timeout:        100 * time.Millisecond,
		MaxStdoutBytes: 1 << 10,
		MaxStderrBytes: 1 << 10,
	})

	res, err := r.Run(context.Background(), Spec{Path: shPath, Args: []string{"-c", "sleep 10"}})
	if err != nil {
		t.Fatalf("Run: %v (timeout is data, not an error)", err)
	}
	if !res.TimedOut {
		t.Error("TimedOut = false, want true")
	}
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 (killed by signal)", res.ExitCode)
	}
	if res.Duration >= time.Second {
		t.Errorf("Duration = %v, want well under 1s for a 100ms timeout", res.Duration)
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, generousCfg())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(100*time.Millisecond, cancel)
	defer timer.Stop()

	start := time.Now()
	res, err := r.Run(ctx, Spec{Path: shPath, Args: []string{"-c", "sleep 10"}})
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want errors.Is(err, context.Canceled)", err)
	}
	if res.TimedOut {
		t.Error("TimedOut = true for cancellation, want false (classified distinctly)")
	}
	if elapsed >= 2*time.Second {
		t.Errorf("Run took %v after cancel, want well under the child's 10s sleep", elapsed)
	}
}

func TestContextDeadlineCountsAsTimeout(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	// cfg.Timeout is generous; the ctx deadline is the binding bound —
	// the hard deadline is min(ctx deadline, cfg.Timeout).
	r := mustRunner(t, generousCfg())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	res, err := r.Run(ctx, Spec{Path: shPath, Args: []string{"-c", "sleep 10"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.TimedOut {
		t.Error("TimedOut = false, want true when the ctx deadline is the binding bound")
	}
	if res.Duration >= time.Second {
		t.Errorf("Duration = %v, want well under 1s", res.Duration)
	}
}

func TestKilledBySignalClassifiedDistinctly(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, generousCfg())

	// The child kills itself: an ExitError with a signal, but NOT a
	// timeout — TimedOut must stay false and err nil.
	res, err := r.Run(context.Background(), Spec{Path: shPath, Args: []string{"-c", "kill -KILL $$"}})
	if err != nil {
		t.Fatalf("Run: %v (signal death is data, not an error)", err)
	}
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for killed-by-signal", res.ExitCode)
	}
	if res.TimedOut {
		t.Error("TimedOut = true, want false: signal death is distinct from timeout")
	}
}

func TestEnvIsExactlyTheAllowlist(t *testing.T) {
	requireTool(t, envPath)
	// A canary in the parent environment must NOT leak to the child.
	t.Setenv("BOUNDEDEXEC_SECRET_CANARY", "leaked-api-key")

	tests := []struct {
		name string
		env  []string
		want []string
	}{
		{"allowlist", []string{"AK_ALLOWED=yes", "AK_OTHER=1"}, []string{"AK_ALLOWED=yes", "AK_OTHER=1"}},
		{"empty slice means empty env", []string{}, nil},
		{"nil means empty env, not inheritance", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := generousCfg()
			cfg.Env = tt.env
			r := mustRunner(t, cfg)
			res, err := r.Run(context.Background(), Spec{Path: envPath})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.ExitCode != 0 {
				t.Fatalf("ExitCode = %d, want 0 (stderr: %q)", res.ExitCode, res.Stderr)
			}
			var got []string
			for _, line := range strings.Split(string(res.Stdout), "\n") {
				if line != "" {
					got = append(got, line)
				}
			}
			want := append([]string(nil), tt.want...)
			sort.Strings(got)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("child env = %q, want exactly %q", got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("child env = %q, want exactly %q", got, want)
				}
			}
			if strings.Contains(string(res.Stdout), "BOUNDEDEXEC_SECRET_CANARY") {
				t.Error("parent canary variable leaked into the child environment")
			}
		})
	}
}

func TestDirSetsWorkingDirectory(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	dir := t.TempDir()
	cfg := generousCfg()
	cfg.Dir = dir
	r := mustRunner(t, cfg)

	res, err := r.Run(context.Background(), Spec{Path: shPath, Args: []string{"-c", "pwd"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.TrimSpace(string(res.Stdout))
	// Compare via os.Stat to be robust to symlinked temp dirs (macOS).
	wantInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatalf("child pwd %q: %v", got, err)
	}
	if !os.SameFile(wantInfo, gotInfo) {
		t.Errorf("child pwd = %q, want %q", got, dir)
	}
}

func TestContextDoneBeforeStart(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, generousCfg())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Run(ctx, Spec{Path: shPath, Args: []string{"-c", "true"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want errors.Is(err, context.Canceled)", err)
	}
}

func TestConcurrentRuns(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)
	r := mustRunner(t, generousCfg())

	const n = 8
	outs := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := r.Run(context.Background(), Spec{
				Path: shPath,
				Args: []string{"-c", "printf run-" + string(rune('a'+i))},
			})
			errs[i] = err
			outs[i] = string(res.Stdout)
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("run %d: %v", i, errs[i])
			continue
		}
		if want := "run-" + string(rune('a'+i)); outs[i] != want {
			t.Errorf("run %d: Stdout = %q, want %q", i, outs[i], want)
		}
	}
}
