package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestObservatoryModelServesStaleWhileRebuilding (RT-113, design §14): a
// request past the model's TTL gets the last model at once while ONE flight
// rebuilds it outside the cache's lock — the proof measured 3.6–4.4 s
// stalls when every request waited on a rebuild under the lock — a cold
// cache waits for that one flight, and the rebuild reads the call log and
// the session ledger from tails: only the bytes appended since the last
// build are parsed, and what they return equals a whole-file read.
func TestObservatoryModelServesStaleWhileRebuilding(t *testing.T) {
	resetObsCache()
	t.Cleanup(resetObsCache)
	release := make(chan struct{})
	var builds atomic.Int32
	prev := obsModelBuilder
	obsModelBuilder = func(baseURL string, now time.Time) observatoryModel {
		n := builds.Add(1)
		if n > 1 {
			<-release
		}
		return observatoryModel{GeneratedAt: now.UTC().Format(time.RFC3339)}
	}
	t.Cleanup(func() { obsModelBuilder = prev })
	t0 := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	// Cold: the caller waits for the one build.
	if m := cachedObservatoryModel("http://x", t0); m.GeneratedAt != "2026-09-27T12:00:00Z" || builds.Load() != 1 {
		t.Fatalf("cold = %q after %d builds", m.GeneratedAt, builds.Load())
	}
	// Past the TTL: every caller gets the last model at once; one flight.
	later := t0.Add(obsCacheTTL + time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			m := cachedObservatoryModel("http://x", later)
			if m.GeneratedAt != "2026-09-27T12:00:00Z" {
				t.Errorf("a stale read got %q", m.GeneratedAt)
			}
			if d := time.Since(start); d > time.Second {
				t.Errorf("a stale read waited %s for the rebuild", d)
			}
		}()
	}
	wg.Wait()
	waitUntil(t, func() bool { return builds.Load() == 2 })
	// The rebuild runs outside the lock: the cache stays usable meanwhile
	// (the governor's drop takes the same lock).
	dropped := make(chan struct{})
	go func() { consoleMemoDrops(); close(dropped) }()
	select {
	case <-dropped:
	case <-time.After(2 * time.Second):
		t.Fatal("the cache's lock was held while the model rebuilt")
	}
	close(release)
	waitUntil(t, func() bool {
		obsCache.Lock()
		defer obsCache.Unlock()
		return !obsCache.building
	})
	if n := builds.Load(); n != 2 {
		t.Errorf("%d builds; want one flight for eight stale reads", n)
	}

	// The tails: parse only what was appended, and agree with whole reads.
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.jsonl")
	line := `{"ts":"2026-09-27T10:00:00Z","tool":"find_symbol","service":"pay","latency_ms":2}` + "\n"
	mustWrite(t, log, line+"not json\n"+`{"ts":"2026-09-27T10:00:01Z","tool":"list_services","origin":"console"}`+"\n")
	var tail callLogEntryTail
	check := func(label string) {
		t.Helper()
		snap, err := tail.read(log)
		want, ws, wc, werr := callLogReadCounting(log)
		if err != nil || werr != nil || snap.fold.rows != len(want) || snap.skipped != ws || snap.consoleReads != wc {
			t.Fatalf("%s: tail %+v %v, whole %d/%d/%d %v", label, snap, err, len(want), ws, wc, werr)
		}
	}
	check("first read")
	before := tail.parsed
	fh, _ := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o644)
	fh.WriteString(line)
	fh.Close()
	check("after an append")
	if got := tail.parsed - before; got != int64(len(line)) {
		t.Errorf("the append parsed %d bytes, want %d (only what was appended)", got, len(line))
	}
	var ledger bytesTail
	lp := filepath.Join(dir, "sessions.jsonl")
	mustWrite(t, lp, "{\"session_id\":\"a\"}\n")
	ledger.read(lp)
	fh, _ = os.OpenFile(lp, os.O_APPEND|os.O_WRONLY, 0o644)
	fh.WriteString("{\"session_id\":\"b\"}\n")
	fh.Close()
	got, err := ledger.read(lp)
	whole, _ := os.ReadFile(lp)
	if err != nil || string(got) != string(whole) {
		t.Errorf("ledger tail = %q, %v; want %q", got, err, whole)
	}
	if _, err := ledger.read(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Errorf("a missing ledger = %v, want not-exist", err)
	}
}

// TestObservatoryModelNeverServesAnEmptyModel (RT-113): a cold reader never
// gets the zero model rendered as zero services and zero traffic. A drop
// during a rebuild (the governor's consoleMemoDrops, under memory
// pressure) makes the waiter start a new flight; a build that panics
// answers a model that names the failure — measured nothing, never zero.
func TestObservatoryModelNeverServesAnEmptyModel(t *testing.T) {
	resetObsCache()
	t.Cleanup(resetObsCache)
	release := make(chan struct{})
	var builds atomic.Int32
	var panics atomic.Bool
	prev := obsModelBuilder
	obsModelBuilder = func(baseURL string, now time.Time) observatoryModel {
		if n := builds.Add(1); n == 2 {
			<-release
		}
		if panics.Load() {
			panic("the fold broke")
		}
		return observatoryModel{GeneratedAt: now.UTC().Format(time.RFC3339)}
	}
	t.Cleanup(func() { obsModelBuilder = prev })
	t0 := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	cachedObservatoryModel("http://x", t0) // warm
	later := t0.Add(obsCacheTTL + time.Second)
	cachedObservatoryModel("http://x", later) // stale; flight 2 blocks
	waitUntil(t, func() bool { return builds.Load() == 2 })
	consoleMemoDrops()
	got := make(chan observatoryModel, 1)
	go func() { got <- cachedObservatoryModel("http://x", later) }()
	time.Sleep(50 * time.Millisecond)
	close(release)
	select {
	case m := <-got:
		if m.GeneratedAt == "" {
			t.Fatalf("a cold read after a drop got the empty model: %+v", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cold read never returned")
	}

	// A panicking build: the cold reader gets a model naming the failure.
	resetObsCache()
	panics.Store(true)
	m := cachedObservatoryModel("http://x", t0)
	if m.GeneratedAt == "" || !strings.Contains(m.DegradedNote, "could not be built") {
		t.Fatalf("a failed build = %+v; want a model naming the failure", m)
	}
	if m.callFacts.AbsentReason == "" {
		t.Errorf("a failed build's call facts claim a read: %+v", m.callFacts)
	}
}

// TestColdModelWaitHonoursTheRequest (RT-113, RT-62): a cold reader whose
// request ends while the one flight builds leaves with a model naming why,
// never holding its handler goroutine for the build.
func TestColdModelWaitHonoursTheRequest(t *testing.T) {
	resetObsCache()
	t.Cleanup(resetObsCache)
	release := make(chan struct{})
	prev := obsModelBuilder
	obsModelBuilder = func(baseURL string, now time.Time) observatoryModel {
		<-release
		return observatoryModel{GeneratedAt: now.UTC().Format(time.RFC3339)}
	}
	t.Cleanup(func() { obsModelBuilder = prev })
	// The flight finishes (and stores) before the cleanups reset the cache.
	defer waitUntil(t, func() bool {
		obsCache.Lock()
		defer obsCache.Unlock()
		return !obsCache.building
	})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	got := make(chan observatoryModel, 1)
	go func() { got <- cachedObservatoryModelCtx(ctx, "http://x", time.Now()) }()
	select {
	case m := <-got:
		if m.DegradedNote == "" {
			t.Fatalf("an abandoned cold read = %+v; want a model naming why", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the request ended and the cold read is still waiting on the build")
	}
}
