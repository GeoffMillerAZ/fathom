package mcp

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/refresh"
)

// TestEnsureNeverIngestsInline is RT-99 (design §7.1): a read of a drifted
// service asks the refresh queue for a re-ingest and answers at once from
// the index as it stands — the envelope's refresh block says queued or
// refreshing with index_stale, the indexed commit and HEAD — and the ingest
// runs on the queue's worker, not the read's goroutine. A console-origin
// read never asks.
func TestEnsureNeverIngestsInline(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 4)
	var running, maxSeen int64
	f := &gatedIngestFake{refreshFake: refreshFake{path: "/r", indexed: "0ld0000aaaa"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	ar := newAR(t, f, "4ead000bbbb")

	ctx, notes := withRefreshNotes(context.Background())
	returned := make(chan struct{})
	go func() {
		if _, err := ar.FindSymbol(ctx, "svc", "X"); err != nil {
			t.Error(err)
		}
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("the read waited on the ingest; it must answer from the index as it stands")
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the requested re-ingest never ran on the queue")
	}
	note := notes.render()["svc"]
	for _, want := range []string{"index_stale", "0ld0000", "4ead000"} {
		if !strings.Contains(note, want) {
			t.Fatalf("refresh note %q lacks %q", note, want)
		}
	}
	if !strings.HasPrefix(note, "queued") && !strings.HasPrefix(note, "refreshing") {
		t.Fatalf("refresh note %q: want queued or refreshing", note)
	}
	// A second read while it runs: answered at once, disclosed as refreshing.
	ctx2, notes2 := withRefreshNotes(context.Background())
	ar.ensure(ctx2, "svc")
	if got := notes2.render()["svc"]; !strings.HasPrefix(got, "refreshing") {
		t.Fatalf("read during the refresh: note %q, want refreshing", got)
	}
	close(release)
	settle(t, ar)
	if f.finished != 1 {
		t.Fatalf("ingests = %d, want 1", f.finished)
	}

	// Console origins never request one.
	for _, origin := range []string{OriginConsole, OriginConsoleInProcess} {
		cf := &refreshFake{path: "/r", indexed: "old"}
		car := newAR(t, cf, "new")
		car.ensure(WithOrigin(context.Background(), origin), "svc")
		if p := car.queue.Pending(); len(p) != 0 {
			t.Fatalf("%s read queued %v", origin, p)
		}
		if cf.ingests != 0 {
			t.Fatalf("%s read ingested", origin)
		}
	}

	// The public constructor runs on a queue too.
	pf := &gatedIngestFake{refreshFake: refreshFake{path: "/r", indexed: "old"}, started: make(chan string, 1), release: make(chan struct{}), running: new(int64), maxSeen: new(int64)}
	pub := NewAutoRefreshForTest(t, pf).(*autoRefreshBackend)
	pub.headFn = func(string) string { return "new" }
	done := make(chan struct{})
	go func() { pub.ensure(context.Background(), "svc"); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("NewAutoRefresh's ensure waited on the ingest")
	}
	<-pf.started
	if pub.queue.StateOf("svc") != refresh.StateRunning {
		t.Fatalf("state %q, want running", pub.queue.StateOf("svc"))
	}
	close(pf.release)
	settle(t, pub)
}

// TestIngestToolRunsOnRefreshQueue (RT-97, RT-98): with the daemon's queue
// installed, the `ingest` tool's ingest runs on it — a call made while the
// service's auto-refresh ingest runs waits for the one follow-up and says it
// was coalesced — never a second ingest beside the first.
func TestIngestToolRunsOnRefreshQueue(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 4)
	var running, maxSeen int64
	f := &gatedIngestFake{refreshFake: refreshFake{path: "/r", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	q := newTestRefreshQueue(t, f)
	ar := newARQueued(t, f, "new", q)
	srv := NewServer(ar)
	srv.SetRefreshQueue(q)

	ar.ensure(context.Background(), "svc") // auto-refresh: queued, starts
	<-started
	params := []byte(`{"name":"ingest","arguments":{"service":"svc"}}`)
	type res struct {
		v   any
		err *rpcError
	}
	done := make(chan res, 1)
	go func() {
		v, e := srv.handleToolsCallWithCtx(context.Background(), params)
		done <- res{v, e}
	}()
	time.Sleep(30 * time.Millisecond)
	if n := len(started); n != 0 {
		t.Fatalf("the ingest tool started a second ingest beside the running one")
	}
	close(release) // the running ingest and then its follow-up finish
	r := <-done
	if r.err != nil {
		t.Fatalf("ingest: %v", r.err.Message)
	}
	env, _ := r.v.(map[string]any)
	text := env["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"coalesced":true`) {
		t.Fatalf("ingest result %s; want coalesced:true", text)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.finished != 2 {
		t.Fatalf("ingests = %d, want 2 (the running one and one follow-up)", f.finished)
	}
	if atomic.LoadInt64(&maxSeen) != 1 {
		t.Fatal("two ingests ran at once")
	}
}

// TestServerAdoptsAutoRefreshQueue (RT-97, I50): a server built over the
// auto-refresh backend — serve-mcp's wiring, which never calls
// SetRefreshQueue — runs the ingest tool on that backend's queue, so an
// ingest-tool call beside a running auto-refresh ingest never starts a
// second ingest of the service.
func TestServerAdoptsAutoRefreshQueue(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 4)
	var running, maxSeen int64
	f := &gatedIngestFake{refreshFake: refreshFake{path: "/r", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	ar := NewAutoRefreshForTest(t, f).(*autoRefreshBackend)
	ar.headFn = func(string) string { return "new" }
	srv := NewServer(ar)
	if srv.RefreshQueue() != ar.RefreshQueue() {
		t.Fatal("the server did not adopt its auto-refresh backend's queue")
	}
	ar.ensure(context.Background(), "svc")
	<-started
	done := make(chan struct{})
	go func() {
		_, _ = srv.handleToolsCallWithCtx(context.Background(), []byte(`{"name":"ingest","arguments":{"service":"svc"}}`))
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	close(release)
	<-done
	settle(t, ar)
	if atomic.LoadInt64(&maxSeen) != 1 {
		t.Fatal("two ingests of svc ran at once (I50: at most one per service)")
	}
}

// probeFake is a non-git service's backend: its watermark is the content
// manifest (a whole-corpus read) and its probe is the stat-cached token.
type probeFake struct {
	refreshFake
	watermarks, probes int
}

func (f *probeFake) ServiceWatermark(context.Context, string) (string, error) {
	f.watermarks++
	return "manifest:new", nil
}

func (f *probeFake) ServiceFreshnessProbe(context.Context, string) (string, error) {
	f.probes++
	return "stat:moved", nil
}

// TestEnsureProbesNeverComputesManifest (RT-110, I48): a read of a non-git
// service asks the stat probe, never the content manifest, and a moved stat
// token queues a refresh.
func TestEnsureProbesNeverComputesManifest(t *testing.T) {
	f := &probeFake{refreshFake: refreshFake{path: "/docs", indexed: "manifest:old"}}
	ar := newAR(t, f, "")
	ar.ensure(context.Background(), "svc")
	settle(t, ar)
	if f.watermarks != 0 || f.probes != 1 {
		t.Fatalf("watermarks=%d probes=%d; want 0 and 1", f.watermarks, f.probes)
	}
	if f.ingests != 1 {
		t.Fatalf("ingests=%d; a moved stat token must queue one refresh", f.ingests)
	}
}
