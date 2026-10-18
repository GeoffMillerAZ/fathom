package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// readoutCapture records the engine's last input and readout of each hook run.
func readoutCapture(t *testing.T) *[]readoutRendered {
	t.Helper()
	var got []readoutRendered
	orig := readoutHookRendered
	t.Cleanup(func() { readoutHookRendered = orig })
	readoutHookRendered = func(in readout.Input, ro readout.Readout) { got = append(got, readoutRendered{in, ro}) }
	return &got
}

type readoutRendered struct {
	in readout.Input
	ro readout.Readout
}

func readoutEnvelope(t *testing.T, cwd, command, stdout string, response map[string]any) string {
	t.Helper()
	resp := map[string]any{"stdout": stdout, "stderr": "", "interrupted": false, "isImage": false}
	for k, v := range response {
		resp[k] = v
	}
	b, err := json.Marshal(map[string]any{
		"session_id": "s-1", "cwd": cwd, "hook_event_name": "PostToolUse", "tool_name": "Bash", "tool_use_id": "toolu_1",
		"tool_input": map[string]any{"command": command}, "tool_response": resp,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// readoutGitRepo is a work tree with files committed; it returns HEAD.
func readoutGitRepo(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Dev", "GIT_AUTHOR_EMAIL=dev@example.com",
			"GIT_COMMITTER_NAME=Dev", "GIT_COMMITTER_EMAIL=dev@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "init")
	return git("rev-parse", "HEAD")
}

// goTestPassing is go test output whose passing packages go-test collapses.
func goTestPassing() string {
	var b strings.Builder
	for i := range 30 {
		fmt.Fprintf(&b, "ok  \texample.com/shop/internal/pkg%02d\t0.%02ds\n", i, i+10)
	}
	return b.String()
}

// TestReadoutHookFillsDeclaredTestPersistedTextAndHeadStamp: the hook fills
// what the built-ins read: TR-1's declared-test answer (go-test hits on a
// declared test command), the persisted output's text (and names a miss),
// and HEAD's stamp (a git rule's receipt carries it); the row's joins ran
// and missed say which.
func TestReadoutHookFillsDeclaredTestPersistedTextAndHeadStamp(t *testing.T) {
	readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
	got := readoutCapture(t)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Taskfile.yml"), []byte("version: '3'\ntasks:\n  test:\n    cmds:\n      - go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReadoutHook(t, nil, readoutEnvelope(t, root, "task test", goTestPassing(), nil))
	if len(*got) != 1 || !(*got)[0].in.DeclaredTest || (*got)[0].ro.Rule != "go-test@1" {
		t.Fatalf("declared test command: %+v, want DeclaredTest and a go-test hit", *got)
	}

	repo := t.TempDir()
	head := readoutGitRepo(t, repo, map[string]string{"main.go": "package main\n"})
	logOut := "commit " + head + "\nAuthor: Dev <dev@example.com>\nDate:   Mon Oct 12 10:00:00 2026 -0700\n\n    init\n\n    Signed-off-by: Dev <dev@example.com>\n"
	runReadoutHook(t, nil, readoutEnvelope(t, filepath.Join(repo), "git log", logOut, nil))
	r := (*got)[1]
	if r.ro.Rule != "git-log@1" || !strings.Contains(r.ro.Receipt, "raw: repeat the command at HEAD "+head[:12]) || len(r.ro.JoinsMissed) != 0 {
		t.Fatalf("git log: rule %q receipt %q missed %v, want the HEAD stamp in the receipt", r.ro.Rule, r.ro.Receipt, r.ro.JoinsMissed)
	}

	saved := filepath.Join(t.TempDir(), "persisted.txt")
	var full strings.Builder
	for i := range 40 {
		fmt.Fprintf(&full, "pkg/file_%02d.go:%d:\treturn tokenValue\n", i%3, i+1)
	}
	if err := os.WriteFile(saved, []byte(full.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	preview := strings.Join(strings.SplitAfter(full.String(), "\n")[:10], "")
	runReadoutHook(t, nil, readoutEnvelope(t, root, "grep -rn tokenValue pkg", preview, map[string]any{"persistedOutputPath": saved}))
	runReadoutHook(t, nil, readoutEnvelope(t, root, "grep -rn tokenValue pkg", preview, map[string]any{"persistedOutputPath": saved + ".gone"}))
	if p := (*got)[2]; p.in.PersistedText != full.String() || p.ro.Rule != "grep-group@1" {
		t.Errorf("persisted read: rule %q, PersistedText %d bytes, want the file's %d", p.ro.Rule, len(p.in.PersistedText), full.Len())
	}
	if p := (*got)[3]; p.in.PersistedText != "" {
		t.Errorf("a missing persisted file filled PersistedText")
	}

	rows := readoutRows(t)
	if len(rows) != 4 {
		t.Fatalf("%d rows, want 4", len(rows))
	}
	for i, want := range []struct{ ran, missed string }{{"0", ""}, {"1", ""}, {"1", ""}, {"0", "persisted"}} {
		if m := rows[i].Meta; m["joins_ran"] != want.ran || m["joins_missed"] != want.missed {
			t.Errorf("row %d (%s): joins_ran %q joins_missed %q, want %q and %q", i, m["rule"], m["joins_ran"], m["joins_missed"], want.ran, want.missed)
		}
	}
}

// readOutlineSource is a Go file over 200 lines with its top-level symbols.
func readOutlineSource() string {
	var b strings.Builder
	b.WriteString("package shelf\n\n")
	for i := range 30 {
		fmt.Fprintf(&b, "// Count%02d counts one shelf.\nfunc Count%02d(n int) int {\n\tn++\n\tn *= 2\n\treturn n + %d\n}\n\n", i, i, i)
	}
	return b.String()
}

// TestReadoutHookReadOutlineFillsTheOutlineUnlessTheFileWasEdited: a whole
// read of a long committed source file gets the in-process outline join; a
// file this session edited, or one that differs from HEAD, stays raw; and a
// join past its deadline says so in the view and the row.
func TestReadoutHookReadOutlineFillsTheOutlineUnlessTheFileWasEdited(t *testing.T) {
	readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
	origDir := editSetDir
	editSetDir = filepath.Join(t.TempDir(), "edit-set")
	t.Cleanup(func() { editSetDir = origDir })
	got := readoutCapture(t)
	src := readOutlineSource()
	repo := t.TempDir()
	readoutGitRepo(t, repo, map[string]string{"shelf/count.go": src, "shelf/edited.go": src, "shelf/dirty.go": src})
	if err := os.WriteFile(filepath.Join(repo, "shelf", "dirty.go"), []byte(src+"// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	markFileEdited("s-1", "shelf/edited.go", time.Now())

	runReadoutHook(t, nil, readoutEnvelope(t, repo, "cat shelf/count.go", src, nil))
	c := (*got)[0]
	if c.ro.Raw || c.ro.Rule != "read-outline@1" || len(c.in.Joins["outline"]) == 0 || !strings.Contains(c.ro.Stdout, "Count00") || !strings.Contains(c.ro.Stdout, "GoFunc Count29") {
		t.Fatalf("committed file: raw=%v reason=%q rule=%q outline %v, want the outline readout", c.ro.Raw, c.ro.Reason, c.ro.Rule, c.in.Joins["outline"])
	}
	runReadoutHook(t, nil, readoutEnvelope(t, repo, "cat shelf/edited.go", src, nil))
	dirty := src + "// changed\n"
	runReadoutHook(t, nil, readoutEnvelope(t, repo, "sed -n '1,$p' shelf/dirty.go", dirty, nil))
	for i, name := range []string{"edited", "dirty"} {
		e := (*got)[1+i]
		if !e.ro.Raw || e.ro.Reason != readout.ReasonEdited || !e.in.Edited || e.in.Joins["outline"] != nil {
			t.Errorf("%s file: raw=%v reason=%q, want raw as edited with no outline computed", name, e.ro.Raw, e.ro.Reason)
		}
	}
	rows := readoutRows(t)
	if len(rows) != 3 {
		t.Fatalf("%d rows, want 3", len(rows))
	}
	for i, want := range []struct{ ran, reason string }{{"2", ""}, {"1", "edited"}, {"1", "edited"}} {
		if m := rows[i].Meta; m["rule"] != "read-outline@1" || m["joins_ran"] != want.ran || m["reason"] != want.reason || m["joins_missed"] != "" {
			t.Errorf("row %d: %v, want joins_ran %s reason %q", i, m, want.ran, want.reason)
		}
	}
}

// TestReadoutHookReadOutlineDeadline: an outline the hook could not compute
// inside its budget reads `outline: not computed (deadline)` and is named in
// the row's joins_missed.
func TestReadoutHookReadOutlineDeadline(t *testing.T) {
	src := readOutlineSource()
	dir := t.TempDir()
	rin := readout.Input{Stdout: src, Mode: readout.ModeObserve}
	cmd := lexReadout("cat shelf/count.go")
	rin.Command = cmd
	rule := &readout.Rule{Match: readout.Match{Argv0: []string{"cat"}, WholeRead: true}, Stages: []readout.Stage{{Kind: readout.StageOutline}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	j := readoutHookJoins(ctx, readoutHookInputFor("s-1", dir), dir, cmd, rule, &rin)
	if got := rin.Joins["outline"]; len(got) != 1 || got[0] != readout.OutlineDeadlineLine {
		t.Fatalf("outline join %v, want the deadline line", got)
	}
	if _, missed := readoutRowJoins(j, nil); strings.Join(missed, ",") != "outline" {
		t.Errorf("missed %v, want outline", missed)
	}
}

func lexReadout(s string) *shellcmd.Command {
	c := shellcmd.ParseCommand(s)
	return &c
}

func readoutHookInputFor(session, cwd string) harness.HookInput {
	return harness.HookInput{SessionID: session, Cwd: cwd}
}
