package mcp

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// ingestText is the JSON text of an ingest tool result.
func ingestText(t *testing.T, v any) map[string]any {
	t.Helper()
	env, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("ingest answered %T, want the tool envelope", v)
	}
	text, _ := env["content"].([]any)[0].(map[string]any)["text"].(string)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("ingest answer %q is not JSON: %v", text, err)
	}
	return out
}

// TestIngestLongRunAnswersQueueState is EW-18 (O128): an ingest that outlasts
// the call's wait budget — a cold typed pass took 3m30s against a 150 s write
// deadline, and /rpc answered with an empty body — answers with the refresh
// queue's state instead, while the run goes on to completion under the
// queue's own context.
func TestIngestLongRunAnswersQueueState(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 4)
	var running, maxSeen int64
	f := &gatedIngestFake{refreshFake: refreshFake{path: "/r", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	q := newTestRefreshQueue(t, f)
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) }) // runs before the queue's own cleanup waits on the run
	srv := NewServer(f)
	srv.SetRefreshQueue(q)
	srv.SetIngestWaitBudget(20 * time.Millisecond)

	v, rerr := srv.handleToolsCallWithCtx(context.Background(), []byte(`{"name":"ingest","arguments":{"service":"svc"}}`))
	if rerr != nil {
		t.Fatalf("a long ingest answered an error, want the queue's state: %v", rerr.Message)
	}
	got := ingestText(t, v)
	if got["service"] != "svc" || got["completed"] != false {
		t.Fatalf("answer %v, want service svc and completed:false", got)
	}
	if got["ok"] == true {
		t.Fatalf("answer %v claims ok:true for an ingest that has not finished", got)
	}
	if st, _ := got["state"].(string); st != string(refresh.StateQueued) && st != string(refresh.StateRunning) {
		t.Fatalf("state %q, want the queue's queued or running", st)
	}
	if note, _ := got["note"].(string); note == "" {
		t.Fatalf("answer %v names no way to follow the run", got)
	}

	// The run was not cancelled by the answer: it starts, runs under the
	// queue's context and finishes when released.
	<-started
	if s := q.StateOf("svc"); s != refresh.StateRunning {
		t.Fatalf("state after the answer %q, want running", s)
	}
	once.Do(func() { close(release) })
	deadline := time.Now().Add(30 * time.Second) // a safety bound, never asserted on
	for q.StateOf("svc") != refresh.StateIdle {
		if time.Now().After(deadline) {
			t.Fatalf("the ingest did not finish after its caller was answered: %v", q.Pending())
		}
		time.Sleep(time.Millisecond)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.finished != 1 || len(f.ctxErrs) != 1 || f.ctxErrs[0] != nil {
		t.Fatalf("finished=%d ctxErrs=%v, want one run whose context was never cancelled", f.finished, f.ctxErrs)
	}
}

// TestIngestAnswersNormallyWithinTheBudget: an ingest that finishes inside
// the wait budget answers as before (ok:true, no completed:false).
func TestIngestAnswersNormallyWithinTheBudget(t *testing.T) {
	f := &refreshFake{path: "/r", indexed: "old"}
	q := newTestRefreshQueue(t, f)
	srv := NewServer(f)
	srv.SetRefreshQueue(q)
	srv.SetIngestWaitBudget(30 * time.Second) // a safety bound: the fake ingest is instant
	v, rerr := srv.handleToolsCallWithCtx(context.Background(), []byte(`{"name":"ingest","arguments":{"service":"svc"}}`))
	if rerr != nil {
		t.Fatalf("ingest: %v", rerr.Message)
	}
	got := ingestText(t, v)
	if got["ok"] != true {
		t.Fatalf("answer %v, want ok:true", got)
	}
	if _, has := got["completed"]; has {
		t.Fatalf("answer %v carries completed on a finished ingest", got)
	}
}

// TestIngestWaitBudgetDefault: a server never told a budget waits the
// default, which stays under the daemon's per-route write deadline.
func TestIngestWaitBudgetDefault(t *testing.T) {
	srv := NewServer(&refreshFake{})
	if got := srv.ingestWaitBudget(); got != DefaultIngestWaitBudget {
		t.Fatalf("default wait budget %v, want %v", got, DefaultIngestWaitBudget)
	}
	srv.SetIngestWaitBudget(time.Second)
	if got := srv.ingestWaitBudget(); got != time.Second {
		t.Fatalf("wait budget %v after SetIngestWaitBudget(1s)", got)
	}
	srv.SetIngestWaitBudget(0)
	if got := srv.ingestWaitBudget(); got != DefaultIngestWaitBudget {
		t.Fatalf("wait budget %v after SetIngestWaitBudget(0), want the default", got)
	}
}

// TestIngestAnsweredEarlyStillLiftsQuarantine: RT-155 says the explicit
// ingest's completion lifts a quarantine. When the call answered from the
// queue's state, the run's completion still lifts it.
func TestIngestAnsweredEarlyStillLiftsQuarantine(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 4)
	var running, maxSeen int64
	f := &gatedIngestFake{refreshFake: refreshFake{path: "/r", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	q := newTestRefreshQueue(t, f)
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	srv := NewServer(f)
	srv.SetRefreshQueue(q)
	srv.SetIngestWaitBudget(20 * time.Millisecond)
	tenancy.AddQuarantined(tenancy.Quarantine{Service: "svc", Since: time.Now(), Reason: "test"})
	t.Cleanup(func() { tenancy.SetQuarantined(nil) })

	if _, rerr := srv.handleToolsCallWithCtx(context.Background(), []byte(`{"name":"ingest","arguments":{"service":"svc"}}`)); rerr != nil {
		t.Fatalf("ingest: %v", rerr.Message)
	}
	<-started
	if !tenancy.IsQuarantined("svc") {
		t.Fatal("the quarantine lifted before the ingest finished")
	}
	once.Do(func() { close(release) })
	deadline := time.Now().Add(30 * time.Second)
	for tenancy.IsQuarantined("svc") {
		if time.Now().After(deadline) {
			t.Fatal("the finished ingest never lifted the quarantine of a call that answered early")
		}
		time.Sleep(time.Millisecond)
	}
}
