package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestClaimCheckFailThenPipedUnknownAtSameTree(t *testing.T) {
	repo := testRunRepo(t, true)
	widenClaimCheckBudget(t)
	t.Setenv("FATHOM_HOME", t.TempDir())
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\nclaim_check = true\n")
	head := commitAll(t, repo)
	serveHookRequests(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	markSessionEdit("tr-session")
	if code, _ := runTestRunHook(t, "failed-test-run", repo, "http://localhost", testRunFailurePayload(repo, "go test ./...", testRunFailingOutput)); code != 0 {
		t.Fatalf("failed-test-run exit=%d", code)
	}
	failed := lastTestRunRow(t)
	if code, _ := runTestRunHook(t, "test-run", repo, "http://localhost", testRunPostPayload(repo, "go test ./... | tail -20", testRunPassingOutput)); code != 0 {
		t.Fatalf("piped test-run exit=%d", code)
	}
	unknown := lastTestRunRow(t)
	if failed["result"] != "fail" || unknown["result"] != "unknown" || unknown["piped"] != "true" || failed["invocation"] != unknown["invocation"] || failed["head"] != unknown["head"] || failed["tree"] != unknown["tree"] || unknown["head"] != head || unknown["tree"] == "" {
		t.Fatalf("fixture rows: failed=%v unknown=%v", failed, unknown)
	}
	line := checkStopClaims(claimCheckInput{Root: repo, Session: "tr-session", Message: "All tests pass."})
	if !strings.Contains(line, "failed at "+head[:7]) || !strings.Contains(line, "TestRound") {
		t.Fatalf("failure hidden by piped rerun: %q", line)
	}
}

func TestClaimCheckUnknownRunKeepsLastDecidedVerdict(t *testing.T) {
	for _, tc := range []struct {
		name, decided, unknownHead, unknownTree string
		editAfter                               bool
		want                                    string
	}{
		{"failure then piped unknown", "fail", "h1", "t1", false, "failed"},
		{"failure before edit", "fail", "h1", "t1", true, "not checked (piped)"},
		{"failure at another head", "fail", "h2", "t1", false, "not checked (piped)"},
		{"failure at another tree", "fail", "h1", "t2", false, "not checked (piped)"},
		{"unknown without tree", "fail", "h1", "", false, "not checked (piped)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testRunRepo(t, true)
			widenClaimCheckBudget(t)
			t.Setenv("FATHOM_HOME", t.TempDir())
			writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\nclaim_check = true\n")
			const session = "claim-history"
			markSessionEdit(session)
			emitBeat(testRunChannel, tc.decided, firingMeta(map[string]string{
				"invocation": "go test", "result": tc.decided, "head": "h1", "tree": "t1", "failing": "money.TestRound",
			}, session, ""))
			beats, _ := readBeatLog(beatLogPath())
			cited := beats[len(beats)-1].At.Format("2006-01-02T15:04:05.999999999Z07:00")
			if tc.editAfter {
				markSessionEdit(session)
			}
			emitBeat(testRunChannel, "unknown", firingMeta(map[string]string{
				"invocation": "go test", "result": "unknown", "piped": "true", "head": tc.unknownHead, "tree": tc.unknownTree,
			}, session, ""))
			line := checkStopClaims(claimCheckInput{Root: repo, Session: session, Message: "All tests pass."})
			rows := claimCheckRows(t)
			if len(rows) != 1 || rows[0]["verdicts"] != tc.want {
				t.Fatalf("claim rows = %v, want %q", rows, tc.want)
			}
			if tc.want == "failed" {
				if !strings.Contains(line, "failed at h1 (failing: money.TestRound)") || rows[0]["rows"] != cited {
					t.Fatalf("lost decided failure: line=%q row=%v", line, rows[0])
				}
			} else if line != "" {
				t.Fatalf("unknown run injected %q", line)
			}
		})
	}
}

func TestClaimCheckUnmatchedCommandIsUnbacked(t *testing.T) {
	for _, result := range []string{"unknown", "fail", "pass"} {
		t.Run(result, func(t *testing.T) {
			repo := testRunRepo(t, true)
			widenClaimCheckBudget(t)
			t.Setenv("FATHOM_HOME", t.TempDir())
			writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\nclaim_check = true\n")
			markSessionEdit("claim-history")
			emitBeat(testRunChannel, result, firingMeta(map[string]string{
				"invocation": "task test", "result": result, "head": "h1", "tree": "t1",
			}, "claim-history", ""))
			line := checkStopClaims(claimCheckInput{Root: repo, Session: "claim-history", Message: "`go test ./...` passes."})
			rows := claimCheckRows(t)
			if !strings.Contains(line, "no `go test` run is recorded in this session since its last edit") || len(rows) != 1 || rows[0]["verdicts"] != "unbacked" {
				t.Fatalf("unmatched claim line=%q rows=%v", line, rows)
			}
		})
	}
}
