package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPostCommitHookPrefersDaemon (HF-2, INC-3): the post-commit refresh
// must go through the daemon — the process that owns the shared store and
// carries its own read grant. A shell ingest against that same store is the
// write path O16 rode when a wedged reader replaced a good index with a
// plausible empty one, and it is what CLAUDE.md's question-shape routing
// rule forbids.
//
// Two daemon calls are expected (wave P3 / O82): the registered-root guard
// asks list_services before ingesting — this fake answers it with no
// matching service (an empty "ok:true" body, not a list_services shape), so
// the guard fails open and the refresh still proceeds — then the ingest
// call itself. `lastTool`/`lastService`/`lastPath` record the LAST call
// (the ingest), which is what the assertions below care about.
func TestPostCommitHookPrefersDaemon(t *testing.T) {
	var called struct {
		tool    string
		service string
		path    string
		n       int
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		called.n++
		called.tool = req.Params.Name
		called.service, _ = req.Params.Arguments["service"].(string)
		called.path, _ = req.Params.Arguments["path"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"ok\":true}"}]}}`))
	}))
	defer srv.Close()

	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := cmdPostCommitHook([]string{"--dir", root, "--url", srv.URL, "--service", "svc"}, &out, &errBuf)

	if code != 0 {
		t.Errorf("a post-commit hook must never block a commit; exit %d stderr=%s", code, errBuf.String())
	}
	if called.n != 2 {
		t.Fatalf("expected exactly two daemon calls (the registered-root guard's list_services, then ingest), got %d", called.n)
	}
	if called.tool != "ingest" {
		t.Errorf("last call was %q, want the daemon's ingest tool", called.tool)
	}
	if called.service != "svc" {
		t.Errorf("hook ingested service %q, want svc", called.service)
	}
	if called.path == "" {
		t.Error("hook must pass the repo root so the daemon refreshes the right corpus")
	}
	if !strings.Contains(out.String(), "through the daemon") {
		t.Errorf("success line should name the path taken: %q", out.String())
	}
}

// TestPostCommitHookFailsOpenAndSaysWhy: a daemon that is down, wedged, or
// refusing (IT-15) must leave the commit alone and SAY so — a silent
// route around the daemon is how INC-3 stayed invisible.
func TestPostCommitHookFailsOpenAndSaysWhy(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	// Port 1 is closed: a connection error, the daemon-down shape.
	code := cmdPostCommitHook([]string{
		"--dir", root, "--url", "http://127.0.0.1:1",
		"--service", "svc", "--timeout", "1s",
	}, &out, &errBuf)

	if code != 0 {
		t.Errorf("daemon down must not block the commit; exit %d", code)
	}
	msg := errBuf.String()
	if !strings.Contains(msg, "daemon refresh failed") {
		t.Errorf("the failure line must name the failure, never route around it silently: %q", msg)
	}
}

// TestPostCommitMessageNamesNoFallback pins RT-170 (design §12.3): the
// failure line says what happened — the daemon refresh failed, nothing was
// ingested — and how to refresh through the daemon when it is back. It used
// to announce "falling back to a store-direct ingest" and then return
// without running one: a false claim made during exactly the incidents it
// reports. Both failure shapes are checked: daemon down and daemon refusing.
func TestPostCommitMessageNamesNoFallback(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"ingest refused: 0 of 12 files readable"}}`))
	}))
	defer refusing.Close()

	for _, url := range []string{"http://127.0.0.1:1", refusing.URL} {
		root := t.TempDir()
		var out, errBuf bytes.Buffer
		if code := cmdPostCommitHook([]string{
			"--dir", root, "--url", url, "--service", "svc", "--timeout", "1s",
		}, &out, &errBuf); code != 0 {
			t.Errorf("%s: exit %d, want 0 (I9)", url, code)
		}
		msg := errBuf.String()
		for _, want := range []string{"daemon refresh failed", "nothing was ingested", "post-commit-hook --dir " + root, `"svc"`} {
			if !strings.Contains(msg, want) {
				t.Errorf("%s: failure line lacks %q: %q", url, want, msg)
			}
		}
		// The temp root's own name contains this test's name; judge the
		// line's words, not the path it quotes.
		words := strings.ToLower(strings.ReplaceAll(msg, root, "<root>"))
		for _, lie := range []string{"falling back", "fallback", "store-direct"} {
			if strings.Contains(words, lie) {
				t.Errorf("%s: failure line announces a fallback it does not run (%q): %q", url, lie, msg)
			}
		}
	}
}

// TestPostCommitHookRefusalIsReported: the daemon answering with a JSON-RPC
// error (IT-15's refusal is exactly that shape) is a failure, not a
// success — reporting it as refreshed would re-create the false green.
func TestPostCommitHookRefusalIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"ingest refused: 0 of 12 files readable"}}`))
	}))
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdPostCommitHook([]string{
		"--dir", t.TempDir(), "--url", srv.URL,
		"--service", "svc", "--timeout", "2s",
	}, &out, &errBuf)
	_ = code
	if strings.Contains(out.String(), "through the daemon") {
		t.Errorf("a refused ingest must not report success: %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "refused") {
		t.Errorf("the refusal's cause must survive to the operator: %q", errBuf.String())
	}
}
