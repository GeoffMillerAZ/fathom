package main

// edithook_pinstamp_test.go — PR-13 (docs/plans/served-not-kept.md Wave 3):
// PreToolUse(Bash) stamps HEAD before a git commit statement, so the
// PostToolUse(Bash) pin check finds the new commit by comparing HEAD to the
// stamp when git printed no summary line (744 of 745 checks were skipped as
// no_summary to 2026-10-10). A missing or stale stamp falls back to the
// summary line, as before.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// pinPrePayload is a PreToolUse(Bash) envelope for the session the post
// envelopes (testRunPostPayload) carry.
func pinPrePayload(cwd, command string) string {
	return fmt.Sprintf(`{"session_id":"tr-session","cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":%q},"tool_use_id":"t1"}`, cwd, command)
}

// runPinPreHook runs `fathom edit-hook --pre` in process on a
// PreToolUse(Bash) envelope.
func runPinPreHook(t *testing.T, cwd, command string) {
	t.Helper()
	srv, _ := countingDaemon(t)
	restore := pipeStdin(t, pinPrePayload(cwd, command))
	defer restore()
	if rc := cmdEditHook([]string{"--pre", "--url", srv.URL}, io.Discard, io.Discard); rc != 0 {
		t.Fatalf("edit-hook --pre exited %d", rc)
	}
}

// runPinPostHookAt is runPinEditHook from cwd.
func runPinPostHookAt(t *testing.T, cwd, command, output string) string {
	t.Helper()
	srv, _ := countingDaemon(t)
	restore := pipeStdin(t, testRunPostPayload(cwd, command, output))
	defer restore()
	var out bytes.Buffer
	if rc := cmdEditHook([]string{"--url", srv.URL}, &out, io.Discard); rc != 0 {
		t.Fatalf("edit-hook exited %d", rc)
	}
	return out.String()
}

func pinHead(t *testing.T, repo string) string {
	t.Helper()
	return strings.TrimSpace(pinGit(t, repo, "rev-parse", "HEAD"))
}

func stampFiles(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir(guardStampDir)
	if err != nil {
		return 0
	}
	return len(entries)
}

func TestPinCheckFindsCommitWithoutSummaryLine(t *testing.T) {
	repo := pinRepo(t, config.PinCheckFeedback)
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	last := func(t *testing.T) map[string]string {
		t.Helper()
		beats := pinBeats(t)
		if len(beats) == 0 {
			t.Fatal("no pin beat")
		}
		return beats[len(beats)-1]
	}
	// step runs the pre hook, then the command's effect (git, quietly), then
	// the post hook with the output the command printed.
	step := func(t *testing.T, cwd, command string, effect func() string) string {
		t.Helper()
		runPinPreHook(t, cwd, command)
		return runPinPostHookAt(t, cwd, command, effect())
	}

	// A non-git command stamps nothing (and forks no git on the pre path).
	runPinPreHook(t, repo, "ls -la && echo committed")
	if n := stampFiles(t); n != 0 {
		t.Fatalf("a Bash call naming no git commit wrote %d stamp(s)", n)
	}

	// `git commit -q`: no summary line; the stamp finds the commit.
	writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+strings.Replace(pinHalfUpTest, "TestRoundHalfUp(", "TestRoundHalfUpRenamed(", 1)+"\n"+pinDownTest)
	out := step(t, repo, "git commit -qam rename", func() string { return pinCommitAll(t, repo, "rename", "-q") })
	if !strings.Contains(out, "fathom pin renamed · TestRoundHalfUp ") {
		t.Fatalf("git commit -q: no pin line from the stamped HEAD: %q", out)
	}
	if b := last(t); b["status"] != pinLost || b["commit"] != pinHead(t, repo) {
		t.Fatalf("git commit -q: beat %v, want lost on HEAD %s", b, pinHead(t, repo))
	}
	if n := stampFiles(t); n != 0 {
		t.Fatalf("the post hook left %d stamp(s); a HEAD stamp is single-use", n)
	}

	// `--amend`, quietly: the amended commit (sharing the stamp's parent) is
	// the one checked.
	writeRepoFile(t, repo, "money/round_test.go", pinTestHeader+strings.Replace(pinHalfUpTest, "TestRoundHalfUp(", "TestRoundHalfUpRenamed(", 1)+"\n"+strings.Replace(pinDownTest, "TestRoundDown(", "TestRoundDownRenamed(", 1))
	out = step(t, repo, "git commit -q --amend -am amended", func() string { return pinCommitAll(t, repo, "amended", "-q", "--amend") })
	if b := last(t); b["status"] != pinLost || b["commit"] != pinHead(t, repo) || b["tests"] != "TestRoundDown,TestRoundHalfUp" {
		t.Fatalf("--amend: beat %v, want lost TestRoundDown,TestRoundHalfUp on the amended HEAD %s", b, pinHead(t, repo))
	}
	if !strings.Contains(out, "TestRoundDown ") {
		t.Fatalf("--amend: no pin line for the amended commit: %q", out)
	}

	// Two commits inside one compound command: both checked, oldest first.
	before := len(pinBeats(t))
	var first string
	step(t, repo, `echo a >> NOTES.md && git add -A && git commit -qm a && echo b >> NOTES.md && git commit -qam b && echo done`, func() string {
		writeRepoFile(t, repo, "NOTES.md", "a\n")
		pinCommitAll(t, repo, "a", "-q")
		first = pinHead(t, repo)
		writeRepoFile(t, repo, "NOTES.md", "a\nb\n")
		pinCommitAll(t, repo, "b", "-q")
		return "done\n"
	})
	beats := pinBeats(t)[before:]
	if len(beats) != 2 || beats[0]["commit"] != first || beats[1]["commit"] != pinHead(t, repo) || beats[0]["status"] != pinKept || beats[1]["status"] != pinKept {
		t.Fatalf("compound command: beats %v, want kept on %s then on HEAD", beats, first)
	}

	// `git -C <dir> commit`, run from the repository's parent.
	writeRepoFile(t, repo, "NOTES.md", "c\n")
	step(t, filepath.Dir(repo), "git -C "+filepath.Base(repo)+" commit -qam c", func() string { return pinCommitAll(t, repo, "c", "-q") })
	if b := last(t); b["status"] != pinKept || b["commit"] != pinHead(t, repo) {
		t.Fatalf("git -C: beat %v, want kept on HEAD %s", b, pinHead(t, repo))
	}

	// `--porcelain` is a dry run: HEAD does not move, nothing to check.
	writeRepoFile(t, repo, "NOTES.md", "d\n")
	step(t, repo, "git commit --porcelain -am d", func() string { return "M  NOTES.md\n" })
	if b := last(t); b["status"] != pinSkipped || b["reason"] != string(pinSkipNoCommit) || b["commit"] != "" {
		t.Fatalf("--porcelain: beat %v, want skipped/%s with no commit", b, pinSkipNoCommit)
	}
	pinGit(t, repo, "checkout", "-q", "--", "NOTES.md")

	// HEAD moved, and not by a commit on top of the stamp: nothing checked.
	step(t, repo, "git commit -qam nothing-staged; git checkout -q HEAD~2", func() string {
		pinGit(t, repo, "checkout", "-q", "HEAD~2")
		return ""
	})
	if b := last(t); b["status"] != pinSkipped || b["reason"] != string(pinSkipHeadMoved) || b["commit"] != "" {
		t.Fatalf("checkout: beat %v, want skipped/%s with no commit", b, pinSkipHeadMoved)
	}

	// No stamp (the pre hook did not run): today's behaviour, no_summary.
	writeRepoFile(t, repo, "NOTES.md", "e\n")
	pinCommitAll(t, repo, "e", "-q")
	runPinPostHookAt(t, repo, "git commit -qam e", "")
	if b := last(t); b["status"] != pinSkipped || b["reason"] != "no_summary" {
		t.Fatalf("no stamp: beat %v, want skipped/no_summary", b)
	}

	// A stale stamp: no_summary too, and it is consumed.
	root := gitToplevelByStat(repo)
	writeHeadStamp("tr-session", root, pinHead(t, repo), time.Now().Add(-2*headStampTTL))
	writeRepoFile(t, repo, "NOTES.md", "f\n")
	pinCommitAll(t, repo, "f", "-q")
	runPinPostHookAt(t, repo, "git commit -qam f", "")
	if b := last(t); b["status"] != pinSkipped || b["reason"] != "no_summary" {
		t.Fatalf("stale stamp: beat %v, want skipped/no_summary", b)
	}
	if n := stampFiles(t); n != 0 {
		t.Fatalf("a stale stamp was left behind (%d file(s))", n)
	}
}
