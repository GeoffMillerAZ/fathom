package mcp

import (
	"context"
	"testing"
)

// TestConsoleOriginNeverTriggersAutoRefresh (RT-61, design §3.3): a
// console-origin read — over HTTP ("console") or in process
// ("console-inprocess") — answers from the index as it stands and never
// starts an inline ingest; an agent read of the same drifted service does.
func TestConsoleOriginNeverTriggersAutoRefresh(t *testing.T) {
	for _, origin := range []string{OriginConsole, OriginConsoleInProcess} {
		f := &refreshFake{path: "/r", indexed: "old"}
		ar := newAR(f, "new")
		ar.ensure(WithOrigin(context.Background(), origin), "svc")
		if f.ingests != 0 {
			t.Fatalf("%s read triggered %d inline ingests", origin, f.ingests)
		}
	}
	f := &refreshFake{path: "/r", indexed: "old"}
	ar := newAR(f, "new")
	ar.ensure(context.Background(), "svc")
	if f.ingests != 1 {
		t.Fatalf("agent read of a drifted service: %d ingests, want 1", f.ingests)
	}
}
