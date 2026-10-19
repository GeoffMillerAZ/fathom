package main

// hookdir_test.go — O1230: a hook reads the directory the tool call acts in
// (a leading cd, a file's directory), not only the session's cwd, which in
// a subagent working in a scratch clone is the parent's project.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestHookEffectiveDirFollowsALeadingCd(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	sub := filepath.Join(a, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// gitC set: the case is the directory `git -C <gitC>` acts in
	// (hookGitDir), a relative one against the leading cd's target.
	cases := []struct {
		name, command, file, want string
		resolved                  bool
		gitC                      string
	}{
		{"a leading cd", "cd " + b + " && go test ./...", "", b, true, ""},
		{"an assignment alone is a quiet prefix", "X=1; cd " + b + " && go test ./...", "", b, true, ""},
		{"export and mkdir -p are quiet prefixes", "export GOFLAGS=-count=1 && mkdir -p out && cd " + b + " && make", "", b, true, ""},
		{"a cd behind a variable", `cd "$D" && go test ./...`, "", a, false, ""},
		{"a cd behind a command substitution", "cd $(git rev-parse --show-toplevel) && ls", "", a, false, ""},
		{"a cd then a semicolon", "cd " + b + "; make", "", b, true, ""},
		{"pushd with a redirect", "pushd " + b + " >/dev/null && ls", "", b, true, ""},
		{"cd -P", "cd -P " + b + " && ls", "", b, true, ""},
		{"a target that is not a directory", "cd /no/such/dir && ls", "", a, false, ""},
		{"a relative cd resolves against cwd", "cd sub && ls", "", sub, true, ""},
		{"a cd after a command that is not quiet", "ls && cd " + b + " && make", "", a, false, ""},
		{"an unreadable command", "cd " + b + ` && echo "unterminated`, "", a, false, ""},
		{"no cd", "go test ./...", "", a, false, ""},
		{"cd -", "cd - && ls", "", a, false, ""},
		{"a backgrounded cd", "cd " + b + " & ls", "", a, false, ""},
		{"a Read of an absolute file", "", filepath.Join(b, "f.go"), b, true, ""},
		{"a relative file resolves against cwd", "", "sub/x.go", sub, true, ""},
		{"a file whose directory is missing", "", "/no/such/dir/f.go", a, false, ""},
		{"a relative git -C resolves against the cd target", "cd sub && git -C rel push", "", filepath.Join(sub, "rel"), true, "rel"},
		{"a relative git -C with no cd resolves against cwd", "git -C rel push", "", filepath.Join(a, "rel"), true, "rel"},
		{"an absolute git -C stays as written", "cd sub && git -C " + b + " push", "", b, true, b},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.gitC != "" {
				if got := hookGitDir(c.command, c.gitC, a); got != c.want {
					t.Errorf("hookGitDir(%q, %q, A) = %q; want %q", c.command, c.gitC, got, c.want)
				}
				return
			}
			got, resolved := hookEffectiveDir(c.command, c.file, a)
			if got != c.want || resolved != c.resolved {
				t.Errorf("hookEffectiveDir(%q, %q, A) = %q, %v; want %q, %v", c.command, c.file, got, resolved, c.want, c.resolved)
			}
		})
	}
}

func TestReadoutHookScopesRootsByTheCommandsDirectory(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	cfgFor := func(root string) string {
		return fmt.Sprintf("[readouts]\nmode = \"observe\"\nroots = [%q]\n", root)
	}
	payload := func(t *testing.T) string {
		return readoutPostToolEnvelope(t, a, "cd "+b+" && git log --oneline", readoutListing(""))
	}
	t.Run("roots name the directory the command cds into", func(t *testing.T) {
		readoutHookEnv(t, cfgFor(b))
		if code, out, _, _ := runReadoutHook(t, []string{"--event", "post-tool"}, payload(t)); code != 0 || out != "" {
			t.Fatalf("exit %d, stdout %q", code, out)
		}
		if n := len(readoutRows(t)); n != 1 {
			t.Errorf("roots=[B], cwd A, `cd B && git log`: %d rows, want 1", n)
		}
	})
	t.Run("roots name only the session's cwd", func(t *testing.T) {
		readoutHookEnv(t, cfgFor(a))
		runReadoutHook(t, []string{"--event", "post-tool"}, payload(t))
		if n := len(readoutRows(t)); n != 0 {
			t.Errorf("roots=[A], cwd A, `cd B && git log`: %d rows, want 0", n)
		}
	})
}

func TestGatePreflightReadsTheRepoTheCommandPushesFrom(t *testing.T) {
	b, _ := preflightRepo(t) // the preflight is on in B, with a session that ran no test
	a := t.TempDir()         // the session's project: the preflight is off there
	gitInitTempRepo(t, a)
	writeRepoFile(t, a, "README.md", "a\n")
	commitAll(t, a)
	t.Setenv("CLAUDE_PROJECT_DIR", a)
	const session = "tr-session"
	markSessionEdit(session)
	srv, _ := countingDaemon(t)
	codec, _ := harness.CodecFor(harness.ClaudeCode)
	for _, command := range []string{"cd " + b + " && git push origin HEAD", "git -C " + b + " push origin HEAD"} {
		var out bytes.Buffer
		cmdEditHookPreCommand(codec, command, a, session, srv.URL, false, &out)
		if !strings.Contains(out.String(), "gate: no passing run at this tree in this session") {
			t.Errorf("cwd A, %q: output %q, want B's preflight line", command, out.String())
		}
	}
	var out bytes.Buffer
	cmdEditHookPreCommand(codec, "git push origin HEAD", a, session, srv.URL, false, &out)
	if out.Len() != 0 {
		t.Errorf("cwd A, a push from A: output %q, want silence (the preflight is off in A)", out.String())
	}
}

func TestSubagentStopClaimCheckReadsTheRepoTheAgentWorkedIn(t *testing.T) {
	srv, _ := countingDaemon(t)
	a, _ := claimCheckRepo(t, true) // the parent session's project (CLAUDE_PROJECT_DIR)
	b := t.TempDir()                // the agent's scratch clone
	gitInitTempRepo(t, b)
	writeRepoFile(t, b, "Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: [\"go test ./...\"]\n")
	writeRepoFile(t, b, ".fathom/config.toml", "[hooks]\ntest_run = true\nclaim_check = true\n")
	writeRepoFile(t, b, "money/money.go", "package money\n\nfunc Round(n int) int { return n + 0 }\n")
	headB := commitAll(t, b)
	const session = "tr-session"
	markSessionEdit(session)
	command := "cd " + b + " && go test ./..."

	// The agent's passing run, through the hook's own entry: its row keys
	// are B's, though the envelope's cwd is A.
	var out, errb bytes.Buffer
	if code := runWithStdin([]string{"session-hook", "--event", "test-run", "--url", srv.URL}, strings.NewReader(agentPostPayload(a, session, "agent-1", command, testRunPassingOutput)), &out, &errb); code != 0 {
		t.Fatalf("session-hook exit %d: %s", code, errb.String())
	}
	if row := lastTestRunRow(t); row["head"] == "" || !strings.HasPrefix(headB, row["head"]) || row["agent_id"] != "agent-1" {
		t.Fatalf("test-run row = %v, want agent-1's run at B's HEAD %s", row, headB)
	}

	// The agent's transcript: a Read in A, then its last Bash call in B, then
	// a tool that acts in no directory.
	tr := filepath.Join(t.TempDir(), "agent-1.jsonl")
	lines := []map[string]any{
		{"type": "assistant", "cwd": a, "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": "t1", "name": "Read", "input": map[string]any{"file_path": filepath.Join(a, "money", "money.go")}}}}},
		{"type": "user", "cwd": a, "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "package money"}}}},
		{"type": "assistant", "cwd": a, "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": "t2", "name": "Bash", "input": map[string]any{"command": command}}}}},
		{"type": "user", "cwd": a, "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "t2", "content": "ok"}}}},
		{"type": "assistant", "cwd": a, "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": "t3", "name": "Grep", "input": map[string]any{"pattern": "Round"}}}}},
		{"type": "assistant", "cwd": a, "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "Done: the tests pass."}}}},
	}
	var body bytes.Buffer
	for _, l := range lines {
		j, _ := json.Marshal(l)
		body.Write(append(j, '\n'))
	}
	if err := os.WriteFile(tr, body.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	stop := func(transcript string) string {
		m := map[string]any{"session_id": session, "cwd": a, "hook_event_name": "SubagentStop", "stop_hook_active": false,
			"agent_id": "agent-1", "agent_type": "general-purpose", "last_assistant_message": "Done: the tests pass."}
		if transcript != "" {
			m["agent_transcript_path"] = transcript
		}
		j, _ := json.Marshal(m)
		return string(j)
	}
	if got := runStopVerb(t, srv.URL, stop(tr), "--event", "subagent-stop"); got != "" {
		t.Errorf("the agent's run at B's HEAD backs its claim, yet the stop printed %q", got)
	}
	if rows := claimCheckRows(t); len(rows) == 0 || rows[len(rows)-1]["verdicts"] != "backed" {
		t.Errorf("claim-check rows = %v, want the last one backed (B's HEAD)", rows)
	}

	// Without the agent's transcript the check reads A, where the run was not.
	if got := runStopVerb(t, srv.URL, stop(""), "--event", "subagent-stop"); !strings.Contains(got, "fathom claim check") {
		t.Errorf("without the transcript the stop printed %q, want A's stale line", got)
	}
	if rows := claimCheckRows(t); rows[len(rows)-1]["verdicts"] != "stale" {
		t.Errorf("last claim-check row = %v, want stale (A's HEAD)", rows[len(rows)-1])
	}
}
