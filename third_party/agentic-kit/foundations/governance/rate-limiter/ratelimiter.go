// Package ratelimiter provides a token-bucket rate limiter with sliding-window
// semantics, both single-key (Limiter) and per-key (KeyedLimiter) variants.
//
// Design constraints (WASM-safe):
//   - stdlib only: no third-party dependencies.
//   - No time.Now() in logic: callers inject a now func() time.Time.
//   - No direct file I/O: all persistence is via caller-managed state.
//   - Concurrency-safe via sync.Mutex.
//   - Deterministic and fail-closed: when tokens are exhausted, Allow returns false.
package ratelimiter

import (
	"sync"
	"sync/atomic"
	"time"
)

// Limiter is a token-bucket rate limiter for a single key.
// It is safe for concurrent use.
type Limiter struct {
	mu         sync.Mutex
	tokens     float64
	ratePerSec float64
	burst      int
	lastRefill time.Time
	now        func() time.Time
}

// New constructs a new Limiter with the given rate (tokens per second), burst
// capacity, and clock function. The bucket starts full (burst tokens available).
//
// ratePerSec must be > 0. burst must be ≥ 1.
func New(ratePerSec float64, burst int, now func() time.Time) *Limiter {
	return &Limiter{
		tokens:     float64(burst),
		ratePerSec: ratePerSec,
		burst:      burst,
		lastRefill: now(),
		now:        now,
	}
}

// Allow reports whether one token is available and consumes it.
// Returns false (fail-closed) when the bucket is empty.
func (l *Limiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN reports whether n tokens are available and consumes them atomically.
// Returns false if n > burst (can never be satisfied) or the bucket has fewer
// than n tokens.
func (l *Limiter) AllowN(n int) bool {
	if n <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	if n > l.burst {
		return false // can never satisfy; fail-closed
	}
	if l.tokens < float64(n) {
		return false
	}
	l.tokens -= float64(n)
	return true
}

// refill computes elapsed time since last refill and adds tokens accordingly.
// Must be called with l.mu held.
func (l *Limiter) refill() {
	now := l.now()
	elapsed := now.Sub(l.lastRefill)
	if elapsed <= 0 {
		return
	}
	l.lastRefill = now
	added := elapsed.Seconds() * l.ratePerSec
	l.tokens += added
	if l.tokens > float64(l.burst) {
		l.tokens = float64(l.burst)
	}
}

// ---- KeyedLimiter -----------------------------------------------------------

// bucket is a per-key token bucket plus last-seen timestamp for idle GC.
type bucket struct {
	lim      *Limiter
	lastSeen time.Time
}

// KeyedLimiter maintains independent token buckets per string key.
// An optional global cap limits total allows across all keys (0 = no cap).
// It is safe for concurrent use.
type KeyedLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	ratePerSec float64
	burst      int
	now        func() time.Time
	globalCap  int64
	globalUsed atomic.Int64
}

// NewKeyed constructs a KeyedLimiter. globalCap ≤ 0 means no global cap.
func NewKeyed(ratePerSec float64, burst int, now func() time.Time, globalCap int) *KeyedLimiter {
	kl := &KeyedLimiter{
		buckets:    make(map[string]*bucket),
		ratePerSec: ratePerSec,
		burst:      burst,
		now:        now,
		globalCap:  int64(globalCap),
	}
	return kl
}

// Allow reports whether one token is available for the given key and consumes it.
// Per-key bucket is created on first use.
func (kl *KeyedLimiter) Allow(key string) bool {
	// Check global cap first (fail-closed, fast path without key lock).
	if kl.globalCap > 0 {
		// Optimistic check; we'll do a proper CAS below.
		cur := kl.globalUsed.Load()
		if cur >= kl.globalCap {
			return false
		}
	}

	kl.mu.Lock()
	b, ok := kl.buckets[key]
	if !ok {
		lim := New(kl.ratePerSec, kl.burst, kl.now)
		b = &bucket{lim: lim, lastSeen: kl.now()}
		kl.buckets[key] = b
	}
	b.lastSeen = kl.now()
	lim := b.lim
	kl.mu.Unlock()

	if !lim.Allow() {
		return false
	}

	// Enforce global cap with CAS after the per-key token is tentatively consumed.
	if kl.globalCap > 0 {
		for {
			cur := kl.globalUsed.Load()
			if cur >= kl.globalCap {
				// Return the token (best-effort: add it back).
				lim.mu.Lock()
				lim.tokens++
				if lim.tokens > float64(lim.burst) {
					lim.tokens = float64(lim.burst)
				}
				lim.mu.Unlock()
				return false
			}
			if kl.globalUsed.CompareAndSwap(cur, cur+1) {
				break
			}
		}
	}
	return true
}

// GC evicts buckets that have been idle for longer than idleThreshold.
// Pass the current time (from the injected clock). Idle threshold is 1 minute.
func (kl *KeyedLimiter) GC(now time.Time) {
	const idleThreshold = time.Minute
	kl.mu.Lock()
	defer kl.mu.Unlock()
	for key, b := range kl.buckets {
		if now.Sub(b.lastSeen) > idleThreshold {
			delete(kl.buckets, key)
		}
	}
}
