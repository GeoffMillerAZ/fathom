// Package identityquota provides a PER-IDENTITY rolling-day quota, the
// companion to budgetguard's aggregate one.
//
// Why this exists: an aggregate budget is a single global counter, so one
// caller operating within its own rate limit can consume the entire daily
// allowance and deny the endpoint to everyone else. On a public endpoint that
// is the cheapest attack available — a for-loop — and it takes the service
// dark precisely when a real visitor arrives. Rate limiting alone does not fix
// it: a rate limit bounds requests per second, not share of the day.
//
// This guard bounds what a SINGLE identity may consume per rolling day, and is
// meant to sit in front of the aggregate guard: identity quota first (cheap,
// local), then the global budget.
//
// Memory is itself an attack surface here — a naive map keyed by client
// identity grows without bound as an attacker rotates addresses. The guard is
// therefore explicitly capacity-bounded (MaxIdentities) with TTL eviction, and
// when it is full of live entries it FAILS CLOSED rather than evicting an
// active limiter (which would let an attacker reset their own quota by
// flooding new identities).
//
// Design constraints (WASM-safe), matching budgetguard:
//   - stdlib only: no third-party dependencies.
//   - No time.Now() in logic: callers inject a now func() time.Time.
//   - No direct file I/O: persistence via Snapshot/Restore (plain structs).
//   - Concurrency-safe via sync.Mutex.
//   - Deterministic and fail-closed.
package identityquota

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Config holds the per-identity limits.
type Config struct {
	// MaxReqPerDay is the maximum requests a single identity may make per
	// rolling day. Must be ≥ 1.
	MaxReqPerDay int

	// MaxUSDPerDay is the maximum estimated USD a single identity may consume
	// per rolling day. Must be > 0.
	MaxUSDPerDay float64

	// MaxIdentities bounds how many identities are tracked at once. This is a
	// memory bound, not a policy bound: reaching it means the guard is under
	// identity-rotation pressure. Must be ≥ 1.
	MaxIdentities int

	// IdentityTTL is how long an idle identity's counters are retained before
	// becoming evictable. Must be > 0. Should be ≥ the rolling day so a
	// quota cannot be reset early by going quiet.
	IdentityTTL time.Duration
}

// Decision reports whether a charge was permitted.
type Decision struct {
	// Allowed is true when the identity is within all of its limits.
	Allowed bool

	// Tripped is the complement of Allowed.
	Tripped bool

	// Reason describes the first limit exceeded. Empty when Allowed.
	Reason string
}

// entry is one identity's rolling-day state.
type entry struct {
	ReqThisDay int
	USDThisDay float64
	DayWindow  time.Time
	LastSeen   time.Time
}

// SnapshotEntry is one identity's persisted state.
type SnapshotEntry struct {
	Identity   string    `json:"identity"`
	ReqThisDay int       `json:"reqThisDay"`
	USDThisDay float64   `json:"usdThisDay"`
	DayWindow  time.Time `json:"dayWindow"`
	LastSeen   time.Time `json:"lastSeen"`
}

// Snapshot is the guard's persistable state.
type Snapshot struct {
	Entries []SnapshotEntry `json:"entries"`
}

// Guard enforces per-identity rolling-day request and USD quotas.
//
// It is safe for concurrent use.
type Guard struct {
	mu      sync.Mutex
	cfg     Config
	now     func() time.Time
	entries map[string]*entry
}

// New constructs a Guard. A nil now defaults to time.Now.
func New(cfg Config, now func() time.Time) *Guard {
	if now == nil {
		now = time.Now
	}
	return &Guard{cfg: cfg, now: now, entries: make(map[string]*entry)}
}

// Validate reports whether the configuration is usable. Zero values here are
// dangerous — MaxReqPerDay of 0 would deny every request — so callers should
// validate at construction rather than discovering it in production.
func (c Config) Validate() error {
	if c.MaxReqPerDay < 1 {
		return fmt.Errorf("identityquota: MaxReqPerDay must be >= 1, got %d", c.MaxReqPerDay)
	}
	if c.MaxUSDPerDay <= 0 {
		return fmt.Errorf("identityquota: MaxUSDPerDay must be > 0, got %v", c.MaxUSDPerDay)
	}
	if c.MaxIdentities < 1 {
		return fmt.Errorf("identityquota: MaxIdentities must be >= 1, got %d", c.MaxIdentities)
	}
	if c.IdentityTTL <= 0 {
		return fmt.Errorf("identityquota: IdentityTTL must be > 0, got %v", c.IdentityTTL)
	}
	return nil
}

// Charge records one request and its estimated cost against an identity,
// returning whether it is permitted. A denied charge records nothing.
func (g *Guard) Charge(identity string, estUSD float64) Decision {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	e := g.entries[identity]

	if e == nil {
		// Admitting a new identity may require space.
		if len(g.entries) >= g.cfg.MaxIdentities && !g.evictOneLocked(now) {
			// Every tracked identity is still live. Refusing here is
			// deliberate: evicting an active limiter would let an attacker
			// clear their own quota by flooding fresh identities.
			return Decision{
				Tripped: true,
				Reason:  "identity table at capacity; try again later",
			}
		}
		e = &entry{DayWindow: now}
		g.entries[identity] = e
	}

	// Roll the day window forward if it has elapsed.
	if now.Sub(e.DayWindow) >= 24*time.Hour {
		e.ReqThisDay = 0
		e.USDThisDay = 0
		e.DayWindow = now
	}
	e.LastSeen = now

	if e.ReqThisDay+1 > g.cfg.MaxReqPerDay {
		return Decision{
			Tripped: true,
			Reason:  fmt.Sprintf("per-identity daily request quota exhausted (%d/day)", g.cfg.MaxReqPerDay),
		}
	}
	if e.USDThisDay+estUSD > g.cfg.MaxUSDPerDay {
		return Decision{
			Tripped: true,
			Reason:  fmt.Sprintf("per-identity daily spend quota exhausted ($%.2f/day)", g.cfg.MaxUSDPerDay),
		}
	}

	e.ReqThisDay++
	e.USDThisDay += estUSD
	return Decision{Allowed: true}
}

// evictOneLocked drops the least-recently-seen entry that is past its TTL.
// Reports whether space was freed. Caller must hold the lock.
func (g *Guard) evictOneLocked(now time.Time) bool {
	var oldestKey string
	var oldest time.Time
	for k, e := range g.entries {
		if now.Sub(e.LastSeen) < g.cfg.IdentityTTL {
			continue // still live — never evict to make room
		}
		if oldestKey == "" || e.LastSeen.Before(oldest) {
			oldestKey, oldest = k, e.LastSeen
		}
	}
	if oldestKey == "" {
		return false
	}
	delete(g.entries, oldestKey)
	return true
}

// GC drops every entry past its TTL. Callers should run it periodically; it is
// not required for correctness, only for steady-state memory.
func (g *Guard) GC() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	n := 0
	for k, e := range g.entries {
		if now.Sub(e.LastSeen) >= g.cfg.IdentityTTL {
			delete(g.entries, k)
			n++
		}
	}
	return n
}

// Len reports how many identities are currently tracked.
func (g *Guard) Len() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.entries)
}

// Snapshot returns the guard's state for persistence. Entries are sorted by
// identity so the output is deterministic (diffable, and stable across
// restarts for tests).
func (g *Guard) Snapshot() Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := Snapshot{Entries: make([]SnapshotEntry, 0, len(g.entries))}
	for k, e := range g.entries {
		out.Entries = append(out.Entries, SnapshotEntry{
			Identity: k, ReqThisDay: e.ReqThisDay, USDThisDay: e.USDThisDay,
			DayWindow: e.DayWindow, LastSeen: e.LastSeen,
		})
	}
	sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].Identity < out.Entries[j].Identity })
	return out
}

// Restore replaces the guard's state. Entries beyond MaxIdentities are
// dropped (most-recently-seen kept) so a tampered or stale snapshot cannot
// exceed the memory bound.
func (g *Guard) Restore(snap Snapshot) {
	g.mu.Lock()
	defer g.mu.Unlock()
	ents := append([]SnapshotEntry(nil), snap.Entries...)
	sort.Slice(ents, func(i, j int) bool { return ents[i].LastSeen.After(ents[j].LastSeen) })
	if len(ents) > g.cfg.MaxIdentities {
		ents = ents[:g.cfg.MaxIdentities]
	}
	g.entries = make(map[string]*entry, len(ents))
	for _, se := range ents {
		g.entries[se.Identity] = &entry{
			ReqThisDay: se.ReqThisDay, USDThisDay: se.USDThisDay,
			DayWindow: se.DayWindow, LastSeen: se.LastSeen,
		}
	}
}
