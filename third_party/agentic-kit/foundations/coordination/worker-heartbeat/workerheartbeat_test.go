package workerheartbeat

import (
	"testing"
	"time"
)

var epoch = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

func TestRecord_Basic(t *testing.T) {
	m := NewMonitor()
	hb := Heartbeat{WorkerID: "w1", At: epoch}
	if err := m.Record(hb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := m.Latest("w1")
	if !ok {
		t.Fatal("expected worker to be tracked after Record")
	}
	if got.At != epoch {
		t.Fatalf("expected At=%v, got %v", epoch, got.At)
	}
}

func TestRecord_EmptyWorkerID(t *testing.T) {
	m := NewMonitor()
	err := m.Record(Heartbeat{WorkerID: "", At: epoch})
	if err != ErrEmptyWorkerID {
		t.Fatalf("expected ErrEmptyWorkerID, got %v", err)
	}
}

func TestRecord_Monotonic(t *testing.T) {
	// Recording an older heartbeat should not overwrite a newer one.
	m := NewMonitor()
	newer := Heartbeat{WorkerID: "w1", At: epoch.Add(time.Minute)}
	older := Heartbeat{WorkerID: "w1", At: epoch}
	must(t, m.Record(newer))
	must(t, m.Record(older))
	got, _ := m.Latest("w1")
	if !got.At.Equal(newer.At) {
		t.Fatalf("expected newer heartbeat to be kept, got %v", got.At)
	}
}

func TestStale_NoWorkers(t *testing.T) {
	m := NewMonitor()
	stale := m.Stale(epoch, time.Minute)
	if len(stale) != 0 {
		t.Fatalf("expected empty slice, got %v", stale)
	}
}

func TestStale_AllFresh(t *testing.T) {
	m := NewMonitor()
	now := epoch.Add(10 * time.Second)
	must(t, m.Record(Heartbeat{WorkerID: "w1", At: now.Add(-5 * time.Second)}))
	must(t, m.Record(Heartbeat{WorkerID: "w2", At: now.Add(-3 * time.Second)}))
	stale := m.Stale(now, 30*time.Second)
	if len(stale) != 0 {
		t.Fatalf("expected no stale workers, got %v", stale)
	}
}

func TestStale_SomeStale(t *testing.T) {
	m := NewMonitor()
	now := epoch.Add(2 * time.Minute)
	ttl := time.Minute
	// w1: last seen 90s ago → stale (>60s)
	must(t, m.Record(Heartbeat{WorkerID: "w1", At: now.Add(-90 * time.Second)}))
	// w2: last seen 30s ago → fresh
	must(t, m.Record(Heartbeat{WorkerID: "w2", At: now.Add(-30 * time.Second)}))
	// w3: last seen 61s ago → stale
	must(t, m.Record(Heartbeat{WorkerID: "w3", At: now.Add(-61 * time.Second)}))

	stale := m.Stale(now, ttl)
	if len(stale) != 2 {
		t.Fatalf("expected 2 stale workers, got %d: %v", len(stale), stale)
	}
	// Verify deterministic order
	if stale[0].WorkerID != "w1" || stale[1].WorkerID != "w3" {
		t.Errorf("expected [w1,w3], got [%v,%v]", stale[0].WorkerID, stale[1].WorkerID)
	}
}

func TestStale_ExactlyAtBoundary(t *testing.T) {
	// A heartbeat at exactly now-ttl is NOT stale (requires strictly before cutoff).
	m := NewMonitor()
	now := epoch.Add(time.Minute)
	ttl := time.Minute
	cutoff := now.Add(-ttl)
	must(t, m.Record(Heartbeat{WorkerID: "w1", At: cutoff}))
	stale := m.Stale(now, ttl)
	// At exactly cutoff is not Before(cutoff), so it should NOT be stale.
	if len(stale) != 0 {
		t.Fatalf("expected 0 stale at boundary, got %v", stale)
	}
}

func TestWorkers_SortedDeterminism(t *testing.T) {
	m := NewMonitor()
	for _, id := range []string{"z", "a", "m", "b"} {
		must(t, m.Record(Heartbeat{WorkerID: id, At: epoch}))
	}
	workers := m.Workers()
	expected := []string{"a", "b", "m", "z"}
	for i, w := range workers {
		if w != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], w)
		}
	}
}

func TestStale_Determinism(t *testing.T) {
	// Run Stale twice with the same monitor state, expect identical output.
	m := NewMonitor()
	now := epoch.Add(5 * time.Minute)
	for _, id := range []string{"w3", "w1", "w2"} {
		must(t, m.Record(Heartbeat{WorkerID: id, At: epoch}))
	}
	s1 := m.Stale(now, time.Minute)
	s2 := m.Stale(now, time.Minute)
	if len(s1) != len(s2) {
		t.Fatalf("non-deterministic: first call %d, second call %d", len(s1), len(s2))
	}
	for i := range s1 {
		if s1[i].WorkerID != s2[i].WorkerID {
			t.Errorf("position %d: %q != %q", i, s1[i].WorkerID, s2[i].WorkerID)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
