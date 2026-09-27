package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// TestFleetFreshnessNoLoopbackOrFork (RT-115, design §14): inside the
// daemon, a console page asks its own daemon nothing over loopback HTTP and
// forks no git per service. Freshness reads the repository's HEAD from its
// files (gitro.Head) and the indexed commit from one in-process statusz;
// the compile states, the agent surface's schema tax and the plans' HEAD
// read the same way. A CLI (no in-process daemon) still asks over HTTP.
func TestFleetFreshnessNoLoopbackOrFork(t *testing.T) {
	root := t.TempDir()
	gitInitTempRepo(t, root)
	// A fathom checkout: the plans page's measured-stale reading compares
	// the daemon with its HEAD.
	mustWrite(t, filepath.Join(root, "go.mod"), "module github.com/geoffmilleraz/fathom\n")
	runGitFootprint(t, root, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "one")
	head := strings.TrimSpace(runGitFootprint(t, root, "rev-parse", "HEAD"))

	// The daemon's own URL: any request that reaches it is a loopback.
	var loopbacks atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loopbacks.Add(1)
		http.Error(w, "loopback", http.StatusTeapot)
	}))
	t.Cleanup(ts.Close)
	var statuszBuilds atomic.Int32
	srv := mcp.NewServer(nil)
	prev := inProcessReg.Load()
	inProcessReg.Store(&inProcessRegistry{
		self: ts.URL,
		statusz: func(context.Context) StatuszReport {
			statuszBuilds.Add(1)
			return StatuszReport{StatuszBuildInfo: StatuszBuildInfo{BinaryCommit: "0123456789ab"},
				Services: []StatuszServiceRow{{Service: "pay", IndexedCommit: head, LastCompilePerLayer: map[string]string{"adr-rules": "ok"},
					TypedEdges: &StatuszTypedEdges{Failed: true, Reason: "go list failed"}}}}
		},
		rpc: srv.HandleRequest,
	})
	t.Cleanup(func() { inProcessReg.Store(prev) })

	// No git on PATH from here: a fork would fail (and is recorded).
	bin := t.TempDir()
	forks := filepath.Join(bin, "forks")
	mustWrite(t, filepath.Join(bin, "git"), "#!/bin/sh\necho \"$@\" >> "+forks+"\nexit 1\n")
	if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}

	hc := consoleRPCClient(0)
	fresh, indexed, repoHead := fleetFreshness(hc, ts.URL, "pay", root)
	if fresh != "fresh" || indexed != head || repoHead != head {
		t.Fatalf("fleetFreshness = %q %q %q; want fresh at %s", fresh, indexed, repoHead, head)
	}
	if got := fetchCompileStates(hc, ts.URL); got["pay"] != "adr-rules: ok" {
		t.Errorf("compile states = %v", got)
	}
	if tools, bytes := surfaceSchemaTax(hc, ts.URL); tools == 0 || bytes == 0 {
		t.Errorf("schema tax = %d tools, %d bytes; want the agent surface measured in process", tools, bytes)
	}
	pc := &consolePlans{deps: &consoleDeps{selfURL: ts.URL}, heads: map[string]plansStamp{}}
	if got := pc.head(root); got != head {
		t.Errorf("plans head = %q, want %q", got, head)
	}
	// A model build's one statusz read serves every service.
	idx := daemonStatuszRead(hc, ts.URL)
	statuszBuilds.Store(0)
	for i := 0; i < 5; i++ {
		fleetFreshnessFrom(idx, "pay", root)
	}
	if n := statuszBuilds.Load(); n != 0 {
		t.Errorf("fleetFreshnessFrom built statusz %d times; the caller's one read serves all", n)
	}

	// The projects page's typed-edge failures, the plans page's
	// measured-stale reading, the intent page's cache key and its index
	// oracle: in process, no fork.
	if got := statuszTypedFailures(ts.URL); got["pay"] != "go list failed" {
		t.Errorf("typed failures = %v", got)
	}
	pc.stale = map[string]plansStamp{}
	if !pc.daemonStale("pay", root) {
		t.Errorf("the plans page's stale reading missed a daemon at another commit than HEAD")
	}
	if k := intentAuditKey(root, "pay", 1); !strings.Contains(k, head) {
		t.Errorf("the intent audit key %q does not carry HEAD %s", k, head)
	}
	oracle := newIndexOracle(ts.URL, "pay", root, head, true, consoleRPCClient(0)).(*indexOracle)
	if _, err := oracle.resolveNames([]string{"TestNothing"}); err != nil && strings.Contains(err.Error(), "loopback") {
		t.Errorf("the intent oracle asked over loopback: %v", err)
	}

	if n := loopbacks.Load(); n != 0 {
		t.Errorf("%d loopback request(s) to the daemon's own URL", n)
	}
	if raw, err := os.ReadFile(forks); err == nil {
		t.Errorf("git forked: %s", raw)
	}

	// A CLI (no in-process registry for this URL) still reads over HTTP.
	inProcessReg.Store(nil)
	if fresh, _, _ := fleetFreshness(hc, ts.URL, "pay", root); !strings.HasPrefix(fresh, "unknown") || loopbacks.Load() == 0 {
		t.Errorf("without the in-process daemon: %q after %d requests", fresh, loopbacks.Load())
	}
}
