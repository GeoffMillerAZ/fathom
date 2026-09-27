package mcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestEnsureBackoffIsDisclosed is RT-99 under I38/I4: a read inside the
// failure backoff of a drifted service is served from the stale index like
// any other drifted read, so its envelope says so — the index is stale, the
// last refresh failed, and when it is retried — instead of answering
// silently as if fresh.
func TestEnsureBackoffIsDisclosed(t *testing.T) {
	fake := &failingIngestFake{refreshFake: refreshFake{path: "/repo", indexed: "0ld0000aaaa"}, failFirst: 1 << 30}
	ar := newAR(t, fake, "4ead000bbbb")
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	ar.now = func() time.Time { return now }
	ensureSettled(t, ar, context.Background(), "svc")
	if fake.ingests != 1 {
		t.Fatalf("setup: %d ingest attempts, want 1", fake.ingests)
	}

	now = now.Add(10 * time.Second)
	ctx, notes := withRefreshNotes(context.Background())
	ar.ensure(ctx, "svc")
	got := notes.render()["svc"]
	for _, want := range []string{"index_stale", "failed", "retried after", "0ld0000"} {
		if !strings.Contains(got, want) {
			t.Fatalf("read inside the backoff: refresh note %q lacks %q", got, want)
		}
	}
	if fake.ingests != 1 {
		t.Fatalf("the disclosure must not retry inside the backoff: %d attempts", fake.ingests)
	}
}
