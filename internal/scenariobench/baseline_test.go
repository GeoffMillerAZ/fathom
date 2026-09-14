package scenariobench

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBaselinePathExcludingFathom_DropsDirsContainingFathom is O53 defect
// 4's PATH-construction proof: any directory on PATH that contains a file
// literally named "fathom" must be dropped, whatever its position, while
// unrelated directories survive in their original order.
func TestBaselinePathExcludingFathom_DropsDirsContainingFathom(t *testing.T) {
	root := t.TempDir()
	hasFathom := filepath.Join(root, "has-fathom")
	clean1 := filepath.Join(root, "clean1")
	clean2 := filepath.Join(root, "clean2")
	for _, d := range []string{hasFathom, clean1, clean2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(hasFathom, "fathom"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	currentPath := strings.Join([]string{clean1, hasFathom, clean2}, string(os.PathListSeparator))

	got := baselinePathExcludingFathom(currentPath, "")

	gotDirs := strings.Split(got, string(os.PathListSeparator))
	if len(gotDirs) != 2 || gotDirs[0] != clean1 || gotDirs[1] != clean2 {
		t.Errorf("baselinePathExcludingFathom = %q, want %s%c%s (order preserved, hasFathom dropped)", got, clean1, os.PathListSeparator, clean2)
	}
}

// TestBaselinePathExcludingFathom_DropsCellBinDir covers the second
// exclusion: the cell's own bin dir, even when it carries no "fathom"
// file (a non-wired arm never builds one there) — defensive, per the
// defect's own wording.
func TestBaselinePathExcludingFathom_DropsCellBinDir(t *testing.T) {
	root := t.TempDir()
	cellBin := filepath.Join(root, "cell-bin")
	other := filepath.Join(root, "other")
	for _, d := range []string{cellBin, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	currentPath := strings.Join([]string{cellBin, other}, string(os.PathListSeparator))

	got := baselinePathExcludingFathom(currentPath, cellBin)

	if got != other {
		t.Errorf("baselinePathExcludingFathom = %q, want %q", got, other)
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
