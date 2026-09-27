package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestPostCommitEnqueuesRefresh is RT-111 (design §12.3): the post-commit
// hook asks the daemon's POST /refresh-request, which queues the refresh
// and answers 202 at once — the ingest runs on the refresh queue, never in
// the hook's request — and a second commit while it runs coalesces into
// the one follow-up. The `ingest` tool is never called; a refusal is
// reported as a failure with its reason, and a service the daemon does not
// index is a 404 naming why.
func TestPostCommitEnqueuesRefresh(t *testing.T) {
	ms := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), ms)
	t.Cleanup(func() { _ = c.Close() })
	root := t.TempDir()
	if err := ms.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc", Path: root, IndexedCommit: "abc"}); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	var runs int32
	started := make(chan struct{}, 4)
	q := refresh.New(refresh.Config{
		Run: func(ctx context.Context, job refresh.Job) error {
			atomic.AddInt32(&runs, 1)
			started <- struct{}{}
			<-release
			return nil
		},
		Gate: func(service string, explicit bool, now time.Time) refresh.Verdict {
			if service == "quarantined" {
				return refresh.Verdict{Refuse: "quarantined: exceeded budget during ingest"}
			}
			return refresh.Verdict{}
		},
		StartWait: -1, Stagger: -1, Warmup: func(time.Time) bool { return false },
	})
	qctx, stop := context.WithCancel(context.Background())
	q.Start(qctx)
	t.Cleanup(func() { close(release); stop(); q.Wait() })

	var toolCalls int32
	mux := http.NewServeMux()
	mux.Handle("/refresh-request", refreshRequestHandler(c, func() *refresh.Queue { return q }))
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name == "ingest" {
			atomic.AddInt32(&toolCalls, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var out, errBuf bytes.Buffer
	begin := time.Now()
	if code := cmdPostCommitHook([]string{"--dir", root, "--url", srv.URL, "--service", "svc", "--timeout", "5s"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	if d := time.Since(begin); d > 3*time.Second {
		t.Fatalf("the hook took %s — it waited on the ingest", d)
	}
	if !strings.Contains(out.String(), "queued a refresh of \"svc\" through the daemon") {
		t.Fatalf("success line %q; want it to say the refresh was queued", out.String())
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the queued refresh never ran")
	}
	// A second commit while the ingest runs: 202, coalesced into the one
	// follow-up — never a second parallel ingest.
	out.Reset()
	if code := cmdPostCommitHook([]string{"--dir", root, "--url", srv.URL, "--service", "svc", "--timeout", "5s"}, &out, &errBuf); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "follow-up") {
		t.Fatalf("second commit mid-ingest: %q; want the follow-up named", out.String())
	}
	if n := atomic.LoadInt32(&runs); n != 1 {
		t.Fatalf("%d ingests ran while one was in flight; want 1", n)
	}
	if n := atomic.LoadInt32(&toolCalls); n != 0 {
		t.Fatalf("the hook called the ingest tool %d time(s); it must only enqueue", n)
	}

	// The raw route: 202 with the queue's answer.
	resp, err := http.Post(srv.URL+"/refresh-request", "application/json", strings.NewReader(`{"service":"svc"}`))
	if err != nil {
		t.Fatal(err)
	}
	var ans refreshRequestResponse
	_ = json.NewDecoder(resp.Body).Decode(&ans)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted || !ans.Coalesced {
		t.Fatalf("POST /refresh-request mid-ingest: %d %+v; want 202 coalesced", resp.StatusCode, ans)
	}

	// Refused (a quarantined service) and not indexed: failures naming why.
	if err := ms.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "quarantined", Path: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := ingestViaDaemon(srv.URL, "quarantined", root, 5*time.Second); err == nil || !strings.Contains(err.Error(), "quarantined") {
		t.Fatalf("refused refresh: %v; want the refusal named", err)
	}
	if _, err := ingestViaDaemon(srv.URL, "nosuch", root, 5*time.Second); err == nil || !strings.Contains(err.Error(), "not indexed") {
		t.Fatalf("unindexed service: %v; want not indexed", err)
	}
	if n := atomic.LoadInt32(&toolCalls); n != 0 {
		t.Fatalf("a refusal fell back to the ingest tool (%d calls)", n)
	}
}

// TestRefreshFileDefersToQueuedServiceRefresh (RT-97): while a whole-service
// refresh of the service is queued or running on the refresh queue,
// /refresh-file leaves the file to it — no per-file write races the ingest
// — and says so.
func TestRefreshFileDefersToQueuedServiceRefresh(t *testing.T) {
	ms := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), ms)
	t.Cleanup(func() { _ = c.Close() })
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	q := refresh.New(refresh.Config{
		Run:       func(ctx context.Context, job refresh.Job) error { started <- struct{}{}; <-release; return nil },
		Gate:      func(string, bool, time.Time) refresh.Verdict { return refresh.Verdict{} },
		StartWait: -1, Stagger: -1, Warmup: func(time.Time) bool { return false },
	})
	qctx, stop := context.WithCancel(context.Background())
	q.Start(qctx)
	t.Cleanup(func() { close(release); stop(); q.Wait() })
	q.RequestRefresh(context.Background(), refresh.Request{Service: "svc", Reason: refresh.ReasonAuto}, false)
	<-started

	srv := httptest.NewServer(refreshFileHandler(c, nil, func() *refresh.Queue { return q }))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/refresh-file", "application/json", strings.NewReader(`{"service":"svc","file":"a.go"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out refreshFileResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Refreshed || !strings.Contains(out.Skipped, "whole-service refresh running") {
		t.Fatalf("/refresh-file during a service refresh: %+v; want skipped, naming the running refresh", out)
	}
}
