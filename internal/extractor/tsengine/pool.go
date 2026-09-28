package tsengine

import (
	"container/list"
	"context"
	"strings"
	"sync"
	"time"
)

// MaxLiveModules is the process-wide cap on live grammar modules (LB-F15,
// design §9): each is up to 64 MiB of linear memory (MemoryLimitPages), so
// the cap — not the number of dialects — bounds what extraction can hold.
// No language raises it.
const MaxLiveModules = 6

// PoolIdleClose is how long a released module may sit unused before the
// pool closes it.
const PoolIdleClose = 2 * time.Minute

// parserPool is the process-wide pool of instantiated grammar modules. Each
// entry is one Parser under a caller's key (an extractor's variant key,
// which names its owner); a key's Parser is reused across Acquire/Release
// cycles, so a module is instantiated once per key while it stays in the
// pool. Past the cap, acquiring closes the least recently used idle
// module; leased modules are never closed under their holder, so when every
// module is leased a new one is made anyway and the surplus is closed at its
// release — the cap holds whenever nothing is mid-extraction, and one
// sequential ingest leases at most two (a host file and an injected range).
type parserPool struct {
	mu      sync.Mutex
	max     int
	idle    time.Duration
	now     func() time.Time
	lru     *list.List // front = most recently released; values *poolEntry
	byKey   map[string]*poolEntry
	live    int
	peak    int // the most modules held at once since ResetPoolPeak
	timer   *time.Timer
	noTimer bool // tests sweep by hand
}

type poolEntry struct {
	key        string
	p          *Parser
	leased     bool
	closeOnRel bool
	lastUsed   time.Time
	elem       *list.Element
}

func newParserPool(max int, idle time.Duration, now func() time.Time) *parserPool {
	return &parserPool{max: max, idle: idle, now: now, lru: list.New(), byKey: map[string]*poolEntry{}}
}

var defaultPool = newParserPool(MaxLiveModules, PoolIdleClose, time.Now)

// Lease is one caller's exclusive hold on a pooled Parser. Release returns
// it to the pool; Discard closes it (after an engine fault).
type Lease struct {
	pool *parserPool
	e    *poolEntry
	done bool
}

// Parser is the leased parser. It must not be used after Release/Discard.
func (l *Lease) Parser() *Parser { return l.e.p }

// Release returns the parser to the pool for reuse under its key.
func (l *Lease) Release() {
	if l.done {
		return
	}
	l.done = true
	l.pool.release(l.e, false)
}

// Discard closes the parser's module and drops it from the pool: the next
// acquire for the key instantiates afresh.
func (l *Lease) Discard() {
	if l.done {
		return
	}
	l.done = true
	l.pool.release(l.e, true)
}

// AcquireParser leases the pooled Parser for key, instantiating grammar's
// module (entryPoint) when the key has none (LB-F15). key identifies the
// caller and the variant, so one owner's Parser — and the queries compiled
// on it (Parser.Cached) — is never handed to another. A key has at most one
// Parser; acquiring a key whose Parser is leased is a caller error answered
// with a second, unpooled module closed at its release.
func AcquireParser(ctx context.Context, key, grammar, entryPoint string) (*Lease, error) {
	return defaultPool.acquire(ctx, key, grammar, entryPoint)
}

// CloseParsers closes every pooled module whose key starts with prefix (an
// owner closing): idle ones now, leased ones at their release.
func CloseParsers(prefix string) { defaultPool.closePrefix(prefix) }

// CloseIdleParsers closes every idle pooled module (the refuse rung; a test
// wanting a clean baseline).
func CloseIdleParsers() { defaultPool.closeIdle() }

// PooledModules is how many modules the pool holds, leased or idle.
func PooledModules() int {
	defaultPool.mu.Lock()
	defer defaultPool.mu.Unlock()
	return defaultPool.live
}

// PoolPeak is the most modules the pool has held at once since
// ResetPoolPeak — the tenancy proof's instrument (TestPolyglotIngestHoldsModulePool).
func PoolPeak() int {
	defaultPool.mu.Lock()
	defer defaultPool.mu.Unlock()
	return defaultPool.peak
}

// ResetPoolPeak restarts PoolPeak from the modules held now.
func ResetPoolPeak() {
	defaultPool.mu.Lock()
	defaultPool.peak = defaultPool.live
	defaultPool.mu.Unlock()
}

func (pp *parserPool) acquire(ctx context.Context, key, grammar, entryPoint string) (*Lease, error) {
	pp.mu.Lock()
	if e, ok := pp.byKey[key]; ok && !e.leased {
		e.leased = true
		pp.lru.Remove(e.elem)
		e.elem = nil
		pp.mu.Unlock()
		return &Lease{pool: pp, e: e}, nil
	}
	_, keyBusy := pp.byKey[key]
	// Make room before instantiating: close least recently used idle
	// modules until one more fits the cap.
	var victims []*Parser
	for pp.live >= pp.max {
		back := pp.lru.Back()
		if back == nil {
			break // every module is leased
		}
		v := back.Value.(*poolEntry)
		pp.removeLocked(v)
		victims = append(victims, v.p)
	}
	pp.live++ // reserve the slot while instantiating unlocked
	if pp.live > pp.peak {
		pp.peak = pp.live
	}
	pp.mu.Unlock()
	for _, v := range victims {
		_ = v.Close()
	}

	p, err := NewParser(ctx, grammar, entryPoint)
	pp.mu.Lock()
	defer pp.mu.Unlock()
	if err != nil {
		pp.live--
		return nil, err
	}
	e := &poolEntry{key: key, p: p, leased: true}
	if keyBusy {
		e.closeOnRel = true // a second module for a leased key: never pooled
	} else {
		pp.byKey[key] = e
	}
	return &Lease{pool: pp, e: e}, nil
}

func (pp *parserPool) release(e *poolEntry, discard bool) {
	pp.mu.Lock()
	e.leased = false
	e.lastUsed = pp.now()
	closeIt := discard || e.closeOnRel || pp.live > pp.max
	if closeIt {
		pp.removeLocked(e)
	} else {
		e.elem = pp.lru.PushFront(e)
		pp.armLocked()
	}
	pp.mu.Unlock()
	if closeIt {
		_ = e.p.Close()
	}
}

// removeLocked drops e from the pool's books (its module is the caller's to
// close, outside the lock).
func (pp *parserPool) removeLocked(e *poolEntry) {
	if e.elem != nil {
		pp.lru.Remove(e.elem)
		e.elem = nil
	}
	if cur, ok := pp.byKey[e.key]; ok && cur == e {
		delete(pp.byKey, e.key)
	}
	pp.live--
}

// armLocked schedules the idle sweep while idle modules exist.
func (pp *parserPool) armLocked() {
	if pp.noTimer || pp.timer != nil || pp.lru.Len() == 0 {
		return
	}
	pp.timer = time.AfterFunc(pp.idle, func() {
		pp.mu.Lock()
		pp.timer = nil
		pp.mu.Unlock()
		pp.sweep()
	})
}

// sweep closes modules idle for at least the pool's idle bound and re-arms
// the timer while idle modules remain.
func (pp *parserPool) sweep() {
	pp.mu.Lock()
	now := pp.now()
	var victims []*Parser
	for el := pp.lru.Back(); el != nil; {
		prev := el.Prev()
		e := el.Value.(*poolEntry)
		if now.Sub(e.lastUsed) >= pp.idle {
			pp.removeLocked(e)
			victims = append(victims, e.p)
		}
		el = prev
	}
	pp.armLocked()
	pp.mu.Unlock()
	for _, v := range victims {
		_ = v.Close()
	}
}

func (pp *parserPool) closePrefix(prefix string) {
	pp.mu.Lock()
	var victims []*Parser
	for key, e := range pp.byKey {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if e.leased {
			e.closeOnRel = true
			delete(pp.byKey, key)
			continue
		}
		pp.removeLocked(e)
		victims = append(victims, e.p)
	}
	pp.mu.Unlock()
	for _, v := range victims {
		_ = v.Close()
	}
}

func (pp *parserPool) closeIdle() {
	pp.mu.Lock()
	var victims []*Parser
	for el := pp.lru.Front(); el != nil; el = pp.lru.Front() {
		e := el.Value.(*poolEntry)
		pp.removeLocked(e)
		victims = append(victims, e.p)
	}
	pp.mu.Unlock()
	for _, v := range victims {
		_ = v.Close()
	}
}

// closeAll closes every module, leased ones included (a test pool's
// cleanup).
func (pp *parserPool) closeAll() {
	pp.closeIdle()
	pp.mu.Lock()
	var victims []*Parser
	for key, e := range pp.byKey {
		delete(pp.byKey, key)
		pp.live--
		victims = append(victims, e.p)
	}
	pp.mu.Unlock()
	for _, v := range victims {
		_ = v.Close()
	}
}

// has reports whether key has a pooled module (tests).
func (pp *parserPool) has(key string) bool {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	_, ok := pp.byKey[key]
	return ok
}
