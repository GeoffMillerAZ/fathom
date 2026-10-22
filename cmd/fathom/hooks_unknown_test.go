package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testrun"
)

func TestTestsToRunUnindexedRootIsSilent(t *testing.T) {
	repo := testRunRepo(t, false)
	t.Setenv("FATHOM_HOME", t.TempDir())
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntests_to_run = true\n")
	serveHookRequests(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity" {
			t.Errorf("unindexed root asked %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(identityResult{Schema: 1})
	}))
	text, ok := withTestsToRun("http://localhost", filepath.Join(repo, "money/money.go"), "unknown-session", "", false)
	if text != "" || ok {
		t.Errorf("unindexed root injected %q, ok=%v", text, ok)
	}
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	for _, b := range beats {
		if b.Channel == testsToRunChannel {
			if b.Kind != "missed" || b.Meta["missed"] != "not indexed" || b.Meta["emitted"] != "false" {
				t.Fatalf("unindexed row = %+v", b)
			}
			return
		}
	}
	t.Fatal("unindexed root logged no miss")
}

func TestClaimCheckUnknownRunIsSilentAndNeverUnbacked(t *testing.T) {
	for _, tc := range []struct {
		name, invocation, result, piped, message, verdict string
	}{
		{"piped unknown", "go test", "unknown", "true", "All tests pass.", "not checked (piped)"},
		{"unreadable result", "go test", "unknown", "false", "All tests pass.", "not checked (piped)"},
		{"piped unknown named claim", "go test", "unknown", "true", "`go test ./...` passes.", "not checked (piped)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testRunRepo(t, true)
			t.Setenv("FATHOM_HOME", t.TempDir())
			writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\nclaim_check = true\n")
			markSessionEdit("unknown-session")
			emitBeat(testRunChannel, tc.result, firingMeta(map[string]string{
				"invocation": tc.invocation, "result": tc.result, "piped": tc.piped, "head": "h1", "tree": "t1",
			}, "unknown-session", ""))
			if line := checkStopClaims(claimCheckInput{Root: repo, Session: "unknown-session", Message: tc.message}); line != "" {
				t.Errorf("unknown run injected %q", line)
			}
			rows := claimCheckRows(t)
			if len(rows) != 1 || rows[0]["verdicts"] != tc.verdict || rows[0]["emitted"] != "false" {
				t.Fatalf("claim rows = %+v, want %q", rows, tc.verdict)
			}
			if tc.verdict == "not checked (piped)" && (rows[0]["kind"] != "not-checked" || rows[0]["reason"] != "piped") {
				t.Errorf("missing not-checked:piped outcome: %+v", rows[0])
			}
		})
	}
}

func TestClaimCheckNoRunSinceEditIsUnbacked(t *testing.T) {
	repo := testRunRepo(t, true)
	t.Setenv("FATHOM_HOME", t.TempDir())
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\nclaim_check = true\n")
	markSessionEdit("unknown-session")
	if line := checkStopClaims(claimCheckInput{Root: repo, Session: "unknown-session", Message: "All tests pass."}); !strings.Contains(line, "no test run is recorded") {
		t.Fatalf("no-run claim = %q", line)
	}
	rows := claimCheckRows(t)
	if len(rows) != 1 || rows[0]["verdicts"] != "unbacked" {
		t.Fatalf("no-run claim rows = %+v", rows)
	}
}

type seamReachIndex struct{ err error }

func (s seamReachIndex) TestImpact(string) ([]string, error) { return nil, s.err }
func (s seamReachIndex) Localize(testrun.LocalizeSeed) ([]testrun.Located, error) {
	return nil, nil
}

func TestLiveTestRunReachRequiresAnIndexAnswer(t *testing.T) {
	run := testrun.ParseTestRun(testRunFailingOutput)
	changed := []testrun.ChangedSymbol{{Ref: "money/money.Round", Name: "money.Round"}}
	for _, tc := range []struct {
		name    string
		changed []testrun.ChangedSymbol
		idx     testrun.Index
		known   bool
	}{
		{"unindexed", changed, nil, false},
		{"daemon failed", changed, seamReachIndex{errors.New("unavailable")}, false},
		{"no changed symbols queried", nil, seamReachIndex{}, false},
		{"index answered empty", changed, seamReachIndex{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testrun.FindCandidates(run, tc.changed, tc.idx)
			line := renderTestRunLine(testrun.Analyze(testRunFailingOutput, testrun.Evidence{}), c, "")
			if got := strings.Contains(line, "no changed symbol reaches"); got != tc.known {
				t.Fatalf("known=%v: %q", tc.known, line)
			}
		})
	}
}
