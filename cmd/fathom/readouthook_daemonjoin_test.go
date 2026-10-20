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
// caller gives up.
func joinDaemon(t *testing.T, repo string, slow bool) *httptest.Server {
	t.Helper()
	name := strings.ToLower(filepath.Base(repo))
	ident := handleIdentity(func(*http.Request) (map[string]svcInfo, error) {
		return map[string]svcInfo{name: {symbols: 42, langs: "go", path: repo}}, nil
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			ident.ServeHTTP(w, r)
			return
		}
		if slow {
			// The server sees the client leave only once the body is read.
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
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
	return srv
}

// joinRichLines runs the readout hook in rich mode over a failing go build
// in repo and returns its injected lines and how long it took.
func joinRichLines(t *testing.T, repo string) ([]string, time.Duration) {
	t.Helper()
	payload := compileFailureEnvelope(t, t.TempDir(), "cd "+repo+" && go build ./...", compileErrorsOutput)
	began := time.Now()
	code, out, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, payload)
	took := time.Since(began)
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
	return strings.Split(doc.Spec.Context, "\n"), took
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
		t.Setenv("FATHOM_HTTP", joinDaemon(t, repo, false).URL)
		// A frozen clock and a long cap: a loaded gate's git reads never
		// spend the join's share of the ceiling.
		origClock, origCap := readoutHookClock, daemonJoinCap
		fixed := time.Now()
		readoutHookClock = func() time.Time { return fixed }
		daemonJoinCap = time.Minute
		t.Cleanup(func() { readoutHookClock, daemonJoinCap = origClock, origCap })

		lines, _ := joinRichLines(t, repo)
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
		t.Setenv("FATHOM_HTTP", joinDaemon(t, repo, true).URL)
		lines, took := joinRichLines(t, repo)
		if took >= readoutHookCeiling {
			t.Errorf("the hook took %v, want it inside its %v ceiling", took, readoutHookCeiling)
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
