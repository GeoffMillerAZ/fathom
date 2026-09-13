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
	if called.n != 1 {
		t.Fatalf("expected exactly one daemon call, got %d", called.n)
	}
	if called.tool != "ingest" {
		t.Errorf("hook called %q, want the daemon's ingest tool", called.tool)
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
// fallback to the store-direct path is how INC-3 stayed invisible.
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
		t.Errorf("the fallback must name the failure, never route around it silently: %q", msg)
	}
	if !strings.Contains(msg, "IT-15") {
		t.Errorf("the fallback line must say the store-direct path still refuses an empty overwrite: %q", msg)
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
