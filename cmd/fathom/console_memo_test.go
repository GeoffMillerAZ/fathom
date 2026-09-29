package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConsoleMemo_ColdKeyNeverBlocksWarmKey pins the tab memos' locking
// (design §1: warm render ≤ 150 ms): the memo's lock guards its map, never
// a computation. While one key computes (an intent audit takes 35–60 s on a
// large repository), a request for another key that is already stored
// answers at once, and concurrent requests for the computing key share its
// one run.
func TestConsoleMemo_ColdKeyNeverBlocksWarmKey(t *testing.T) {
	var m consoleMemo[string]
	m.max = 4
	now := time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC)
	if v, _ := m.do("warm", now, func() (string, time.Duration) { return "w", time.Minute }); v != "w" {
		t.Fatalf("warm = %q", v)
	}
	release := make(chan struct{})
	started := make(chan struct{})
	var runs atomic.Int32
	cold := func() (string, time.Duration) {
		if runs.Add(1) == 1 {
			close(started)
		}
		<-release
		return "c", time.Minute
	}
	var wg sync.WaitGroup
	results := make([]string, 3)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], _ = m.do("cold", now, cold)
		}()
		if i == 0 {
			<-started
		}
	}
	done := make(chan string, 1)
	go func() {
		v, _ := m.do("warm", now.Add(time.Second), func() (string, time.Duration) { return "recomputed", time.Minute })
		done <- v
	}()
	select {
	case v := <-done:
		if v != "w" {
			t.Errorf("warm key = %q, want its stored value", v)
		}
	case <-time.After(waitSafety):
		t.Fatal("a stored key waited behind another key's computation")
	}
	close(release)
	wg.Wait()
	for i, v := range results {
		if v != "c" {
			t.Errorf("request %d for the cold key = %q", i, v)
		}
	}
	if n := runs.Load(); n != 1 {
		t.Errorf("concurrent requests for one key ran it %d times, want once", n)
	}
	// Stored for its keep, then recomputed; a zero keep is never stored.
	if v, _ := m.do("cold", now.Add(2*time.Minute), func() (string, time.Duration) { return "c2", 0 }); v != "c2" {
		t.Errorf("an expired key must recompute: %q", v)
	}
	if v, _ := m.do("cold", now.Add(2*time.Minute), func() (string, time.Duration) { return "c3", time.Minute }); v != "c3" {
		t.Errorf("a zero keep must not be stored: %q", v)
	}
	// Bounded: the oldest key goes first.
	for _, k := range []string{"a", "b", "c", "d"} {
		m.do(k, now, func() (string, time.Duration) { return k, -1 })
	}
	if len(m.entries) > m.max {
		t.Errorf("memo holds %d entries, bound %d", len(m.entries), m.max)
	}
}

// TestConsoleMemoWaiterHonoursContext (RT-62, §3.5): a console memo waiter
// on another request's computation leaves when its context ends instead of
// parking behind it, while the computation runs on for its own caller.
// (The console page memos are not memo groups yet — I51's open item — so
// this pins their waiters directly.)
func TestConsoleMemoWaiterHonoursContext(t *testing.T) {
	var m consoleMemo[int]
	release := make(chan struct{})
	first := make(chan int, 1)
	go func() {
		v, _ := m.do("k", time.Now(), func() (int, time.Duration) { <-release; return 1, -1 })
		first <- v
	}()
	deadline := time.Now().Add(waitSafety)
	for {
		m.mu.Lock()
		inFlight := m.flights["k"] != nil
		m.mu.Unlock()
		if inFlight {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the first computation never started")
		}
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	// The first computation stays blocked (release closes below), so a waiter
	// that outstayed its ctx could not return; the 30 ms is the caller's input.
	var ok bool
	returnsUnaided(t, "a memo waiter whose ctx ended", func() {
		_, _, ok = m.doCtx(ctx, "k", time.Now(), func() (int, time.Duration) { return 2, -1 })
	})
	if ok {
		t.Fatal("a waiter whose ctx ended reported a value")
	}
	close(release)
	if v := <-first; v != 1 {
		t.Fatalf("the computation's own caller got %d", v)
	}
}
