//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// postVerdictPauseReader pauses the first review in Services, a read only
// the service-ref hops make, after the verdict and its findings
// persistence: the first Services call must come after the review claims
// read. It waits for the request's cancellation, never for a timer.
type postVerdictPauseReader struct {
	*cancelledReviewReader
	findingsRead atomic.Bool
	early        atomic.Bool
	services     atomic.Int64
	paused       chan struct{}
}

func (b *postVerdictPauseReader) ClaimsForService(ctx context.Context, service, layer string) ([]core.SemanticClaim, error) {
	if layer == review.ClaimsLayer {
		b.findingsRead.Store(true)
	}
	return b.Client.ClaimsForService(ctx, service, layer)
}

func (b *postVerdictPauseReader) Services(ctx context.Context) ([]string, error) {
	if ctx.Err() != nil {
		b.after.Add(1)
	}
	if b.services.Add(1) == 1 {
		b.early.Store(!b.findingsRead.Load())
		close(b.paused)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return b.Client.Services(ctx)
}

// logReviewGit puts a git wrapper first on PATH that logs each command and
// runs the real git: the SARIF anchor's commit reads are git commands.
func logReviewGit(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	log := filepath.Join(dir, "commands")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + quote(log) + "\nexec " + quote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// TestReviewDiffCancelledAfterVerdictStopsWorkAndReleasesAdmission (GDEV-03,
// consumer O21's post-verdict half): an HTTP client that disconnects while
// review_diff is past its verdict — paused in the service-ref hops — gets a
// named cancellation; the handler settles, no git or counted source read
// starts afterwards, the admission slot is released, and the next review
// succeeds.
func TestReviewDiffCancelledAfterVerdictStopsWorkAndReleasesAdmission(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q")
	changed := []string{"one.go", "two.go"}
	for _, path := range changed {
		if err := os.WriteFile(filepath.Join(root, path), []byte("package sample\nfunc Value() int { return 1 }\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-qm", "base")
	head := strings.TrimSpace(git("rev-parse", "HEAD"))
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "review.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	if err := c.Store().SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "sample", Path: root, IndexedCommit: head}); err != nil {
		t.Fatal(err)
	}
	gitLog := logReviewGit(t)
	b := &postVerdictPauseReader{cancelledReviewReader: &cancelledReviewReader{Client: c}, paused: make(chan struct{})}
	srv := mcp.NewServer(b)
	reg := tenancy.NewInFlight(nil)
	budget, _ := tenancy.ResolveBudget(tenancy.Config{Budget: tenancy.GiB}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 4, Managed: true})
	srv.SetGate(tenancy.NewGate(tenancy.GateConfig{Budget: budget, Mode: tenancy.ModeEnforce, InFlight: reg}))
	handler := newHTTPHandlerLogged(srv, c, "127.0.0.1:0", nil, testDiscardLogger())
	settled := make(chan struct{}, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
		settled <- struct{}{}
	}))
	defer ts.Close()
	request := func(ctx context.Context) *http.Request {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/rpc", rpcBody(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "sample", "changed_files": changed}}}))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := request(ctx)
	done := make(chan error, 1)
	go func() {
		resp, err := ts.Client().Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		done <- err
	}()
	select {
	case <-b.paused:
	case <-time.After(waitSafety):
		t.Fatal("review never reached the service-ref hops")
	}
	if b.early.Load() {
		t.Fatal("Services was read before the findings persistence: not a post-verdict pause")
	}
	if len(reg.Snapshot()) != 1 {
		t.Fatal("review was not admitted")
	}
	before, err := os.ReadFile(gitLog)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want named cancellation, got %v", err)
		}
	case <-time.After(waitSafety):
		t.Fatal("client did not cancel")
	}
	select {
	case <-settled:
	case <-time.After(waitSafety):
		t.Fatal("server kept working after disconnect")
	}
	after, err := os.ReadFile(gitLog)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("git reads after cancellation: before=%q after=%q", before, after)
	}
	if n := b.after.Load(); n != 0 {
		t.Errorf("%d counted reads started after cancellation", n)
	}
	if len(reg.Snapshot()) != 0 {
		t.Fatal("admission slot was not released")
	}
	resp, err := ts.Client().Do(request(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Error  any `json:"error"`
		Result any `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || out.Error != nil || out.Result == nil {
		t.Fatalf("next review failed: status %d, %+v", resp.StatusCode, out)
	}
	if got := b.services.Load(); got != 2 {
		t.Errorf("Services reads = %d, want one per review", got)
	}
	// The git log is a real witness: the uncancelled review's SARIF anchor
	// does run git after the hops.
	if next, _ := os.ReadFile(gitLog); len(next) == len(after) {
		t.Errorf("the next review made no git read after the hops: %q", next)
	}
}
