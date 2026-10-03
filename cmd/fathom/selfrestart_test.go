package main

// selfrestart_test.go — the daemon may adopt a new binary on its own, and
// must refuse to adopt one that cannot run.
//
// The refusal case is the load-bearing one. On 2026-08-12 an unsigned
// binary was installed over the daemon's own path; macOS refused to
// execute it (OS_REASON_CODESIGNING) and the daemon stayed down fleet-wide.
// A self-restarter that exits on "the file changed" alone would have
// turned that into an unbounded restart loop against a binary that could
// never start.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// dlogOver returns a *dlog.Logger writing to w at debug level with a fixed
// clock — the seam every selfrestart_test.go case uses in place of the old
// raw io.Writer parameter (DG-4).
func dlogOver(w *syncBuffer) *dlog.Logger {
	return dlog.New(w, dlog.LevelDebug, time.Now)
}

// syncBuffer is a concurrency-safe sink. The watcher writes from its own
// goroutine while the test polls for a phrase; a bare strings.Builder is a
// data race there, and -race catches it (the same class CLAUDE.md flags:
// reading a server's output buffer while its goroutine writes).
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// fakeBinary writes an executable script that prints what it is told.
func fakeBinary(t *testing.T, path, prints string, exitCode int) {
	t.Helper()
	body := "#!/bin/sh\n"
	if prints != "" {
		body += "echo '" + prints + "'\n"
	}
	body += "exit " + itoa(exitCode) + "\n"
	// Linux refuses to open a file for writing while an exec of it is in
	// flight (ETXTBSY), and a watcher under test re-verifies a refused
	// candidate on every poll, executing it in place (CS-8 caught one in a
	// root container). The refused open changes nothing on disk, so the
	// write is retried until the exec has let go; macOS never refuses it.
	deadline := time.Now().Add(waitSafety)
	for {
		err := os.WriteFile(path, []byte(body), 0o755)
		if err == nil {
			return
		}
		if !errors.Is(err, syscall.ETXTBSY) || time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSelfRestartAdoptsAVerifiedBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixtures are POSIX")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fathom")
	fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)

	var exited atomic.Int32
	exited.Store(-1)
	var out syncBuffer
	done := make(chan struct{})
	go func() {
		watchOwnBinary(bin, dlogOver(&out), selfRestartConfig{poll: 5 * time.Millisecond, exit: func(c int) { exited.Store(int32(c)) }})
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	fakeBinary(t, bin, "fathom v0.1.0 @ bbbbbbbbbbbb", 0)

	select {
	case <-done:
	case <-time.After(waitSafety):
		t.Fatalf("the watcher never adopted the new binary; log:\n%s", out.String())
	}
	// A schema-2 plist (KeepAlive {SuccessfulExit: false}) reads exit 0 as
	// "stay down", so adoption exits 75 — and this binary may claim
	// tenancy_contract >= 1 (which `daemon install --reconcile` gates on)
	// only because it does (design §8.2, §8.3).
	if exited.Load() != int32(adoptionExitCode()) || adoptionExitCode() != tenancy.ExitRetryLater {
		t.Errorf("exit code = %d, want %d (tenancy.ExitRetryLater) — exit 0 under a schema-2 plist leaves the daemon down", exited.Load(), tenancy.ExitRetryLater)
	}
	if tenancyContract >= 1 && exited.Load() == 0 {
		t.Errorf("tenancy_contract %d is claimed by a binary that exits 0 on adoption", tenancyContract)
	}
	if !strings.Contains(out.String(), "adopting") {
		t.Errorf("the adoption must be announced; a silent self-restart is indistinguishable from a crash:\n%s", out.String())
	}
}

func TestSelfRestartRefusesABinaryThatCannotRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixtures are POSIX")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fathom")
	fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)

	var exited atomic.Bool
	var out syncBuffer
	go watchOwnBinary(bin, dlogOver(&out), selfRestartConfig{poll: 5 * time.Millisecond, exit: func(int) { exited.Store(true) }})

	time.Sleep(20 * time.Millisecond)
	// The 2026-08-12 shape: present, changed, and unable to execute.
	if err := os.WriteFile(bin, []byte("\x00\x01 not an executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(waitSafety)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), "REFUSED") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if exited.Load() {
		t.Fatal("the daemon exited for a binary that cannot run — that is an unbounded restart loop, " +
			"and it takes every fathom tool down fleet-wide")
	}
	log := out.String()
	if !strings.Contains(log, "REFUSED") {
		t.Fatalf("the refusal must be announced with its reason:\n%s", log)
	}
	if !strings.Contains(log, "task deploy") {
		t.Errorf("the refusal must name the fix (signing), since that is the cause that actually occurred:\n%s", log)
	}
}

func TestSelfRestartWaitsForTheFileToSettle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixtures are POSIX")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fathom")
	fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)

	var exited atomic.Bool
	var out syncBuffer
	// A long poll: the file changes twice within one interval, so a
	// watcher that acted on first sight would adopt a half-written file.
	go watchOwnBinary(bin, dlogOver(&out), selfRestartConfig{poll: 150 * time.Millisecond, exit: func(int) { exited.Store(true) }})

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(bin, []byte("partial"), 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	fakeBinary(t, bin, "fathom v0.1.0 @ cccccccccccc", 0)

	// One poll is not enough to adopt: the bytes must repeat.
	time.Sleep(200 * time.Millisecond)
	if exited.Load() {
		t.Error("adopted after a single sighting — a file still being written must not be adopted")
	}
}

func TestSelfRestartDisabledByEnv(t *testing.T) {
	t.Setenv("FATHOM_SELF_RESTART", "off")
	if !selfRestartDisabled() {
		t.Error("an operator must be able to pin the binary")
	}
	t.Setenv("FATHOM_SELF_RESTART", "")
	if selfRestartDisabled() {
		t.Error("empty means on — the default is to adopt")
	}
}

// TestWatchOwnBinary_AdoptLineIncludesBuildAge pins DG-4: the existing
// "adopting" line already named the old and new hash, but never how long
// the outgoing binary had been running. selfRestartBuildStamp/selfRestartNow
// are swapped for fixed values (buildinfo.Read's own sync.Once means the
// REAL function cannot be driven to a known answer per test), so the
// elapsed figure asserted here is deterministic rather than dependent on
// whatever VCS stamp — if any — the go test binary happens to carry.
func TestWatchOwnBinary_AdoptLineIncludesBuildAge(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixtures are POSIX")
	}
	builtAt := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	origStamp, origNow := selfRestartBuildStamp, selfRestartNow
	selfRestartBuildStamp = func() buildinfo.Stamp { return buildinfo.Stamp{BuiltAt: builtAt.Format(time.RFC3339)} }
	selfRestartNow = func() time.Time { return builtAt.Add(3*time.Hour + 2*time.Minute) }
	defer func() { selfRestartBuildStamp, selfRestartNow = origStamp, origNow }()

	dir := t.TempDir()
	bin := filepath.Join(dir, "fathom")
	fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)

	var exited atomic.Int32
	exited.Store(-1)
	var out syncBuffer
	done := make(chan struct{})
	go func() {
		watchOwnBinary(bin, dlogOver(&out), selfRestartConfig{poll: 5 * time.Millisecond, exit: func(c int) { exited.Store(int32(c)) }})
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	fakeBinary(t, bin, "fathom v0.1.0 @ bbbbbbbbbbbb", 0)

	select {
	case <-done:
	case <-time.After(waitSafety):
		t.Fatalf("the watcher never adopted the new binary; log:\n%s", out.String())
	}
	log := out.String()
	if !strings.Contains(log, "adopting") {
		t.Fatalf("adoption not announced:\n%s", log)
	}
	if !strings.Contains(log, "built 3h2m0s ago") {
		t.Errorf("adoption line must name elapsed time since this binary's own build stamp:\n%s", log)
	}
}

// TestBuildAgeSuffix_OmittedWhenNoBuildStamp pins the graceful-degradation
// half: a binary with no embedded build timestamp (built without
// -buildvcs) gets no fabricated or zero-valued age — the suffix is empty.
func TestBuildAgeSuffix_OmittedWhenNoBuildStamp(t *testing.T) {
	origStamp := selfRestartBuildStamp
	selfRestartBuildStamp = func() buildinfo.Stamp { return buildinfo.Stamp{} }
	defer func() { selfRestartBuildStamp = origStamp }()

	if got := buildAgeSuffix(); got != "" {
		t.Errorf("buildAgeSuffix() = %q with no BuiltAt, want empty", got)
	}
}
