package main

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testrun"
)

func TestCloneHookReachAndFreshness(t *testing.T) {
	repo := testRunRepo(t, false)
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntests_to_run = true\ntest_run = true\n")
	writeRepoFile(t, repo, "money/money.go", "package money\nfunc Round(n int) int {return n}\n")
	commitAll(t, repo)
	gitT(t, repo, "remote", "add", "origin", "https://forge.example/team/project")
	base := t.TempDir()
	clone := filepath.Join(base, "clone")
	gitT(t, base, "clone", "-q", repo, clone)
	// Transport and registry are in process: no daemon or listening socket.
	const url = "http://127.0.0.1:1"
	prev := inProcessReg.Load()
	inProcessReg.Store(&inProcessRegistry{self: url, services: func(context.Context) (map[string]svcInfo, error) { return map[string]svcInfo{"svc": {path: repo}}, nil }})
	t.Cleanup(func() { inProcessReg.Store(prev) })
	old := testRunHTTPClient
	t.Cleanup(func() { testRunHTTPClient = old })
	testRunHTTPClient = func() *http.Client {
		return &http.Client{Transport: testRunRoundTrip(func(r *http.Request) (*http.Response, error) {
			raw := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"total_symbols\":1,\"tests\":[\"svc::money/money.TestRound\"]}"}]}}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(raw)), Header: make(http.Header)}, nil
		})}
	}
	line, row := testsToRunLine(url, filepath.Join(clone, "money/money.go"), "clone-session")
	if !strings.Contains(line, "TestRound") || !strings.Contains(line, "clone at ") {
		t.Fatalf("clone reach: %q %v", line, row)
	}
	if svc, err := reachService(context.Background(), url, clone); err != nil || svc != "svc" {
		t.Fatalf("test-run identity: %q %v", svc, err)
	}
	writeRepoFile(t, clone, "money/money.go", "package money\nfunc Round(n int) int {return n+1}\n")
	code, out := runTestRunHook(t, "test-run", clone, url, testRunPostPayload(clone, "go test ./money", testRunFailingOutput))
	if code != 0 || !strings.Contains(out, "clone at ") || !strings.Contains(out, "unknown: changed in clone") {
		t.Fatalf("test-run clone freshness: %d %s", code, out)
	}
	// A later hook process, after the daemon stopped indexing the service:
	// no in-process memo and no cached answer carry over.
	resetIdentityMemo()
	identCacheDir = filepath.Join(t.TempDir(), "ident-after")
	inProcessReg.Store(&inProcessRegistry{self: url, services: func(context.Context) (map[string]svcInfo, error) { return map[string]svcInfo{}, nil }})
	line, row = testsToRunLine(url, filepath.Join(clone, "money/money.go"), "unknown-session")
	if line != "" || row["missed"] != daemonJoinMissNotIndexed {
		t.Fatalf("unresolved tests-to-run: %q %v", line, row)
	}
}

func TestUnresolvedReachIsSilent(t *testing.T) {
	res := testrun.Analyze("--- FAIL: TestA (0.00s)\nFAIL\texample.invalid/a\t0.1s\n", testrun.Evidence{})
	line := renderTestRunLine(res, testrun.Candidates{}, "")
	clone := renderTestRunLine(res, testrun.Candidates{Reach: []testrun.TestReach{{Test: "example.invalid/a.TestA"}}}, "", "clone at abcdef0, index at abcdef1")
	if !strings.Contains(clone, "unknown") || strings.Contains(clone, "no changed symbol") {
		t.Fatalf("unknown clone reach asserted absence: %q", clone)
	}
	if strings.Contains(line, "no changed symbol") {
		t.Fatalf("unresolved reach asserted absence: %q", line)
	}
}

func TestCloneFoldRequiresRootCommit(t *testing.T) {
	f := newDestFixture(t)
	target := filepath.Join(f.pathClone, "internal/widget/widget.go")
	if folded := foldCloneTarget(f.repo, target); !folded.tracked || folded.class != destClone {
		t.Fatalf("verified fold: %+v", folded)
	}
	gitT(t, f.pathClone, "checkout", "--orphan", "unrelated")
	gitT(t, f.pathClone, "add", "internal/widget/widget.go")
	gitT(t, f.pathClone, "commit", "-qm", "different root")
	if folded := foldCloneTarget(f.repo, target); folded.tracked {
		t.Fatalf("origin alone served a tracked fold: %+v", folded)
	}
}
