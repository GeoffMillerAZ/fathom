package main

// gatepreflight_test.go — RO-40 (docs/plans/readouts.md): the gate
// preflight on PreToolUse(Bash) for `git commit` and `git push`.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func preflightGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// preflightRepo is claimRepo (HEAD holds an unclaimed row, the working
// tree claims it) with a declared test task, the test-run hook and the
// preflight switched on, and a bare upstream holding HEAD.
func preflightRepo(t *testing.T) (dir, svc string) {
	t.Helper()
	isolateHookRun(t)
	t.Setenv("HOME", t.TempDir())
	isolateClaimTelemetry(t)
	oldMD := "- [ ] One {id: T-1, symbol: Ghost}\n- [x] Two {id: T-2}\n"
	newMD := "- [x] One {id: T-1, symbol: Ghost}\n- [x] Two {id: T-2}\n"
	dir, _ = claimRepo(t, oldMD, newMD)
	writeRepoFile(t, dir, "Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: [\"go test ./...\"]\n")
	writeRepoFile(t, dir, ".fathom/config.toml", "[hooks]\ntest_run = true\ngate_preflight = true\n")
	writeRepoFile(t, dir, ".gitignore", ".fathom/\nTaskfile.yml\n")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	preflightGit(t, dir, "remote", "add", "origin", remote)
	preflightGit(t, dir, "push", "-q", "-u", "origin", "HEAD")
	t.Setenv("CLAUDE_PROJECT_DIR", dir)
	return dir, strings.ToLower(filepath.Base(dir))
}

func preflightRows(t *testing.T) []map[string]string {
	t.Helper()
	var out []map[string]string
	beats, _ := readBeatLog(beatLogPath())
	for _, b := range beats {
		if b.Channel == gatePreflightChannel {
			m := map[string]string{"kind": b.Kind}
			for k, v := range b.Meta {
				m[k] = v
			}
			out = append(out, m)
		}
	}
	return out
}

func TestGatePreflightNamesARefusedClaimAndAMissingPassingRun(t *testing.T) {
	dir, svc := preflightRepo(t)
	const session = "tr-session"
	calls := 0
	refusing := fakeClaimDaemon(t, svc, "fail", &calls)
	defer refusing.Close()

	// commit: the staged diff claims T-1, which verify refuses; no gate half.
	preflightGit(t, dir, "add", "docs/plans/p.md")
	line := gatePreflightLine(dir, "", `git add -A && git commit -m "claim T-1"`, session, refusing.URL)
	for _, want := range []string{"fathom gate preflight", "verify: refused", "docs/plans/p.md [T-1] One", "not found in the index"} {
		if !strings.Contains(line, want) {
			t.Errorf("commit line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "gate:") || strings.Contains(line, "\n") || strings.Contains(line, "T-2") {
		t.Errorf("commit line %q: want one line, no gate half, no row claimed at HEAD", line)
	}
	if calls != 1 {
		t.Errorf("verify calls = %d, want 1", calls)
	}

	// push: the pushed range claims T-1, and no run passed at this tree.
	preflightGit(t, dir, "commit", "-q", "-m", "claim T-1")
	markSessionEdit(session)
	line = gatePreflightLine(dir, "", "git push origin HEAD", session, refusing.URL)
	if !strings.Contains(line, "verify: refused") || !strings.Contains(line, "[T-1]") || !strings.Contains(line, "gate: no passing run at this tree in this session") {
		t.Errorf("push line = %q, want the refused row and the missing passing run", line)
	}

	// A passing run at this head and tree, and a verify that passes: silent.
	srv, _ := countingDaemon(t)
	runTestRunHook(t, "test-run", dir, srv.URL, testRunPostPayload(dir, "go test ./...", testRunPassingOutput))
	if row := lastTestRunRow(t); row["result"] != "pass" || row["head"] == "" || row["tree"] == "" {
		t.Fatalf("test-run row = %v, want a pass with head and tree", row)
	}
	passing := fakeClaimDaemon(t, svc, "pass", &calls)
	defer passing.Close()
	if line := gatePreflightLine(dir, "", "git push", session, passing.URL); line != "" {
		t.Errorf("both halves hold, yet the line = %q", line)
	}

	// Lexed, never pattern-matched: an unreadable command or a quoted
	// mention is silent; a non-gated verb is no preflight at all.
	n := len(preflightRows(t))
	for _, cmd := range []string{`git commit -m "unterminated`, `echo "git push"`, "git status", "git log --oneline"} {
		if got := gatePreflightLine(dir, "", cmd, session, refusing.URL); got != "" {
			t.Errorf("%q gave %q, want silence", cmd, got)
		}
	}
	if got := len(preflightRows(t)); got != n {
		t.Errorf("non-gated commands logged %d rows", got-n)
	}

	// A verify past the deadline reads not checked (deadline).
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		http.NotFound(w, r)
	}))
	defer slow.Close()
	orig := gatePreflightVerifyCap
	gatePreflightVerifyCap = 50 * time.Millisecond
	defer func() { gatePreflightVerifyCap = orig }()
	preflightGit(t, dir, "reset", "-q", "--soft", "HEAD~1") // the claim is staged again
	if line := gatePreflightLine(dir, "", "git commit -m x", session, slow.URL); !strings.Contains(line, "verify: not checked (deadline)") {
		t.Errorf("slow verify line = %q, want verify: not checked (deadline)", line)
	}

	rows := preflightRows(t)
	if len(rows) < 4 || rows[0]["verb"] != "commit" || rows[0]["refused"] != "1" || rows[1]["gate"] != "missing" || rows[2]["kind"] != "silent" || rows[2]["gate"] != "passing" {
		t.Errorf("preflight rows = %v", rows)
	}

	// Off by default: the same refused claim says nothing.
	gatePreflightVerifyCap = orig
	writeRepoFile(t, dir, ".fathom/config.toml", "[hooks]\ntest_run = true\n")
	if line := gatePreflightLine(dir, "", "git commit -m x", session, refusing.URL); line != "" {
		t.Errorf("gate_preflight off, yet the line = %q", line)
	}
}

// TestGatePreflightRidesThePreToolUseBashHook: the line reaches the agent
// through `edit-hook --pre` on Bash, as advisory context, never a deny.
func TestGatePreflightRidesThePreToolUseBashHook(t *testing.T) {
	dir, svc := preflightRepo(t)
	calls := 0
	srv := fakeClaimDaemon(t, svc, "fail", &calls)
	defer srv.Close()
	preflightGit(t, dir, "add", "docs/plans/p.md")
	payload, _ := json.Marshal(map[string]any{"session_id": "tr-session", "cwd": dir, "hook_event_name": "PreToolUse",
		"tool_name": "Bash", "tool_input": map[string]any{"command": "git commit -m claim"}})
	restore := pipeStdin(t, string(payload))
	defer restore()
	var out, errb bytes.Buffer
	if code := runWithStdin([]string{"edit-hook", "--pre", "--url", srv.URL}, bytes.NewReader(payload), &out, &errb); code != 0 {
		t.Fatalf("edit-hook --pre exit %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "additionalContext") || !strings.Contains(got, "verify: refused") || strings.Contains(got, "permissionDecision") {
		t.Errorf("edit-hook --pre output = %q, want the preflight as advisory context and no deny", got)
	}
}
