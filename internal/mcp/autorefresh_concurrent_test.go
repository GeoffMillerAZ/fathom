package mcp

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestEnsureDefersDriftBesideLiveForeignIngest (GDEV-02): HEAD drifted, but
// another live process holds an in_progress row for the service (a CLI
// ingest into the daemon's store). ensure queues nothing beside it, and
// the read's envelope says the refresh was deferred and why; once the
// owner is gone, the same drift is queued.
func TestEnsureDefersDriftBesideLiveForeignIngest(t *testing.T) {
	started := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	foreign := os.Getpid() + 1
	fake := &ingestStateFake{refreshFake: refreshFake{path: "/repo", indexed: "old"},
		row: store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: foreign, StartedAt: started.UnixNano(), Path: "/repo"}}
	ar := newAR(t, fake, "new")
	ar.ownerAlive = ownerProbe(true)

	ctx, notes := withRefreshNotes(context.Background())
	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 0 || len(ar.queue.Pending()) != 0 || ar.AutoRefreshTotal() != 0 {
		t.Fatalf("a drift beside a live foreign ingest queued work: ingests %d, pending %v, total %d", fake.ingests, ar.queue.Pending(), ar.AutoRefreshTotal())
	}
	ar.mu.Lock()
	pending := len(ar.pending)
	ar.mu.Unlock()
	if pending != 0 {
		t.Fatalf("pending = %d, want none", pending)
	}
	want := "deferred (another process is ingesting svc: pid " + itoa(foreign) + " since 2026-10-26T12:00:00Z) — served from the index as it stands"
	if got := notes.render()["svc"]; got != want {
		t.Fatalf("refresh note = %q, want %q", got, want)
	}

	ar.ownerAlive = ownerProbe(false)
	ensureSettled(t, ar, context.Background(), "svc")
	if fake.ingests != 1 {
		t.Fatalf("once the owner is gone the drift must be queued: ingests = %d, want 1", fake.ingests)
	}
}
