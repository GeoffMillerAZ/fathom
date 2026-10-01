package scenariobenchwired_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	. "github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// TestRenderArm_WiredKeepsRealFathomInitHooks is the CRITICAL defect's own
// end-to-end regression proof: build the REAL fathom binary, run the REAL
// `fathom init` against a temp worktree exactly as PrepareSubject's
// wireScratchDaemon does, then call RenderArm on the same worktree and
// assert the real hook stack — including edit-hook and session-hook —
// survives in ArmFiles.SettingsPath, never replaced by a stand-in.
//
// It lives here, not in internal/scenariobench, so `go test` runs it in its
// own test binary beside the package's other tests (see doc.go).
func TestRenderArm_WiredKeepsRealFathomInitHooks(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the real fathom binary; skipped in -short")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")

	binDir := t.TempDir()
	bin := filepath.Join(binDir, "fathom")
	// Built the way a cell builds it: from a clean export of HEAD, cached
	// per commit — the working tree may carry another editor's half-edit.
	if out, err := BuildFathomBinary(context.Background(), repoRoot, bin); err != nil {
		t.Fatalf("building fathom: %v\n%s", err, out)
	}

	// A wired arm's real settings.json is written by `fathom init`, into
	// the CELL's worktree, naming the CELL's own binary — never the
	// operator's real ~/.fathom or ~/.claude state.
	t.Setenv("HOME", t.TempDir())
	worktree := t.TempDir()
	initCmd := exec.Command(bin, "init", "--path", worktree, "--service", "renderarmtest", "--url", "http://127.0.0.1:1", "--bin", bin, "--harness", "claude-code")
	out, _ := initCmd.CombinedOutput() // exit 2 (daemon unreachable) is fine; wiring still completes
	t.Logf("fathom init output:\n%s", out)

	settingsPath := filepath.Join(worktree, ".claude", "settings.json")
	before, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("fathom init did not write %s: %v (init output: %s)", settingsPath, err, out)
	}
	for _, want := range []string{"edit-hook", "session-hook"} {
		if !strings.Contains(string(before), want) {
			t.Fatalf("fathom init's own settings.json is missing %q — test precondition failed: %s", want, before)
		}
	}

	arm := Arm{Wired: true, Instructions: "stanza"}
	files, err := RenderArm(arm, worktree)
	if err != nil {
		t.Fatalf("RenderArm: %v", err)
	}
	after, err := os.ReadFile(files.SettingsPath)
	if err != nil {
		t.Fatalf("reading %s: %v", files.SettingsPath, err)
	}
	for _, want := range []string{"edit-hook", "session-hook"} {
		if !strings.Contains(string(after), want) {
			t.Errorf("RenderArm replaced fathom init's real hooks — %q did not survive: %s", want, after)
		}
	}
}
