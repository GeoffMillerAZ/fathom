package main

// thread_scope_test.go — TH-6 (docs/plans/ambient-fathom.md): a note files
// on the current repo's own thread by default, and list/brief scope to the
// current repo first — other repos' items counted, never silently hidden
// (law 3), and shown on request.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// servicesDaemon fakes the daemon's list_services with one recorded corpus
// root per service — the authoritative repo↔service identity resolution
// the hooks use (serviceident.go), so a temp repo can resolve to a service
// whose id differs from its directory basename (the O15 rename case).
func servicesDaemon(t *testing.T, roots map[string]string) *httptest.Server {
	t.Helper()
	type svc struct {
		Service     string   `json:"service"`
		SymbolCount int      `json:"symbol_count"`
		Languages   []string `json:"languages"`
		Path        string   `json:"path"`
	}
	var list []svc
	for name, root := range roots {
		list = append(list, svc{Service: name, SymbolCount: 10, Languages: []string{"go"}, Path: root})
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"result": map[string]any{"services": list},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte("ok"))
		case "/rpc":
			_, _ = w.Write(body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestThreadNoteDefaultsToRepoThread pins TH-6 end to end: the default
// thread is the resolved service id ("general" only when nothing
// resolves; an explicit --thread still wins), the default listing hides
// other repos' items behind a named count, and --all / --all-repos show
// them; the brief names the hidden count inside its budget.
func TestThreadNoteDefaultsToRepoThread(t *testing.T) {
	threadEnv(t)
	repo := t.TempDir()
	srv := servicesDaemon(t, map[string]string{"svc-alpha": repo})
	t.Setenv("FATHOM_HTTP", srv.URL)

	note := func(args ...string) {
		t.Helper()
		var out, errb bytes.Buffer
		if code := cmdThreadNote(args, &out, &errb); code != 0 {
			t.Fatalf("note %v: exit %d: %s", args, code, errb.String())
		}
	}
	byText := func(text string) threadItem {
		t.Helper()
		for _, it := range loadThreadItems(threadsPath()) {
			if it.Text == text {
				return it
			}
		}
		t.Fatalf("no item with text %q", text)
		return threadItem{}
	}

	// The row's symbol, called directly: the default thread is the service.
	var out, errb bytes.Buffer
	if code := cmdThreadNote([]string{"where the work stands", "--repo", repo}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `on thread "svc-alpha"`) {
		t.Fatalf("the verb must say where it filed:\n%s", out.String())
	}
	if got := byText("where the work stands").Thread; got != "svc-alpha" {
		t.Fatalf("default thread = %q, want the resolved service id svc-alpha", got)
	}
	note("explicit wins", "--repo", repo, "--thread", "mine")
	if got := byText("explicit wins").Thread; got != "mine" {
		t.Fatalf("--thread must still win: got %q", got)
	}

	// Another repo's handoff, tagged the way every deposit is tagged.
	seedItem(t, "th-foreign", "handoff-note", "foreign handoff", func(it *threadItem) {
		it.Thread = "elsewhere"
		it.Pointers = map[string]string{"repo": "elsewhere"}
		it.FiledAt = time.Now().UTC().Format(time.RFC3339)
	})

	list := func(args ...string) string {
		t.Helper()
		var out, errb bytes.Buffer
		if code := cmdThreadList(args, &out, &errb); code != 0 {
			t.Fatalf("list %v: exit %d: %s", args, code, errb.String())
		}
		return out.String()
	}
	t.Run("list defaults to this repo and counts the rest", func(t *testing.T) {
		out := list("--repo", repo)
		if !strings.Contains(out, "where the work stands") || !strings.Contains(out, "explicit wins") {
			t.Fatalf("this repo's items must show:\n%s", out)
		}
		if strings.Contains(out, "foreign handoff") {
			t.Fatalf("another repo's item must be hidden by default:\n%s", out)
		}
		if !strings.Contains(out, "1 open item(s) in other repos") {
			t.Fatalf("the hidden count must be named — nothing is silently hidden (law 3):\n%s", out)
		}
	})
	t.Run("--all and --all-repos show the other repos", func(t *testing.T) {
		for _, flag := range []string{"--all", "--all-repos"} {
			out := list("--repo", repo, flag)
			if !strings.Contains(out, "foreign handoff") || !strings.Contains(out, "@elsewhere") {
				t.Fatalf("%s must show the foreign item with its repo:\n%s", flag, out)
			}
			if strings.Contains(out, "in other repos") {
				t.Fatalf("%s hides nothing, so it must count nothing:\n%s", flag, out)
			}
		}
	})
	t.Run("--thread crosses repos", func(t *testing.T) {
		if out := list("--repo", repo, "--thread", "elsewhere"); !strings.Contains(out, "foreign handoff") {
			t.Fatalf("an explicit thread is asked for by name, wherever it was filed:\n%s", out)
		}
	})
	t.Run("the brief names the hidden count", func(t *testing.T) {
		out := renderThreadBrief(loadThreadItems(threadsPath()), repo, threadBriefBudgetBytes, time.Now())
		if strings.Contains(out, "foreign handoff") {
			t.Fatalf("the brief leads with this repo; other repos' handoffs are a count:\n%s", out)
		}
		if !strings.Contains(out, "1 open item(s) in other repos") || !strings.Contains(out, "fathom thread list --all") {
			t.Fatalf("the hidden count and where it lives must be named:\n%s", out)
		}
		if len(out) > threadBriefBudgetBytes {
			t.Fatalf("brief is %d bytes against a %d budget", len(out), threadBriefBudgetBytes)
		}
		var ob, eb bytes.Buffer
		if code := cmdThreadBrief([]string{"--repo", repo, "--all-repos"}, &ob, &eb); code != 0 {
			t.Fatalf("exit %d: %s", code, eb.String())
		}
		if !strings.Contains(ob.String(), "foreign handoff") {
			t.Fatalf("--all-repos rides the other repos' items along:\n%s", ob.String())
		}
	})
	t.Run("no resolvable service files on general", func(t *testing.T) {
		other := t.TempDir()
		note("unresolved repo", "--repo", other)
		if got := byText("unresolved repo").Thread; got != "general" {
			t.Fatalf("with nothing resolved the thread is general, got %q", got)
		}
	})
}
