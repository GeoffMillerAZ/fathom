package main

// edithook_teststorun_test.go — RO-43 (H4): after an edit of a Go file, one
// line naming at most 5 tests that reach it, `and N more`, the go test
// recipe as text, and whether the session ran them at this tree; silent
// when nothing reaches, a named miss past the deadline, off by default.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// reachDaemon resolves repo over GET /identity, answers blast_radius with
// *tests and every other tool with no rules; a slow one holds each /rpc
// call until the caller gives up.
func reachDaemon(t *testing.T, repo string, tests *[]string, slow bool) *httptest.Server {
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
		body, _ := io.ReadAll(r.Body)
		if slow {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
			return
		}
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Params.Name == "blast_radius" {
			keys := make([]string, len(*tests))
			for i, k := range *tests {
				keys[i] = name + "::" + k
			}
			joinRPCText(w, map[string]any{"total_symbols": 4, "tests": keys})
			return
		}
		joinRPCText(w, map[string]any{"rules": []any{}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTestsToRunLineNamesReachingTestsAndARecipe(t *testing.T) {
	const session = "tr-session"
	repo := testRunRepo(t, false)
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\ntests_to_run = true\n")
	writeRepoFile(t, repo, ".gitignore", ".fathom/\n")
	writeRepoFile(t, repo, "money/money.go", "package money\n\nfunc Round(n int) int { return n }\n")
	writeRepoFile(t, repo, "cart/cart.go", "package cart\n\nfunc Total(n int) int { return n }\n")
	commitAll(t, repo)
	edited := filepath.Join(repo, "money", "money.go")
	// Success paths run real git work in a temp repo: a generous budget, so
	// machine load never reads as a deadline miss. Only the deadline case
	// below shrinks it, against a daemon that blocks.
	origWait := testsToRunWait
	testsToRunWait = time.Minute
	defer func() { testsToRunWait = origWait }()
	markSessionEdit(session) // the tail reaches the session's last edit, so "not run" is a fact

	tests := []string{
		"cart/cart.TestD", "cart/cart.TestA", "money/money.TestRound", "cart/cart.TestC",
		"money/money.TestRoundUp", "cart/cart.TestB", "money/money.TestFloor", "money/money.BenchmarkRound",
	}
	srv := reachDaemon(t, repo, &tests, false)
	const named = "tests to run: TestFloor, TestRound, TestRoundUp, TestA, TestB and 2 more; recipe: go test -run '^(TestFloor|TestRound|TestRoundUp|TestA|TestB)$' ./cart ./money; "

	t.Run("cap, and N more, the recipe, not run at this tree", func(t *testing.T) {
		line, row := testsToRunLine(srv.URL, edited, session)
		if want := named + "at this tree: not run"; line != want {
			t.Errorf("line = %q\nwant   %q", line, want)
		}
		if row["kind"] != "line" || row["reached"] != "7" || row["named"] != "5" || row["ran"] != "not run" {
			t.Errorf("row = %v, want a line row reaching 7, naming 5, not run", row)
		}
	})

	t.Run("a recorded run at this tree", func(t *testing.T) {
		counting, _ := countingDaemon(t)
		runTestRunHook(t, "test-run", repo, counting.URL, testRunPostPayload(repo, "go test ./cart/", testRunPassingOutput))
		if line, _ := testsToRunLine(srv.URL, edited, session); !strings.HasSuffix(line, "; at this tree: 2 of 5 ran") {
			t.Errorf("after a ./cart/ run, line = %q, want 2 of 5 ran", line)
		}
		runTestRunHook(t, "test-run", repo, counting.URL, testRunPostPayload(repo, "go test ./...", testRunPassingOutput))
		if line, _ := testsToRunLine(srv.URL, edited, session); line != named+"at this tree: ran, passed" {
			t.Errorf("after a passing ./... run, line = %q", line)
		}
		// The edit changes the tree: the run no longer counts.
		writeRepoFile(t, repo, "money/money.go", "package money\n\nfunc Round(n int) int { return n + 1 }\n")
		if line, _ := testsToRunLine(srv.URL, edited, session); line != named+"at this tree: not run" {
			t.Errorf("after an edit, line = %q, want not run at the new tree", line)
		}
	})

	t.Run("silent when no test reaches", func(t *testing.T) {
		saved := tests
		tests = []string{"money/money.BenchmarkRound"}
		defer func() { tests = saved }()
		line, row := testsToRunLine(srv.URL, edited, session)
		if line != "" || row["kind"] != "silent" || row["reached"] != "0" {
			t.Errorf("line %q, row %v; want silence and a silent row", line, row)
		}
	})

	t.Run("a daemon past the deadline is the named miss", func(t *testing.T) {
		slow := reachDaemon(t, repo, &tests, true)
		testsToRunWait = 50 * time.Millisecond
		defer func() { testsToRunWait = time.Minute }()
		line, row := testsToRunLine(slow.URL, edited, session)
		if line != "tests: not computed (deadline)" || row["kind"] != "missed" || row["missed"] != "deadline" {
			t.Errorf("line %q, row %v; want the deadline miss", line, row)
		}
	})

	t.Run("not a Go file", func(t *testing.T) {
		if line, row := testsToRunLine(srv.URL, filepath.Join(repo, "README.md"), session); line != "" || row != nil {
			t.Errorf("README.md: line %q, row %v; want nothing", line, row)
		}
	})

	t.Run("rides the post-edit hook, off by default", func(t *testing.T) {
		edit := func() string {
			hookStdin(t, fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":%q}`, edited, repo, session))
			var out, errb bytes.Buffer
			if code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errb); code != 0 {
				t.Fatalf("exit %d: %s", code, errb.String())
			}
			return out.String()
		}
		out := edit()
		if !strings.Contains(out, "tests to run: TestFloor") || !strings.Contains(out, "recipe: go test -run") {
			t.Errorf("flag on: hook output %q lacks the line", out)
		}
		rows := 0
		beats, _ := readBeatLog(beatLogPath())
		for _, b := range beats {
			if b.Channel == testsToRunChannel && b.Meta["emitted"] == "true" && b.Meta["session"] == session {
				rows++
			}
		}
		if rows != 1 {
			t.Errorf("emitted tests-to-run rows = %d, want 1", rows)
		}

		writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n")
		if out := edit(); strings.Contains(out, "tests to run") {
			t.Errorf("flag off: hook output %q carries the line", out)
		}
	})
}
