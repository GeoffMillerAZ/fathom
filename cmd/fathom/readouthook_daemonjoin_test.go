package main

// readouthook_daemonjoin_test.go — RO-30: the readout hook's daemon joins
// (the symbol at a compile error's identifier, the tests that reach the
// changed files) run under their own deadline inside the 1 s ceiling; a
// miss reads "index: not consulted (deadline)", never as nothing found.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// joinRPCText answers one tools/call with payload as its text content.
func joinRPCText(w http.ResponseWriter, payload any) {
	text, _ := json.Marshal(payload)
	quoted, _ := json.Marshal(string(text))
	fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":%s}]}}`, quoted)
}

// joinDaemon resolves repo to its service over GET /identity and answers
// find_symbol (Bar is defined at lib/bar.go:12) and blast_radius (three
// tests, one a benchmark). A slow daemon holds every /rpc call until the
// caller gives up and sends how long it held it on held.
func joinDaemon(t *testing.T, repo string, slow bool) (srv *httptest.Server, held <-chan time.Duration) {
	t.Helper()
	name := strings.ToLower(filepath.Base(repo))
	ident := handleIdentity(func(*http.Request) (map[string]svcInfo, error) {
		return map[string]svcInfo{name: {symbols: 42, langs: "go", path: repo}}, nil
	})
	heldc := make(chan time.Duration, 8)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			ident.ServeHTTP(w, r)
			return
		}
		if slow {
			// The server sees the client leave only once the body is read.
			_, _ = io.Copy(io.Discard, r.Body)
			began := time.Now()
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
			select {
			case heldc <- time.Since(began):
			default:
			}
			return
		}
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		switch req.Params.Name {
		case "find_symbol":
			if req.Params.Arguments["name"] == "Bar" {
				joinRPCText(w, map[string]any{"matched_via": "exact", "symbols": []any{map[string]any{"name": "Bar", "file_path": "lib/bar.go", "line": 12}}})
				return
			}
			joinRPCText(w, map[string]any{"matched_via": "none", "symbols": []any{}})
		case "blast_radius":
			joinRPCText(w, map[string]any{"total_symbols": 3, "tests": []string{
				name + "::money/money.TestRound", name + "::cart/cart.TestCheckout", name + "::cart/cart.BenchmarkCart",
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, heldc
}

// joinRichLines runs the readout hook in rich mode over a failing go build
// in repo and returns its injected lines.
func joinRichLines(t *testing.T, repo string) []string {
	t.Helper()
	payload := compileFailureEnvelope(t, t.TempDir(), "cd "+repo+" && go build ./...", compileErrorsOutput)
	code, out, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, payload)
	if code != 0 || out == "" {
		t.Fatalf("exit %d, stdout %q; want 0 and one JSON line", code, out)
	}
	var doc struct {
		Spec struct {
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	return strings.Split(doc.Spec.Context, "\n")
}

func TestDaemonJoinMissNamesTheDeadline(t *testing.T) {
	repo := t.TempDir()
	readoutGitRepo(t, repo, map[string]string{
		"go.mod":         "module example.com/shop\n\ngo 1.22\n",
		"money/money.go": "package money\n\nfunc Round(x int) int { return x }\n",
	})
	if err := os.WriteFile(filepath.Join(repo, "money", "money.go"), []byte("package money\n\nfunc Round(x, places int) int { return x + places }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("a fast daemon places the symbol and names the tests", func(t *testing.T) {
		readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
		srv, _ := joinDaemon(t, repo, false)
		t.Setenv("FATHOM_HTTP", srv.URL)
		// A long cap (the clock is readoutHookEnv's frozen one): a loaded
		// gate's daemon round trips never spend the join's deadline.
		origCap := daemonJoinCap
		daemonJoinCap = time.Minute
		t.Cleanup(func() { daemonJoinCap = origCap })

		lines := joinRichLines(t, repo)
		if len(lines) != 2 {
			t.Fatalf("lines = %q, want the compile line and the tests line", lines)
		}
		for _, want := range []string{
			"the signature of money.Round, money/money.go:3",
			"undefined: Bar, lib/bar.go:12 (4 errors)",
		} {
			if !strings.Contains(lines[0], want) {
				t.Errorf("compile line %q lacks %q", lines[0], want)
			}
		}
		if strings.Contains(lines[0], "not consulted") {
			t.Errorf("compile line %q names a miss with the daemon answering", lines[0])
		}
		if want := "tests to run: TestCheckout, TestRound; recipe: go test -run '^(TestCheckout|TestRound)$' ./cart ./money"; lines[1] != want {
			t.Errorf("tests line = %q, want %q", lines[1], want)
		}
		rows := readoutRows(t)
		if len(rows) != 1 || rows[0].Meta["rich"] != "compile_error,tests_to_run" || rows[0].Meta["rich_missed"] != "" {
			t.Errorf("rows = %+v, want one row naming both lines and no miss", rows)
		}
	})

	t.Run("a slow daemon is the named miss inside the ceiling", func(t *testing.T) {
		readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
		srv, held := joinDaemon(t, repo, true)
		t.Setenv("FATHOM_HTTP", srv.URL)
		lines := joinRichLines(t, repo)
		// The join's own wait, timed where the daemon holds the call: the
		// rest of the hook is CPU a contended race runner stretches past
		// any wall-clock bound, and the ceiling over it is
		// TestReadoutRichPastTheCeilingAddsNothing's.
		select {
		case h := <-held:
			if h >= readoutHookCeiling {
				t.Errorf("the daemon held the join %v, want it given up inside the hook's %v ceiling", h, readoutHookCeiling)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("the slow daemon never saw the join give up")
		}
		if len(lines) != 1 || !strings.HasSuffix(lines[0], "; index: not consulted (deadline)") {
			t.Fatalf("lines = %q, want the compile line ending in the named miss", lines)
		}
		if strings.Contains(lines[0], "lib/bar.go") {
			t.Errorf("compile line %q places a root the daemon never answered", lines[0])
		}
		if !strings.Contains(lines[0], "undefined: Bar, other/x.go:5 (4 errors)") {
			t.Errorf("compile line %q lost the local grouping", lines[0])
		}
		rows := readoutRows(t)
		if len(rows) != 1 || !strings.Contains(rows[0].Meta["rich_missed"], "index") {
			t.Errorf("rows = %+v, want rich_missed naming the index join", rows)
		}
	})

	t.Run("the join's own deadline", func(t *testing.T) {
		called := false
		if got := daemonJoin(readoutHookCeiling, func(context.Context) error { called = true; return nil }); got != daemonJoinMissDeadline || called {
			t.Errorf("a spent ceiling: miss %q, called %v; want deadline and no call", got, called)
		}
		var left time.Duration
		if got := daemonJoin(800*time.Millisecond, func(ctx context.Context) error {
			dl, _ := ctx.Deadline()
			left = time.Until(dl)
			return nil
		}); got != "" {
			t.Errorf("an answering join reads miss %q", got)
		}
		if left > readoutHookCeiling-800*time.Millisecond-daemonJoinReserve {
			t.Errorf("join deadline %v runs past the ceiling less its reserve", left)
		}
		if got := daemonJoin(0, func(context.Context) error { return errors.New("HTTP 500") }); got != daemonJoinMissDaemon {
			t.Errorf("a failing daemon reads miss %q, want %q", got, daemonJoinMissDaemon)
		}
		if got := daemonJoin(0, func(context.Context) error { return errNoService }); got != daemonJoinMissNotIndexed {
			t.Errorf("an unindexed root reads miss %q, want %q", got, daemonJoinMissNotIndexed)
		}
	})
}

// TestDaemonJoinPlacesNoneOfSeveralDefinitions pins indexDefinition's
// "never a guessed root": a name the index defines more than once places
// nothing, bare or under its qualifier; one hit under the qualifier places.
func TestDaemonJoinPlacesNoneOfSeveralDefinitions(t *testing.T) {
	sym := func(file string, line int) map[string]any {
		return map[string]any{"name": "x", "file_path": file, "line": line}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		switch req.Params.Arguments["name"] {
		case "Round":
			joinRPCText(w, map[string]any{"matched_via": "exact", "symbols": []any{sym("money/round.go", 3), sym("geo/round.go", 7)}})
		case "Floor":
			joinRPCText(w, map[string]any{"matched_via": "exact", "symbols": []any{sym("money/floor.go", 4), sym("money/floor_v2.go", 9)}})
		default:
			joinRPCText(w, map[string]any{"matched_via": "none", "symbols": []any{}})
		}
	}))
	t.Cleanup(srv.Close)
	ctx := context.Background()
	for _, ident := range []string{"Round", "money.Floor", "cart.Round"} {
		if d, ok, err := indexDefinition(ctx, srv.URL, "svc", ident); err != nil || ok {
			t.Errorf("%s placed %+v (ok %v, err %v); want none of several chosen", ident, d, ok, err)
		}
	}
	if d, ok, err := indexDefinition(ctx, srv.URL, "svc", "money.Round"); err != nil || !ok || d.File != "money/round.go" || d.Line != 3 {
		t.Errorf("money.Round = %+v (ok %v, err %v); want the one hit under money/", d, ok, err)
	}
}
