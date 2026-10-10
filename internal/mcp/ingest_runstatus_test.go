package mcp

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestIngestRunAnswersCompletion is EW-66 (G4): an ingest answered from the
// queue's state (completed:false after the wait budget) names its run, and
// the ingest tool called with that run answers whether it finished, from
// the queue's state and the service's last finished run, without queuing
// another ingest. Before, the only follow-up was a poll hint at
// list_services or staleness_report, neither of which says the run is done.
func TestIngestRunAnswersCompletion(t *testing.T) {
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
	call := func(args string) map[string]any {
		t.Helper()
		v, rerr := srv.handleToolsCallWithCtx(context.Background(), []byte(`{"name":"ingest","arguments":`+args+`}`))
		if rerr != nil {
			t.Fatalf("ingest %s: %v", args, rerr.Message)
		}
		return ingestText(t, v)
	}

	got := call(`{"service":"svc"}`)
	run, _ := got["run"].(string)
	if got["completed"] != false || run == "" {
		t.Fatalf("queue-state answer %v, want completed:false and the run that serves it", got)
	}
	if note, _ := got["note"].(string); !strings.Contains(note, "run") {
		t.Errorf("note %q names no way to ask about the run", note)
	}
	<-started

	st := call(`{"service":"svc","run":"` + run + `"}`)
	if st["completed"] != false || st["run"] != run || st["state"] != "running" {
		t.Fatalf("run status mid-run = %v, want completed:false, state running", st)
	}
	if _, has := st["ok"]; has {
		t.Errorf("run status mid-run carries ok: %v", st)
	}

	once.Do(func() { close(release) })
	deadline := time.Now().Add(waitSafetyMCP)
	for {
		st = call(`{"service":"svc","run":"` + run + `"}`)
		if st["completed"] == true {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run status never read completed: %v", st)
		}
		time.Sleep(2 * time.Millisecond)
	}
	if st["ok"] != true || st["state"] != "done" {
		t.Errorf("finished run status = %v, want ok:true, state done", st)
	}
	// Asking queued nothing: the one run is the only ingest.
	f.mu.Lock()
	n := f.finished
	f.mu.Unlock()
	if n != 1 || q.StateOf("svc") != "idle" {
		t.Errorf("finished=%d state=%s after status calls, want one ingest and an idle queue", n, q.StateOf("svc"))
	}

	unknown := call(`{"service":"svc","run":"nonsense-1"}`)
	if unknown["completed"] != false || unknown["state"] != "unknown" {
		t.Errorf("unknown run answered %v, want completed:false, state unknown", unknown)
	}
	if _, rerr := srv.handleToolsCallWithCtx(context.Background(), []byte(`{"name":"ingest","arguments":{"service":"svc","run":"x-1","path":"/elsewhere"}}`)); rerr == nil {
		t.Error("run with path was accepted: a status call never onboards")
	}
}

// waitSafetyMCP bounds a poll, never asserted on as a duration.
const waitSafetyMCP = 30 * time.Second
