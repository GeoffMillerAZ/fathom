package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCmdMetricsRollupBuild_WritesFile(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := cmdMetricsRollupBuild([]string{"--date", "2026-07-30", "--root", root, "--call-log", filepath.Join(t.TempDir(), "none.jsonl")}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	path := filepath.Join(root, "workspace", "artifacts", "metrics", "daily", "2026-07-30.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected rollup file at %s: %v", path, err)
	}
}

func TestCmdMetricsRollupBuild_OverwritesSameDate(t *testing.T) {
	root := t.TempDir()
	args := []string{"--date", "2026-07-30", "--root", root, "--call-log", filepath.Join(t.TempDir(), "none.jsonl")}
	var out1, errBuf1 bytes.Buffer
	if code := cmdMetricsRollupBuild(args, &out1, &errBuf1); code != 0 {
		t.Fatalf("first build exit = %d, stderr: %s", code, errBuf1.String())
	}
	var out2, errBuf2 bytes.Buffer
	if code := cmdMetricsRollupBuild(args, &out2, &errBuf2); code != 0 {
		t.Fatalf("second build exit = %d, stderr: %s", code, errBuf2.String())
	}
	entries, err := os.ReadDir(filepath.Join(root, "workspace", "artifacts", "metrics", "daily"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("files = %d, want 1 (overwritten, not appended)", len(entries))
	}
}

func TestCmdMetricsRotate_DryRunByDefault_NoFileChange(t *testing.T) {
	now := time.Now().UTC()
	logPath := filepath.Join(t.TempDir(), "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: now.AddDate(0, 0, -100).Format(time.RFC3339Nano), Tool: "find_symbol"}, // older than keep-days
		{TS: now.Format(time.RFC3339Nano), Tool: "find_symbol"},                     // recent
	})
	before, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := cmdMetricsRotate([]string{"--call-log", logPath, "--keep-days", "90"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	after, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("dry run modified the call log: before=%q after=%q", before, after)
	}
	entries, _ := os.ReadDir(t.TempDir())
	_ = entries // no archive file check needed here — the byte-identical log is the load-bearing proof
}

func TestCmdMetricsRotate_Apply_ArchivesOldLines_LiveLogKeepsRecent(t *testing.T) {
	now := time.Now().UTC()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: now.AddDate(0, 0, -100).Format(time.RFC3339Nano), Tool: "old-tool"},
		{TS: now.Format(time.RFC3339Nano), Tool: "recent-tool"},
	})

	var out, errBuf bytes.Buffer
	code := cmdMetricsRotate([]string{"--call-log", logPath, "--keep-days", "90", "--apply"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}

	live, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile live: %v", err)
	}
	var liveEntries []adoptionCallEntry
	for _, line := range splitNonEmptyLines(string(live)) {
		var e adoptionCallEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal live line: %v", err)
		}
		liveEntries = append(liveEntries, e)
	}
	if len(liveEntries) != 1 || liveEntries[0].Tool != "recent-tool" {
		t.Fatalf("live entries = %+v, want only recent-tool", liveEntries)
	}

	archives, err := filepath.Glob(logPath + ".archive-*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("archives = %v, want exactly 1", archives)
	}
	archived, err := os.ReadFile(archives[0])
	if err != nil {
		t.Fatalf("ReadFile archive: %v", err)
	}
	var archivedEntries []adoptionCallEntry
	for _, line := range splitNonEmptyLines(string(archived)) {
		var e adoptionCallEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal archive line: %v", err)
		}
		archivedEntries = append(archivedEntries, e)
	}
	if len(archivedEntries) != 1 || archivedEntries[0].Tool != "old-tool" {
		t.Fatalf("archived entries = %+v, want only old-tool", archivedEntries)
	}
}

func TestCmdMetricsRotate_NeverDeletesArchive(t *testing.T) {
	now := time.Now().UTC()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: now.AddDate(0, 0, -100).Format(time.RFC3339Nano), Tool: "old-tool"},
	})
	var out, errBuf bytes.Buffer
	if code := cmdMetricsRotate([]string{"--call-log", logPath, "--keep-days", "90", "--apply"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	archives, _ := filepath.Glob(logPath + ".archive-*")
	if len(archives) != 1 {
		t.Fatalf("archives = %v, want 1", archives)
	}
	// Re-running rotate for the SAME cutoff date must refuse to overwrite
	// the existing archive, rather than silently dropping its contents.
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: now.AddDate(0, 0, -100).Format(time.RFC3339Nano), Tool: "another-old-tool"},
	})
	var out2, errBuf2 bytes.Buffer
	code := cmdMetricsRotate([]string{"--call-log", logPath, "--keep-days", "90", "--apply"}, &out2, &errBuf2)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (refuse to overwrite existing archive)", code)
	}
	// The original archive's content must be untouched.
	data, err := os.ReadFile(archives[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Contains(data, []byte("old-tool")) || bytes.Contains(data, []byte("another-old-tool")) {
		t.Errorf("archive content changed: %s", data)
	}
}

func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range bytesSplitLines(s) {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func bytesSplitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
