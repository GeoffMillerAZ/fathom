package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func retroGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func retroBeatLine(at time.Time, channel, kind string, meta map[string]string) string {
	parts := []string{}
	for k, v := range meta {
		parts = append(parts, fmt.Sprintf("%q:%q", k, v))
	}
	return fmt.Sprintf(`{"channel":%q,"kind":%q,"at":%q,"meta":{%s}}`,
		channel, kind, at.UTC().Format(time.RFC3339Nano), strings.Join(parts, ","))
}

// The core IR-2 join: a guard fire whose file was later committed reads
// "committed"; an injection whose file never changed reads "untouched";
// a pre-IR-1 firing without stamps is counted and NAMED, never dropped.
func TestRetroJoinsFiringsToAftermath(t *testing.T) {
	root := t.TempDir()
	retroGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	retroGit(t, root, "add", "-A")
	retroGit(t, root, "commit", "-q", "-m", "seed")

	start := time.Now().Add(-2 * time.Minute)
	log := filepath.Join(t.TempDir(), "hooks.jsonl")
	lines := []string{
		retroBeatLine(start, "edit-hook", "answered", map[string]string{
			"emitted": "true", "phase": "pre", "target": "file",
			"session": "sess-1", "file": filepath.Join(root, "a.go"), "project_dir": root}),
		retroBeatLine(start.Add(10*time.Second), "edit-hook", "answered", map[string]string{
			"emitted": "true", "phase": "post",
			"session": "sess-1", "file": filepath.Join(root, "b.go"), "project_dir": root}),
		retroBeatLine(start.Add(20*time.Second), "edit-hook", "answered", map[string]string{
			"emitted": "true", "phase": "post", "project_dir": root}), // pre-IR-1: no stamps
		retroBeatLine(start.Add(30*time.Second), "edit-hook", "answered", map[string]string{
			"emitted": "false", "phase": "post",
			"session": "sess-1", "file": filepath.Join(root, "a.go"), "project_dir": root}),
	}
	if err := os.WriteFile(log, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_HOOKS_LOG", log)
	t.Setenv("HOME", t.TempDir()) // isolate the ledger + concerns reads

	// The guarded file gets committed AFTER the firing; b.go never exists.
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	retroGit(t, root, "add", "-A")
	retroGit(t, root, "commit", "-q", "-m", "post-firing edit")

	var out, errB bytes.Buffer
	if code := cmdRetro([]string{"--session", "sess-1", "--project", root}, &out, &errB); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, errB.String())
	}
	s := out.String()
	for _, want := range []string{
		"guard", "a.go", "committed",
		"obligations", "b.go", "untouched",
		"1 firing(s) unjoinable",
		"1 silent-on-clean",
		"law 12", // the no-causality line is part of the contract, not decoration
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("retro output missing %q:\n%s", want, s)
		}
	}
}

func TestRetroNamesAbsence(t *testing.T) {
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "absent.jsonl"))
	t.Setenv("HOME", t.TempDir())
	var out, errB bytes.Buffer
	if code := cmdRetro([]string{"--project", t.TempDir()}, &out, &errB); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, errB.String())
	}
	if !strings.Contains(out.String(), "no firings recorded") {
		t.Fatalf("absence must be named, got %q", out.String())
	}
}
