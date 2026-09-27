package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestPostCommitWaitRunsTheIngestTool (I38): the console's refresh remedy
// must re-index, not only queue. `post-commit-hook --wait` asks the
// daemon's ingest tool (an explicit request on the refresh queue, waited
// for up to --timeout), reports "refreshed" only when it returned, and
// reports the ingest's failure; it never uses the fire-and-forget route.
func TestPostCommitWaitRunsTheIngestTool(t *testing.T) {
	if !strings.Contains(daemonRefreshRemedy("/r", "svc"), "--wait") {
		t.Fatalf("remedy %q only queues; it must wait for the refresh", daemonRefreshRemedy("/r", "svc"))
	}
	var enqueues, toolCalls int32
	var fail atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/refresh-request", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&enqueues, 1)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"service":"svc","state":"queued"}`))
	})
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Params.Name == "ingest" {
			atomic.AddInt32(&toolCalls, 1)
			if fail.Load() {
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"ingest: database or disk is full"}}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	root := t.TempDir()

	var out, errBuf bytes.Buffer
	cmdPostCommitHook([]string{"--dir", root, "--url", srv.URL, "--service", "svc", "--wait", "--timeout", "5s"}, &out, &errBuf)
	if atomic.LoadInt32(&toolCalls) != 1 || atomic.LoadInt32(&enqueues) != 0 {
		t.Fatalf("--wait: ingest tool calls=%d, enqueues=%d; want 1 and 0", toolCalls, enqueues)
	}
	if !strings.Contains(out.String(), "refreshed \"svc\"") {
		t.Fatalf("--wait success line %q", out.String())
	}
	fail.Store(true)
	out.Reset()
	errBuf.Reset()
	cmdPostCommitHook([]string{"--dir", root, "--url", srv.URL, "--service", "svc", "--wait", "--timeout", "5s"}, &out, &errBuf)
	if !strings.Contains(errBuf.String(), "disk is full") || out.Len() != 0 {
		t.Fatalf("--wait failure: stdout %q stderr %q; want the ingest's error reported", out.String(), errBuf.String())
	}
}
