package main

// postcommithook_remap_test.go — wave P3 defect 3(b) / O82
// (.fathom/observations.md): the post-commit-hook verb must skip with a
// message, never call the daemon's ingest tool at all, when its OWN repo
// root differs from the service's recorded root. Relying only on the
// ingest tool's own remap refusal (defect 3(a)) would still print a
// confusing "daemon refresh failed ... falling back" line for what is
// really "this hook is running inside the wrong checkout" — a hook must
// name that plainly and do nothing, not stumble into a refusal.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPostCommitHookSkipsWhenRootIsNotRegistered: a commit made inside a
// SEPARATE clone (not a linked worktree — that shape is PC-33/O22's own
// test) of a repo whose service is registered elsewhere must refresh
// nothing and name both roots plus the remap escape hatch.
func TestPostCommitHookSkipsWhenRootIsNotRegistered(t *testing.T) {
	base := t.TempDir()
	registeredRoot := filepath.Join(base, "sextant")
	if err := os.MkdirAll(registeredRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	pushClone := filepath.Join(base, "pushclone")
	if err := os.MkdirAll(pushClone, 0o755); err != nil {
		t.Fatal(err)
	}

	ingestCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Params.Name {
		case "list_services":
			inner := fmt.Sprintf(`{"services":[{"service":"svc","symbol_count":1,"languages":["go"],"path":%q}]}`, registeredRoot)
			b, _ := json.Marshal(inner)
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":%s}]}}`, b)
		case "ingest":
			ingestCalls++
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"ok\":true}"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`))
		}
	}))
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdPostCommitHook([]string{"--dir", pushClone, "--url", srv.URL, "--service", "svc"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("a post-commit hook must never block a commit; exit %d stderr=%s", code, errBuf.String())
	}
	if ingestCalls != 0 {
		t.Errorf("hook must skip the ingest call entirely when its root is not the registered one, got %d call(s)", ingestCalls)
	}
	msg := errBuf.String()
	for _, want := range []string{`"svc"`, registeredRoot, pushClone, "remap"} {
		if !strings.Contains(msg, want) {
			t.Errorf("skip line must carry %q, got %q", want, msg)
		}
	}
	if strings.Contains(out.String(), "refreshed") {
		t.Errorf("no success line when the hook skipped: %q", out.String())
	}
}

// TestPostCommitHookRefreshesWhenRootMatchesRegistered: the common case
// (the hook's repo root IS the registered root) must still refresh through
// the daemon exactly as before — the registered-root guard is a refusal for
// a mismatch, never friction on the matching path.
func TestPostCommitHookRefreshesWhenRootMatchesRegistered(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "sextant")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	ingestCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Params.Name {
		case "list_services":
			inner := fmt.Sprintf(`{"services":[{"service":"svc","symbol_count":1,"languages":["go"],"path":%q}]}`, root)
			b, _ := json.Marshal(inner)
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":%s}]}}`, b)
		case "ingest":
			ingestCalls++
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"ok\":true}"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`))
		}
	}))
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdPostCommitHook([]string{"--dir", root, "--url", srv.URL, "--service", "svc"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit %d stderr=%s", code, errBuf.String())
	}
	if ingestCalls != 1 {
		t.Errorf("expected exactly one ingest call when the root matches the registered path, got %d", ingestCalls)
	}
	if !strings.Contains(out.String(), "through the daemon") {
		t.Errorf("success line should name the path taken: %q", out.String())
	}
}
