package main

// console_memo.go — the workspace tabs' memo (design §1 "Caching"; budget
// "warm render ≤ 150 ms"). Each tab keeps one bounded map of recent reads
// keyed by their input watermarks, and the memo's lock guards that map and
// nothing else: a cold computation — an intent audit is 35–60 s on a large
// repository — runs outside it, and concurrent requests for the same key
// share its one run. The survey watched a cached tab wait 35 s behind
// another project's audit while one global lock was held across it.

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"
)

// consoleMemo is one tab's memo. The zero value is ready; max bounds the
// stored keys (0: unbounded).
type consoleMemo[V any] struct {
	mu      sync.Mutex
	max     int
	entries map[string]consoleMemoEntry[V]
	order   []string
	flights map[string]*consoleMemoFlight[V]
}

type consoleMemoEntry[V any] struct {
	v    V
	at   time.Time
	keep time.Duration // < 0: until evicted
}

func (e consoleMemoEntry[V]) fresh(now time.Time) bool {
	return e.keep < 0 || (now.Sub(e.at) < e.keep && !now.Before(e.at))
}

// consoleMemoFlight is one computation in progress; its waiters read v and
// at once done is closed.
type consoleMemoFlight[V any] struct {
	done chan struct{}
	v    V
	at   time.Time
}

// do returns the value stored under key while it is fresh at now.
// Otherwise it runs compute — outside the lock, once per key however many
// requests arrive meanwhile — and stores the value for as long as compute
// says: 0 is not stored (the next request retries: a failed read clears
// with the wedge that caused it), < 0 is kept until evicted. at is when
// the returned value was computed.
func (m *consoleMemo[V]) do(key string, now time.Time, compute func() (V, time.Duration)) (V, time.Time) {
	//fathom:tenancy-allow bg-ctx do() is the context-free wrapper for computations that take no context; callers with a request use doCtx
	v, at, _ := m.doCtx(context.Background(), key, now, compute)
	return v, at
}

// doCtx is do for a request: a waiter on another request's computation
// leaves when ctx ends (RT-62, §3.5), with ok false and the zero value —
// the computation itself runs on for the requests still waiting.
func (m *consoleMemo[V]) doCtx(ctx context.Context, key string, now time.Time, compute func() (V, time.Duration)) (V, time.Time, bool) {
	m.mu.Lock()
	if e, ok := m.entries[key]; ok && e.fresh(now) {
		m.mu.Unlock()
		return e.v, e.at, true
	}
	if fl, ok := m.flights[key]; ok {
		m.mu.Unlock()
		select {
		case <-fl.done:
			return fl.v, fl.at, true
		case <-ctx.Done():
			var zero V
			return zero, time.Time{}, false
		}
	}
	fl := &consoleMemoFlight[V]{done: make(chan struct{}), at: now}
	if m.flights == nil {
		m.flights = map[string]*consoleMemoFlight[V]{}
	}
	m.flights[key] = fl
	m.mu.Unlock()

	var keep time.Duration
	// Deferred so a computation that panics still releases its waiters.
	defer func() {
		m.mu.Lock()
		delete(m.flights, key)
		if keep != 0 {
			m.storeLocked(key, consoleMemoEntry[V]{v: fl.v, at: now, keep: keep})
		}
		m.mu.Unlock()
		close(fl.done)
	}()
	fl.v, keep = compute()
	return fl.v, now, true
}

// get returns the value stored under key while it is fresh at now.
func (m *consoleMemo[V]) get(key string, now time.Time) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[key]; ok && e.fresh(now) {
		return e.v, true
	}
	var zero V
	return zero, false
}

// put stores v under key, computed at, kept until evicted.
func (m *consoleMemo[V]) put(key string, v V, at time.Time) {
	m.mu.Lock()
	m.storeLocked(key, consoleMemoEntry[V]{v: v, at: at, keep: -1})
	m.mu.Unlock()
}

// storeLocked stores e under key as the newest entry, evicting the oldest
// past the bound.
func (m *consoleMemo[V]) storeLocked(key string, e consoleMemoEntry[V]) {
	if m.entries == nil {
		m.entries = map[string]consoleMemoEntry[V]{}
	}
	if _, ok := m.entries[key]; ok {
		for i, k := range m.order {
			if k == key {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
	}
	m.entries[key] = e
	m.order = append(m.order, key)
	for m.max > 0 && len(m.order) > m.max {
		delete(m.entries, m.order[0])
		m.order = m.order[1:]
	}
}

// reset forgets every stored value (tests; in-flight computations finish
// and store as usual).
func (m *consoleMemo[V]) reset() {
	m.mu.Lock()
	m.entries, m.order = nil, nil
	m.mu.Unlock()
}

// fileWatermark is a file's size and modification time: the cheap input
// watermark a memo keys on (design §1: "ledger size and mtime"). A missing
// file and one that could not be stat'ed are watermarks too, each distinct.
func fileWatermark(path string) string {
	fi, err := os.Stat(path)
	switch {
	case err != nil && os.IsNotExist(err):
		return "absent"
	case err != nil:
		return "unreadable:" + err.Error()
	}
	return strconv.FormatInt(fi.Size(), 10) + "@" + strconv.FormatInt(fi.ModTime().UnixNano(), 10)
}
