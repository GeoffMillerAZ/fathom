package scenariobench

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallFathomShim_WritesExecutableRefusal is the fix-first review's
// own proof of the shim script itself: an executable "fathom" that runs,
// exits 127 (the shell's "command not found" convention), and refuses on
// stderr.
func TestInstallFathomShim_WritesExecutableRefusal(t *testing.T) {
	shimDir := filepath.Join(t.TempDir(), "shim")
	got, err := installFathomShim(shimDir)
	if err != nil {
		t.Fatalf("installFathomShim: %v", err)
	}
	if got != shimDir {
		t.Errorf("installFathomShim returned %q, want %q", got, shimDir)
	}

	path := filepath.Join(shimDir, "fathom")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat shim: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("expected the shim to be executable")
	}

	out, err := exec.Command(path).CombinedOutput()
	if err == nil {
		t.Fatal("expected the shim to exit non-zero")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 127 {
		t.Errorf("expected exit 127, got err=%v", err)
	}
	if !strings.Contains(string(out), fathomShimRefusal) {
		t.Errorf("expected the refusal text on stderr, got %q", out)
	}
}

// TestInstallFathomShim_Idempotent proves a re-install (a retried
// session, a re-run of the same cell) just overwrites the same file
// rather than erroring.
func TestInstallFathomShim_Idempotent(t *testing.T) {
	shimDir := filepath.Join(t.TempDir(), "shim")
	if _, err := installFathomShim(shimDir); err != nil {
		t.Fatalf("first installFathomShim: %v", err)
	}
	if _, err := installFathomShim(shimDir); err != nil {
		t.Fatalf("second installFathomShim: %v", err)
	}
}

// TestBaselinePathWithFathomShim_PrependsAndKeepsRest is the
// PATH-construction proof: shimDir goes FIRST, every other directory
// survives untouched — no stripping (fix-first review, wave-n1: the
// earlier design dropped a whole PATH directory for containing a
// "fathom" file, which could also remove an unrelated tool sharing it).
func TestBaselinePathWithFathomShim_PrependsAndKeepsRest(t *testing.T) {
	current := strings.Join([]string{"/a", "/b", "/c"}, string(os.PathListSeparator))
	got := baselinePathWithFathomShim(current, "/shim")
	want := strings.Join([]string{"/shim", "/a", "/b", "/c"}, string(os.PathListSeparator))
	if got != want {
		t.Errorf("baselinePathWithFathomShim = %q, want %q", got, want)
	}
}

// TestBaselinePathWithFathomShim_EmptyCurrentPath covers the edge case of
// a session with no inherited PATH at all.
func TestBaselinePathWithFathomShim_EmptyCurrentPath(t *testing.T) {
	if got := baselinePathWithFathomShim("", "/shim"); got != "/shim" {
		t.Errorf("got %q, want /shim", got)
	}
}

// resolveOnPath mimics shell PATH resolution: the first directory (in
// order) on pathValue containing an executable file named name, or "" if
// none does — used to assert on what a real shell would actually run,
// not merely on the PATH string's own shape.
func resolveOnPath(pathValue, name string) string {
	for _, dir := range filepath.SplitList(pathValue) {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// TestBaselineIsolationEnv_SharedDirKeepsClaudeShimsFathom is the
// fix-first review's own required proof: a directory holding BOTH a real
// `fathom` and a real `claude` executable must not lose `claude` — only
// `fathom` resolves to something else (the shim, ahead of it on PATH),
// because the earlier design dropped the WHOLE directory for containing
// a "fathom" file.
func TestBaselineIsolationEnv_SharedDirKeepsClaudeShimsFathom(t *testing.T) {
	shared := t.TempDir()
	for _, bin := range []string{"fathom", "claude"} {
		if err := os.WriteFile(filepath.Join(shared, bin), []byte("#!/bin/sh\necho real\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldPath, hadPath := os.LookupEnv("PATH")
	defer func() {
		if hadPath {
			os.Setenv("PATH", oldPath)
		} else {
			os.Unsetenv("PATH")
		}
	}()
	os.Setenv("PATH", shared)

	cellDir := t.TempDir()
	env, err := baselineIsolationEnv(Arm{Wired: false}, cellDir)
	if err != nil {
		t.Fatalf("baselineIsolationEnv: %v", err)
	}

	wantClaude := filepath.Join(shared, "claude")
	if got := resolveOnPath(env["PATH"], "claude"); got != wantClaude {
		t.Errorf("claude resolved to %q, want the real one at %q", got, wantClaude)
	}
	wantShim := filepath.Join(cellDir, "baseline-fathom-shim", "fathom")
	if got := resolveOnPath(env["PATH"], "fathom"); got != wantShim {
		t.Errorf("fathom resolved to %q, want the shim at %q (never the real one at %s)", got, wantShim, filepath.Join(shared, "fathom"))
	}
}

// TestBaselineIsolationEnv_WiredArmGetsNoOverrides proves the shim (and
// the unroutable FATHOM_HTTP) are scoped to non-wired arms only.
func TestBaselineIsolationEnv_WiredArmGetsNoOverrides(t *testing.T) {
	env, err := baselineIsolationEnv(Arm{Wired: true}, t.TempDir())
	if err != nil {
		t.Fatalf("baselineIsolationEnv: %v", err)
	}
	if env != nil {
		t.Errorf("expected nil overrides for a wired arm, got %v", env)
	}
}

// TestBaselineIsolationEnv_EmptyCellDirSkipsShim covers the documented
// edge case: no CellDir (a hand-built test-fixture Worktree) skips the
// shim rather than erroring, but still pins FATHOM_HTTP.
func TestBaselineIsolationEnv_EmptyCellDirSkipsShim(t *testing.T) {
	env, err := baselineIsolationEnv(Arm{Wired: false}, "")
	if err != nil {
		t.Fatalf("baselineIsolationEnv: %v", err)
	}
	if _, ok := env["PATH"]; ok {
		t.Errorf("expected no PATH override with no CellDir, got %q", env["PATH"])
	}
	if env["FATHOM_HTTP"] != unroutableFathomURL {
		t.Errorf("FATHOM_HTTP = %q, want %q", env["FATHOM_HTTP"], unroutableFathomURL)
	}
}

// TestFathomCommandRE covers the invocation matcher directly: bare, via a
// relative or absolute path, chained after shell separators, and in a
// pipeline or subshell all match; a mention of "fathom" as a path
// COMPONENT of something else, or as a fragment of a longer identifier,
// must not.
func TestFathomCommandRE(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"fathom verify --service fathom", true},
		{"fathom verify", true},
		{"./fathom verify", true},
		{"bin/fathom --version", true},
		{"/usr/local/bin/fathom doctor", true},
		{"echo hi && fathom verify", true},
		{"echo hi; fathom verify", true},
		{"echo hi | fathom verify", true},
		{"result=$(fathom verify)", true},
		{"echo `fathom --version`", true},
		{"go build ./cmd/fathom", false},
		{"go vet ./cmd/fathom ./internal/...", false},
		{"go doc github.com/geoffmilleraz/fathom/internal/scenariobench", false},
		{"cat cmd/fathom/main.go", false},
		{"fathomless --check", false},
		{"echo unrelated-fathomthing", false},
		{"grep -r fathom internal/", false},
	}
	for _, c := range cases {
		got := fathomCommandRE.MatchString(c.cmd)
		if got != c.want {
			t.Errorf("fathomCommandRE.MatchString(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

// writeBashToolUseTranscript writes one assistant Bash tool_use event (no
// paired tool_result — BaselineRanFathomScan only reads tool_use inputs)
// as a single stream-json line.
func writeBashToolUseTranscript(t *testing.T, dir, name, command string) string {
	t.Helper()
	input, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		t.Fatal(err)
	}
	ev := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []map[string]any{
				{"type": "tool_use", "id": "tu1", "name": "Bash", "input": json.RawMessage(input)},
			},
		},
	}
	line, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestBaselineRanFathomScan_DetectsBashInvocation is O53 defect 4's
// post-session-scan proof: a Bash tool_use invoking fathom must be
// caught, even if PATH exclusion should have kept it from ever resolving.
func TestBaselineRanFathomScan_DetectsBashInvocation(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := writeBashToolUseTranscript(t, dir, "bash-fathom.jsonl", "fathom verify --service fathom")
	session := SessionResult{RawPath: transcriptPath}
	if !BaselineRanFathomScan(session) {
		t.Error("expected a fathom invocation to be detected")
	}
}

// TestBaselineRanFathomScan_NoInvocationReturnsFalse covers the converse.
func TestBaselineRanFathomScan_NoInvocationReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := writeBashToolUseTranscript(t, dir, "bash-clean.jsonl", "go build ./cmd/fathom")
	session := SessionResult{RawPath: transcriptPath}
	if BaselineRanFathomScan(session) {
		t.Error("expected no leak from an unrelated Bash command")
	}
}

// TestBaselineRanFathomScan_EmptyRawPathIsFalse: no transcript to scan is
// "not measurable", never a false positive.
func TestBaselineRanFathomScan_EmptyRawPathIsFalse(t *testing.T) {
	if BaselineRanFathomScan(SessionResult{RawPath: ""}) {
		t.Error("expected false with no RawPath")
	}
}

// TestRunSessions_BaselineRanFathomLosesTheCell is the end-to-end proof: a
// non-wired session whose transcript invokes fathom gets
// BaselineRanFathom set, and CellLost reports the cell lost because of
// it.
func TestRunSessions_BaselineRanFathomLosesTheCell(t *testing.T) {
	streamsTmp := t.TempDir()
	writeBashToolUseTranscript(t, streamsTmp, "ran-fathom.jsonl", "fathom verify --service fathom")

	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsTmp,
		TaskID:     "T1",
		ArmName:    "grep",
		Streams:    map[string]string{"T1/grep": "ran-fathom.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "grep", Arm{Wired: false}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results))
	}
	if !results[0].BaselineRanFathom {
		t.Error("expected BaselineRanFathom = true")
	}
	reason := CellLost(results)
	if reason == "" {
		t.Fatal("expected CellLost to report this cell as lost")
	}
	if got := reason; got != "baseline_ran_fathom: a Bash call invoked the fathom binary despite this arm's PATH exclusion" {
		t.Errorf("CellLost reason = %q", got)
	}
}

// TestRunSessions_WiredArmFathomInvocationNotFlagged proves the scan is
// scoped to non-wired arms only: a wired arm's own transcript naming
// fathom (its stanza tells it to run `fathom verify`) is expected
// behavior, not a defect.
func TestRunSessions_WiredArmFathomInvocationNotFlagged(t *testing.T) {
	streamsTmp := t.TempDir()
	writeBashToolUseTranscript(t, streamsTmp, "wired-fathom.jsonl", "fathom verify --service fathom")

	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsTmp,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "wired-fathom.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{Wired: true}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if results[0].BaselineRanFathom {
		t.Error("expected a wired arm's own fathom invocation not to be flagged")
	}
}
