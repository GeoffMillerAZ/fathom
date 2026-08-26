package custodian

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// CC-14: every tool the fathom stanza names must be served by the
// running daemon — "instructed means true" applied to fathom's own
// instructions. The registry is injected so no daemon is needed here;
// the three outcomes are holds / violated-with-file:line / unable
// (daemon down is NEVER a violation, law 3).
func TestStanzaCurrentConcern(t *testing.T) {
	root := t.TempDir()
	stanza := "# repo\n" +
		"\n" +
		"<!-- fathom:begin v3 sha256:0000 -->\n" +
		"## fathom code intelligence\n" +
		"\n" +
		"Use `find_symbol`, `references`, and `verify_plan`; refresh via the `ingest` tool (`{\"service\":\"x\"}`); run `fathom thread brief`.\n" +
		"<!-- fathom:end -->\n" +
		"\n" +
		"Outside the stanza `not_a_tool` is prose, not an instruction about fathom.\n"
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), stanza)
	writeTestFile(t, filepath.Join(root, "AGENTS.md"), stanza)
	c := Concern{
		ID: "stanza_current", Want: "every tool the stanza names is served",
		Check: Check{Kind: KindStanzaToolsServed},
	}

	// Everything named is served → holds, and the detail says what was
	// compared against what.
	calls := 0
	served := func(url string) ([]string, error) {
		calls++
		return []string{"outline", "ingest", "verify_plan", "references", "find_symbol"}, nil
	}
	fs := Audit(root, []Concern{c}, WithToolsFetcher(served), WithDaemonURL("http://daemon.test"))
	if len(fs) != 1 || fs[0].Status != StatusHolds {
		t.Fatalf("all-served stanza must hold, got %+v", fs)
	}
	if !strings.Contains(fs[0].Detail, "4 tool name") || !strings.Contains(fs[0].Detail, "http://daemon.test") {
		t.Fatalf("holds detail must count names and name the daemon, got %q", fs[0].Detail)
	}
	if calls != 1 {
		t.Fatalf("registry fetched %d times, want exactly 1", calls)
	}

	// A pruned tool → violated with file:line → name evidence per
	// occurrence, sorted by file.
	pruned := func(string) ([]string, error) {
		return []string{"find_symbol", "references", "ingest"}, nil
	}
	fs = Audit(root, []Concern{c}, WithToolsFetcher(pruned))
	if fs[0].Status != StatusViolated {
		t.Fatalf("pruned tool must violate, got %+v", fs[0])
	}
	wantEv := []string{"AGENTS.md:6 → verify_plan", "CLAUDE.md:6 → verify_plan"}
	if len(fs[0].Evidence) != len(wantEv) || fs[0].Evidence[0] != wantEv[0] || fs[0].Evidence[1] != wantEv[1] {
		t.Fatalf("evidence must be file:line → tool, got %v want %v", fs[0].Evidence, wantEv)
	}
	if !strings.Contains(fs[0].Detail, "verify_plan") {
		t.Fatalf("violated detail must name the missing tool, got %q", fs[0].Detail)
	}

	// Daemon unreachable → unable-to-check naming the URL, never violated.
	down := func(string) ([]string, error) { return nil, errors.New("connection refused") }
	fs = Audit(root, []Concern{c}, WithToolsFetcher(down), WithDaemonURL("http://127.0.0.1:1"))
	if fs[0].Status != StatusUnable {
		t.Fatalf("daemon down must be unable-to-check, got %+v", fs[0])
	}
	if !strings.Contains(fs[0].Detail, "http://127.0.0.1:1") || !strings.Contains(fs[0].Detail, "connection refused") {
		t.Fatalf("unable detail must name the URL and the error, got %q", fs[0].Detail)
	}

	// An EMPTY registry (a fetcher that answers but names nothing) is the
	// same absence of evidence — unable naming the URL, never eight false
	// violations.
	empty := func(string) ([]string, error) { return nil, nil }
	fs = Audit(root, []Concern{c}, WithToolsFetcher(empty), WithDaemonURL("http://daemon.test"))
	if fs[0].Status != StatusUnable {
		t.Fatalf("empty registry must be unable-to-check, got %+v", fs[0])
	}
	if !strings.Contains(fs[0].Detail, "http://daemon.test") || !strings.Contains(fs[0].Detail, "empty registry") || len(fs[0].Evidence) != 0 {
		t.Fatalf("empty-registry detail must name the URL and the shape with no evidence, got %+v", fs[0])
	}

	// No stanza anywhere → holds vacuously and the registry is never asked.
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "# plain\n`find_symbol` outside any stanza\n")
	writeTestFile(t, filepath.Join(root, "AGENTS.md"), "# plain\n")
	asked := false
	fs = Audit(root, []Concern{c}, WithToolsFetcher(func(string) ([]string, error) { asked = true; return nil, nil }))
	if fs[0].Status != StatusHolds || !strings.Contains(fs[0].Detail, "no fathom stanza") {
		t.Fatalf("no stanza must hold naming the absence, got %+v", fs[0])
	}
	if asked {
		t.Fatal("registry must not be fetched when no stanza names a tool")
	}

	// A corrupted marker pair is unable-to-check naming the file — the
	// audit never guesses at a broken region.
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "<!-- fathom:begin -->\n`find_symbol`\n")
	fs = Audit(root, []Concern{c}, WithToolsFetcher(served))
	if fs[0].Status != StatusUnable || !strings.Contains(fs[0].Detail, "CLAUDE.md") {
		t.Fatalf("corrupt markers must be unable naming the file, got %+v", fs[0])
	}
}

// The v3 template's "Opening moves" lines name tools BARE — find_callers,
// pack_context, verify_plan — so a bare underscore-bearing token inside
// the marker region is policed exactly like a backticked one. Bare
// single words (obligations, references, outline) are NOT: they are
// prose as often as tools, and a false violation teaches people to
// ignore the custodian. A name cited both ways on one line is one ref.
func TestStanzaBareUnderscoreTokensPoliced(t *testing.T) {
	root := t.TempDir()
	stanza := "<!-- fathom:begin v3 sha256:0000 -->\n" +
		"## fathom code intelligence\n" +
		"\n" +
		"Resolve via `find_symbol`, `references`, `pack_context`.\n" +
		"\n" +
		"Opening moves by task shape (first 2-3 calls, then edit):\n" +
		"- debug/trace: find_symbol -> find_callers/references -> symbol_context\n" +
		"- feature/change: repo_map -> pack_context(concern) -> references\n" +
		"- review/verify: verify_plan -> obligations -> references on changed symbols\n" +
		"- refactor: references -> outline -> symbol_context\n" +
		"\n" +
		"Obligations are injected on every edit; capabilities and ingest are verbs here, not tool cites.\n" +
		"<!-- fathom:end -->\n"
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), stanza)
	c := Concern{
		ID: "stanza_current", Want: "every tool the stanza names is served",
		Check: Check{Kind: KindStanzaToolsServed},
	}

	// The registry lacks find_callers and symbol_context (bare, with an
	// underscore → policed) and lacks obligations and outline (bare single
	// words → NOT policed). Only the underscore tokens surface.
	reg := func(string) ([]string, error) {
		return []string{"find_symbol", "references", "pack_context", "repo_map", "verify_plan"}, nil
	}
	fs := Audit(root, []Concern{c}, WithToolsFetcher(reg), WithDaemonURL("http://daemon.test"))
	if fs[0].Status != StatusViolated {
		t.Fatalf("bare underscore token not served must violate, got %+v", fs[0])
	}
	wantEv := []string{
		"CLAUDE.md:7 → find_callers",
		"CLAUDE.md:7 → symbol_context",
		"CLAUDE.md:10 → symbol_context",
	}
	if strings.Join(fs[0].Evidence, "|") != strings.Join(wantEv, "|") {
		t.Fatalf("evidence must list bare underscore tokens per line, got %v want %v", fs[0].Evidence, wantEv)
	}
	for _, unpoliced := range []string{"obligations", "outline", "capabilities", "ingest"} {
		if strings.Contains(fs[0].Detail, unpoliced) {
			t.Fatalf("bare single word %q must not be policed, detail %q", unpoliced, fs[0].Detail)
		}
	}
	if !strings.Contains(fs[0].Detail, "2 of 7 stanza tool name(s)") {
		t.Fatalf("detail must count unique names (find_symbol references pack_context find_callers symbol_context repo_map verify_plan), got %q", fs[0].Detail)
	}

	// Serve everything the underscore tokens name → holds, still 7 names.
	all := func(string) ([]string, error) {
		return []string{"find_symbol", "references", "pack_context", "repo_map", "verify_plan", "find_callers", "symbol_context"}, nil
	}
	fs = Audit(root, []Concern{c}, WithToolsFetcher(all))
	if fs[0].Status != StatusHolds || !strings.Contains(fs[0].Detail, "7 tool name(s)") {
		t.Fatalf("all bare tokens served must hold naming 7 tools, got %+v", fs[0])
	}
}

// The default fetcher reads the daemon's tools/list on the agent surface,
// falling back to the bare /rpc for a daemon that predates scoped
// surfaces; names come back sorted.
func TestFetchServedToolsParsesRegistry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rpc/agent" {
			http.Error(w, "unknown surface", http.StatusNotFound)
			return
		}
		if r.URL.Path != "/rpc" || r.Method != http.MethodPost {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"references"},{"name":"find_symbol"}]}}`))
	}))
	defer srv.Close()

	tools, err := FetchServedTools(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 || tools[0] != "find_symbol" || tools[1] != "references" {
		t.Fatalf("want sorted [find_symbol references], got %v", tools)
	}

	if _, err := FetchServedTools("http://127.0.0.1:1"); err == nil {
		t.Fatal("unreachable daemon must error")
	}
}

// An HTTP 200 whose body is not a registry — result:null, a bare `{}`,
// an empty tools array, nameless entries, or some other service's HTML
// on the port — is an ERROR naming the URL and the shape seen, never an
// empty registry: an empty registry would report every stanza tool as
// violated, and none of these bodies is evidence the stanza is false.
func TestFetchServedToolsRefusesEmptyOrMalformedRegistry(t *testing.T) {
	cases := []struct {
		name, body, shape string
	}{
		{"result-null", `{"jsonrpc":"2.0","id":1,"result":null}`, "no result"},
		{"empty-object", `{}`, "no result"},
		{"no-tools-array", `{"jsonrpc":"2.0","id":1,"result":{}}`, "no tools array"},
		{"empty-tools", `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`, "empty tools array"},
		{"nameless-tools", `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"description":"x"}]}}`, "no name"},
		{"wrong-service", `<html><body>login</body></html>`, "not JSON-RPC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			tools, err := FetchServedTools(srv.URL)
			if err == nil {
				t.Fatalf("body %q must error, got tools %v", tc.body, tools)
			}
			if !strings.Contains(err.Error(), srv.URL) {
				t.Fatalf("error must name the URL %s, got %v", srv.URL, err)
			}
			if !strings.Contains(err.Error(), tc.shape) {
				t.Fatalf("error must name the shape %q, got %v", tc.shape, err)
			}

			// End to end through the audit: unable-to-check, zero evidence.
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "CLAUDE.md"),
				"<!-- fathom:begin -->\nUse `find_symbol`, `references`, `symbol_context`, `outline`, `repo_map`, `pack_context`, `verify_plan`, `ingest`.\n<!-- fathom:end -->\n")
			c := Concern{ID: "stanza_current", Want: "w", Check: Check{Kind: KindStanzaToolsServed}}
			fs := Audit(root, []Concern{c}, WithToolsFetcher(FetchServedTools), WithDaemonURL(srv.URL))
			if fs[0].Status != StatusUnable || len(fs[0].Evidence) != 0 {
				t.Fatalf("malformed registry must be unable with no evidence, got %+v", fs[0])
			}
			if !strings.Contains(fs[0].Detail, srv.URL) || !strings.Contains(fs[0].Detail, "8 tool name(s)") {
				t.Fatalf("unable detail must name the URL and what went uncompared, got %q", fs[0].Detail)
			}
		})
	}
}
