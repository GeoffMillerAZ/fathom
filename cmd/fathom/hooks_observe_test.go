package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// serveHookRequests exercises the real HTTP clients without binding a
// listener. Requests stay in process and each test restores the transport.
func serveHookRequests(t *testing.T, handler http.Handler) {
	t.Helper()
	orig, origCache := http.DefaultTransport, identCacheDir
	identCacheDir = t.TempDir()
	http.DefaultTransport = testRunRoundTrip(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result(), nil
	})
	t.Cleanup(func() { http.DefaultTransport, identCacheDir = orig, origCache })
}

func TestTestsToRunObserveComputesWithoutInjecting(t *testing.T) {
	repo := testRunRepo(t, false)
	widenTestsToRunWait(t)
	t.Setenv("FATHOM_HOME", t.TempDir())
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntests_to_run = \"observe\"\n")
	ident := handleIdentity(func(*http.Request) (map[string]svcInfo, error) {
		return map[string]svcInfo{"svc": {symbols: 1, langs: "go", path: repo}}, nil
	})
	serveHookRequests(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			ident.ServeHTTP(w, r)
		} else {
			joinRPCText(w, map[string]any{"total_symbols": 1, "tests": []string{"svc::money/money.TestRound"}})
		}
	}))
	text, ok := withTestsToRun("http://localhost", filepath.Join(repo, "money/money.go"), "observe-session", "existing context", true)
	if text != "existing context" || !ok {
		t.Fatalf("observe changed context: %q, %v", text, ok)
	}
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	for _, beat := range beats {
		if beat.Channel == testsToRunChannel {
			if beat.Meta["emitted"] != "false" || beat.Meta["named"] != "1" || beat.Meta["tests"] != "money.TestRound" {
				t.Fatalf("observe row = %+v", beat)
			}
			return
		}
	}
	t.Fatal("observe logged no tests-to-run row")
}

func TestPromptDefinitionsObserveComputesWithoutInjecting(t *testing.T) {
	repo := testRunRepo(t, false)
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv("CLAUDE_PROJECT_DIR", repo)
	serveHookRequests(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			_ = json.NewEncoder(w).Encode(identityResult{Schema: 1, Service: "svc", Root: repo, Path: repo})
			return
		}
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name == "find_symbol" {
			joinRPCText(w, map[string]any{"matched_via": "exact", "symbols": []any{map[string]any{
				"name": "Round", "kind": "func", "file_path": "money/money.go", "line": 3,
			}}})
			return
		}
		joinRPCText(w, map[string]any{})
	}))
	promptHookStdin(t, repo, "check `Round`")
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\nprompt_definitions = \"observe\"\n")
	var out, errb strings.Builder
	if code := cmdPromptHook([]string{"--url", "http://localhost"}, &out, &errb); code != 0 || strings.Contains(out.String(), "defined:") {
		t.Fatalf("observe exit=%d output=%q stderr=%q", code, out.String(), errb.String())
	}
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	for _, beat := range beats {
		if beat.Channel == "prompt-definitions" {
			if beat.Meta["emitted"] != "false" || beat.Meta["definitions"] != "1" {
				t.Fatalf("observe row = %+v", beat)
			}
			return
		}
	}
	t.Fatal("observe logged no prompt-definitions row")
}

func TestClaimCheckObserveComputesWithoutInjecting(t *testing.T) {
	repo := testRunRepo(t, true)
	widenClaimCheckBudget(t)
	t.Setenv("FATHOM_HOME", t.TempDir())
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\nclaim_check = \"observe\"\n")
	markSessionEdit("observe-session")
	emitBeat(testRunChannel, "fail", firingMeta(map[string]string{
		"invocation": "go test", "result": "fail", "head": "h1", "tree": "t1",
	}, "observe-session", ""))
	if line := checkStopClaims(claimCheckInput{Root: repo, Session: "observe-session", Message: "All tests pass."}); line != "" {
		t.Fatalf("observe injected %q", line)
	}
	rows := claimCheckRows(t)
	if len(rows) != 1 || rows[0]["verdicts"] != "failed" || rows[0]["emitted"] != "false" {
		t.Fatalf("observe row = %+v, want computed failed verdict and emitted=false", rows)
	}
}

func TestObserveHooksAreRegisteredByInitAndReconcile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOME", t.TempDir())
	repo := t.TempDir()
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\nclaim_check = \"observe\"\nread_outline_hint = \"observe\"\n")
	targets, err := initTargetsFor(harness.ClaudeCode, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	spec, _ := initWiringSpecFor(repo, "svc", testBinPath, targets, hooksScopeProject)
	for _, marker := range []string{"stop-hook --event subagent-stop", "readout-hook --event pre-read"} {
		found := false
		for _, key := range spec.Keys {
			found = found || strings.Contains(key.Body, marker)
		}
		if !found {
			t.Errorf("init lacks %s for observe", marker)
		}
	}
	missing := strings.Join(analyzeWiring(repo).Missing, "\n")
	if !strings.Contains(missing, "SubagentStop[") || !strings.Contains(missing, "PreToolUse[Read]") {
		t.Errorf("reconcile missing rows = %q", missing)
	}
}

func TestGatePreflightObserveSuppressesDeadlineLine(t *testing.T) {
	repo := testRunRepo(t, false)
	t.Setenv("FATHOM_HOME", t.TempDir())
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ngate_preflight = \"observe\"\n")
	orig := gatePreflightToplevel
	gatePreflightToplevel = func(context.Context, string) ([]byte, error) { return nil, context.DeadlineExceeded }
	t.Cleanup(func() { gatePreflightToplevel = orig })
	if line := gatePreflightLine(repo, repo, "git push", "observe-session", ""); line != "" {
		t.Fatalf("observe injected %q", line)
	}
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	for _, beat := range beats {
		if beat.Channel == gatePreflightChannel {
			if beat.Meta["emitted"] != "false" || beat.Meta["verify"] != "not checked (deadline)" {
				t.Fatalf("observe row = %+v", beat)
			}
			return
		}
	}
	t.Fatal("observe logged no preflight row")
}

func TestReadOutlineObserveComputesWithoutInjecting(t *testing.T) {
	repo := testRunRepo(t, false)
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv("CLAUDE_PROJECT_DIR", repo)
	widenReadOutlineHintBudgets(t) // asserts what observe computes, not how fast
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\nread_outline_hint = \"observe\"\n")
	writeRepoFile(t, repo, "big.go", readOutlineHintSource(14))
	commitAll(t, repo)
	code, out := runReadOutlineHint(t, readOutlineHintEnvelope(t, repo, filepath.Join(repo, "big.go"), nil))
	if code != 0 || out != "" {
		t.Fatalf("observe exit=%d output=%q", code, out)
	}
	rows := readOutlineHintRows(t)
	if len(rows) != 1 || rows[0].Meta["symbols"] != "14" || rows[0].Meta["emitted"] != "false" {
		t.Fatalf("observe row = %+v", rows)
	}
}
