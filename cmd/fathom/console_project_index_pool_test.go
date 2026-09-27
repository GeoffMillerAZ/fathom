package main

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// TestProjectIndexRunsInTheJobPool (RT-117, design §14: "console.Jobs …
// serves plans, conserve and the project index"): a project's index read
// runs as a job in the console's bounded pool — counted by its stats, one
// flight per key — never on the request's own goroutine outside it, and a
// full pool is refused by name and not kept.
func TestProjectIndexRunsInTheJobPool(t *testing.T) {
	f := newConsoleFixture(t)
	projectIndexMemo.reset()
	t.Cleanup(projectIndexMemo.reset)
	hold := make(chan struct{})
	prev := projectIndexCompute
	projectIndexCompute = func(ctx context.Context, d *consoleDeps, pc projectContext) projectIndexData {
		<-hold
		return projectIndexData{facts: console.IndexFacts{Service: pc.svc}}
	}
	t.Cleanup(func() { projectIndexCompute = prev })
	deps := newConsoleDeps(f.client, "http://x", f.now)
	pc := projectContext{svc: "proof-service"}
	got := make(chan projectIndexData, 2)
	for i := 0; i < 2; i++ {
		go func() { got <- projectIndexFor(context.Background(), deps, pc) }()
	}
	waitUntil(t, func() bool { return deps.jobPool().Stats().Running == 1 })
	if st := deps.jobPool().Stats(); st.Running+st.Queued != 1 {
		t.Fatalf("pool = %+v; want the one index read, however many requests", st)
	}
	close(hold)
	for i := 0; i < 2; i++ {
		select {
		case d := <-got:
			if d.facts.Service != "proof-service" {
				t.Errorf("index = %+v", d.facts)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the index read never finished")
		}
	}

	// A full pool: refused by name, never kept.
	full := &consoleDeps{c: f.client, selfURL: "http://x", jobs: console.NewJobs(1, 1)}
	block := make(chan struct{})
	defer close(block)
	if _, err := full.jobPool().SubmitJob("", "busy", func() { <-block }); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return full.jobPool().Stats().Running == 1 })
	if _, err := full.jobPool().SubmitJob("", "waiting", func() {}); err != nil {
		t.Fatal(err)
	}
	projectIndexMemo.reset()
	d := projectIndexFor(context.Background(), full, projectContext{svc: "other"})
	if d.facts.InventoryErr == "" {
		t.Fatalf("a full pool = %+v; want the refusal named", d.facts)
	}
}
