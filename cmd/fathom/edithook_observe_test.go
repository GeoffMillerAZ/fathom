package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// RT-131 (design §11.3): observe-only hook paths — the Read|Grep|Glob
// observer and a post-bash call with no write targets — exit before any
// git fork or daemon request; a write-shaped Bash call still reaches the
// daemon (the control that proves the probes can see).
func TestObserveOnlyHooksSkipGitAndDaemon(t *testing.T) {
	var daemonHits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		daemonHits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	// Every git fork, from anywhere in the process, lands in gitlog.
	bin := t.TempDir()
	gitlog := filepath.Join(t.TempDir(), "git.log")
	realGit := "/usr/bin/git"
	if p, err := lookPathOutside(bin, "git"); err == nil {
		realGit = p
	}
	script := "#!/bin/sh\necho \"$@\" >>" + gitlog + "\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	prevExec := gitToplevelExec
	var execs atomic.Int64
	gitToplevelExec = func(dir string) string { execs.Add(1); return prevExec(dir) }
	t.Cleanup(func() { gitToplevelExec = prevExec; resetToplevelMemo() })
	resetToplevelMemo()

	repo, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	obsLog := filepath.Join(t.TempDir(), "observed.jsonl")
	t.Setenv("FATHOM_OBSERVED_DISCOVERIES_LOG", obsLog)
	t.Setenv("CLAUDE_PROJECT_DIR", repo)

	run := func(args []string, body string) string {
		t.Helper()
		restore := pipeStdin(t, body)
		defer restore()
		var out bytes.Buffer
		if rc := cmdEditHook(append(args, "--url", srv.URL), &out, io.Discard); rc != 0 {
			t.Fatalf("edit-hook %v exited %d", args, rc)
		}
		return out.String()
	}
	gitForks := func() int {
		b, _ := os.ReadFile(gitlog)
		return strings.Count(string(b), "\n")
	}

	for _, tool := range []string{"Read", "Grep", "Glob"} {
		input := `{"file_path":"` + filepath.Join(repo, "a.go") + `"}`
		if tool != "Read" {
			input = `{"pattern":"foo","path":"` + repo + `"}`
		}
		out := run([]string{"--pre"}, `{"session_id":"s1","cwd":"`+repo+`","hook_event_name":"PreToolUse","tool_name":"`+tool+`","tool_input":`+input+`}`)
		if out != "" {
			t.Fatalf("%s observer wrote output: %q", tool, out)
		}
	}
	if b, err := os.ReadFile(obsLog); err != nil || !strings.Contains(string(b), "a.go") {
		t.Fatalf("the observer still records its pointer row: %v %s", err, b)
	}
	out := run(nil, `{"session_id":"s1","cwd":"`+repo+`","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls -la"},"tool_response":{}}`)
	if out != "" {
		t.Fatalf("a read-only post-bash wrote output: %q", out)
	}
	if n := gitForks(); n != 0 || execs.Load() != 0 {
		t.Fatalf("observe-only paths forked git %d time(s) (toplevel execs %d): %s", n, execs.Load(), mustRead(gitlog))
	}
	if n := daemonHits.Load(); n != 0 {
		t.Fatalf("observe-only paths reached the daemon %d time(s)", n)
	}

	// Control: a write-shaped post-bash does real work and reaches the daemon.
	run(nil, `{"session_id":"s1","cwd":"`+repo+`","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"echo x > a.go"},"tool_response":{}}`)
	if daemonHits.Load() == 0 {
		t.Fatal("control: a write-shaped post-bash never reached the daemon — the probe is blind")
	}
}

func mustRead(p string) string { b, _ := os.ReadFile(p); return string(b) }

// lookPathOutside finds name on PATH, skipping dir.
func lookPathOutside(dir, name string) (string, error) {
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if d == dir || d == "" {
			continue
		}
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}
