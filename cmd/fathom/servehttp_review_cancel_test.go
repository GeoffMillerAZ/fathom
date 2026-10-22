//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// The source interfaces count any reads attempted after cancellation.
// A generated git wrapper pauses inside the baseline/conservation git reads.
type cancelledReviewReader struct {
	*client.Client
	after atomic.Int64
}

func (b *cancelledReviewReader) ReadSourceFiles(ctx context.Context, service string, paths []string) ([]core.SourceFile, error) {
	if ctx.Err() != nil {
		b.after.Add(1)
	}
	files, err := b.Client.ReadSourceFiles(ctx, service, paths)
	return files, err
}

func (b *cancelledReviewReader) ScanSources(ctx context.Context, service string, _ corpus.Budget, visit func(core.SourceFile) bool) (corpus.Coverage, error) {
	if ctx.Err() != nil {
		b.after.Add(1)
	}
	paths, err := b.Client.SourcePaths(ctx, service)
	if err != nil {
		return corpus.Coverage{}, err
	}
	for _, path := range paths {
		files, err := b.ReadSourceFiles(ctx, service, []string{path})
		if err != nil {
			return corpus.Coverage{}, err
		}
		for _, f := range files {
			if visit(f) {
				return corpus.Coverage{}, nil
			}
		}
	}
	return corpus.Coverage{}, nil
}

func TestReviewDiffCancelledBaselineStopsWorkAndReleasesAdmission(t *testing.T) {
	for _, phase := range []string{"git", "conservation"} {
		t.Run(phase, func(t *testing.T) { testReviewCancelledBaseline(t, phase) })
	}
}

func testReviewCancelledBaseline(t *testing.T, phase string) {
	t.Helper()
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
	var changed []string
	for i := range 64 {
		path := fmt.Sprintf("source%03d.go", i)
		changed = append(changed, path)
		if err := os.WriteFile(filepath.Join(root, path), []byte(fmt.Sprintf("package sample\nfunc Value%d() int { return 1 }\n", i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-qm", "base")
	base := strings.TrimSpace(git("rev-parse", "HEAD"))
	for i, path := range changed {
		if err := os.WriteFile(filepath.Join(root, path), []byte(fmt.Sprintf("package sample\nfunc Value%d() int { return 2 }\n", i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "review.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	if err := c.Store().SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "sample", Path: root, IndexedCommit: base}); err != nil {
		t.Fatal(err)
	}
	started, gitLog := pauseReviewGit(t, phase)
	b := &cancelledReviewReader{Client: c}
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := func(ctx context.Context) *http.Request {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/rpc", rpcBody(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "sample", "changed_files": changed, "since": base}}}))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		return req
	}
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
	case <-started:
	case <-time.After(waitSafety):
		t.Fatal("review never reached the selected git read")
	}
	if len(reg.Snapshot()) != 1 {
		t.Fatal("review was not admitted")
	}
	before, err := os.ReadFile(gitLog)
	if err != nil {
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
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("git reads after cancellation: before=%s after=%s", before, after)
	}
	if n := b.after.Load(); n != 0 {
		t.Errorf("%d source reads started after cancellation", n)
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
}

// pauseReviewGit blocks once in a shell builtin, so CommandContext kills the
// actual paused git process. Success uses FIFO handshakes, never a sleep.
func pauseReviewGit(t *testing.T, phase string) (<-chan struct{}, string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	start, hold := filepath.Join(dir, "start"), filepath.Join(dir, "hold")
	for _, p := range []string{start, hold} {
		if err := syscall.Mkfifo(p, 0600); err != nil {
			t.Fatal(err)
		}
	}
	signal, err := os.OpenFile(start, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { signal.Close() })
	started := make(chan struct{})
	go func() {
		var b [1]byte
		if _, err := signal.Read(b[:]); err == nil {
			close(started)
		}
	}()
	pattern := "*'show '*"
	if phase == "conservation" {
		pattern = "*'diff '*--name-only*"
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	log := filepath.Join(dir, "commands")
	script := "#!/bin/sh\n" + "printf '%s\\n' \"$*\" >> " + quote(log) + "\n" +
		"case \"$*\" in " + pattern + ")\n" +
		"if /bin/mkdir " + quote(filepath.Join(dir, "once")) + " 2>/dev/null; then\n" +
		"printf x > " + quote(start) + "\nread token < " + quote(hold) + "\nfi\n;;\nesac\n" +
		"exec " + quote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return started, log
}
