package main

// prompthook_relay_test.go — EW-21 / O126: the prompt hook serves nothing for
// a cross-agent relay message. A sub-agent's or a peer session's message
// reaches the session as a user-role prompt nobody typed; its prose names
// scratch paths and build artifacts, and the hook answered it with six root
// rules and "178 more eligible".

import (
	"bytes"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIsAgentRelay(t *testing.T) {
	cases := map[string]bool{
		`<cross-session-message from="worker-7">built bin/fathom, script at /tmp/x/genv.sh</cross-session-message>`: true,
		"  <teammate-message teammate_id=\"w1\" summary=\"done\">ran bin/fathom</teammate-message>":                 true,
		"[Workflow harness \u2014 computed task] The task text below was computed at runtime":                       true,
		// A relay can arrive queued behind typed text: a line that opens with the tag is one.
		"also check the diff\n<teammate-message teammate_id=\"w1\">done</teammate-message>": true,
		// Prose that merely names a tag mid-sentence is user text and is served normally.
		"why does the hook see <teammate-message> at all? fix internal/store/backend.go": false,
		"a prompt that mentions [Workflow harness in passing":                            false,
		"please fix internal/store/backend.go":                                           false,
		"":                                                                               false,
	}
	for prompt, want := range cases {
		if got := isAgentRelay(prompt); got != want {
			t.Errorf("isAgentRelay(%q) = %v, want %v", prompt, got, want)
		}
	}
}

// The relay in O126's own shape: it names bin/fathom and a scratchpad path.
// No output and no daemon call, whatever the daemon would have answered.
func TestPromptHook_AgentRelay_SilentNoDaemonCall(t *testing.T) {
	dir := t.TempDir()
	touchTree(t, dir, "bin/fathom", "internal/store/backend.go")
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the daemon must never be queried for an agent relay", 500)
	})
	defer srv.Close()

	for _, prompt := range []string{
		`<cross-session-message from="w1">rebuilt bin/fathom; script at /tmp/scratch/genv.sh; see internal/store/backend.go</cross-session-message>`,
		`<teammate-message teammate_id="w2">done: internal/store/backend.go</teammate-message>`,
	} {
		promptHookStdin(t, dir, prompt)
		var out, errBuf bytes.Buffer
		if code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
			t.Fatalf("prompt-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
		}
		if out.Len() != 0 {
			t.Errorf("expected silence for an agent relay, got %q", out.String())
		}
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("daemon queried %d time(s) for an agent relay, want 0", n)
	}
}

// A typed prompt that names only things that do not exist serves nothing and
// never reaches obligations (the O126 repro without the relay wrapper).
func TestPromptHook_NamesNothingReal_NoObligationsCall(t *testing.T) {
	dir := t.TempDir()
	touchTree(t, dir, "bin/fathom")
	var obl int32
	srv, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		if strings.Contains(buf.String(), `"name":"obligations"`) {
			atomic.AddInt32(&obl, 1)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`))
	})
	defer srv.Close()

	promptHookStdin(t, dir, "run bin/fathom on /tmp/scratch/genv.sh and schema/ghost.json")
	var out, errBuf bytes.Buffer
	if code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("prompt-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("expected silence, got %q", out.String())
	}
	if n := atomic.LoadInt32(&obl); n != 0 {
		t.Errorf("obligations queried %d time(s) for tokens that name nothing, want 0", n)
	}
}
