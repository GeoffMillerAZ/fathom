package dlog

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// dedupeState backs Logger.Dedupe/FlushDedupe. Mirrors the shape of
// internal/semantic/packs.go's brokenPackWarnMu/brokenPackWarned
// once-per-process pattern, generalized to count repeats instead of just
// suppressing them (DG-2: no log call site anywhere dedupes or
// rate-limits a repeating condition before this).
type dedupeState struct {
	mu      sync.Mutex
	entries map[string]*dedupeEntry
}

type dedupeEntry struct {
	level     Level
	msg       string // the fully-rendered (fmt.Sprintf'd) first message
	firstSeen time.Time
	repeats   int // suppressed occurrences since the first emission
}

func newDedupeState() *dedupeState {
	return &dedupeState{entries: make(map[string]*dedupeEntry)}
}

// Dedupe logs msg (a printf-style format string, matching the
// fmt.Fprintf call sites it replaces — e.g. "fleet: sources for %s: %v")
// immediately the first time key is seen, and on every subsequent call with
// the same key increments a counter instead of emitting anything. Call
// FlushDedupe to emit a summary line for keys that repeated.
//
// key scopes the dedup — typically per call site plus the identity of what
// is failing (e.g. "fleet:sources:"+service), never per line, so a burst of
// N failures for the SAME service collapses to one immediate line plus one
// eventual summary, while a failure for a DIFFERENT service still gets its
// own first-occurrence line.
func (l *Logger) Dedupe(level Level, key, msg string, args ...any) {
	rendered := fmt.Sprintf(msg, args...)

	l.dedupe.mu.Lock()
	e, seen := l.dedupe.entries[key]
	if !seen {
		l.dedupe.entries[key] = &dedupeEntry{level: level, msg: rendered, firstSeen: l.now()}
		l.dedupe.mu.Unlock()
		l.emit(level, rendered)
		return
	}
	e.repeats++
	l.dedupe.mu.Unlock()
}

// FlushDedupe emits one "<msg> (repeated N times since <first-seen>)" line
// per deduped key whose repeat count is nonzero, then clears every tracked
// key (repeated or not) so the next occurrence starts a fresh window. now
// is the flush event's own timestamp (injected — never time.Now directly);
// each summary line's "since <first-seen>" text uses that key's OWN
// first-emission time, captured when Dedupe first saw it.
//
// Callers: a ticker (piggybacking the daemon's existing scavenge ticker, or
// its own) and once more at graceful shutdown, so a burst that never
// repeats again before exit still gets its summary recorded.
func (l *Logger) FlushDedupe(now time.Time) {
	type summary struct {
		level     Level
		key       string
		msg       string
		firstSeen time.Time
		repeats   int
	}

	l.dedupe.mu.Lock()
	pending := make([]summary, 0, len(l.dedupe.entries))
	for key, e := range l.dedupe.entries {
		if e.repeats > 0 {
			pending = append(pending, summary{level: e.level, key: key, msg: e.msg, firstSeen: e.firstSeen, repeats: e.repeats})
		}
		delete(l.dedupe.entries, key)
	}
	l.dedupe.mu.Unlock()

	// Determinism contract (internal/CLAUDE.md): map iteration order must
	// never reach a result — sort by key before emitting.
	sort.Slice(pending, func(i, j int) bool { return pending[i].key < pending[j].key })

	for _, p := range pending {
		line := fmt.Sprintf("%s (repeated %d times since %s)", p.msg, p.repeats, p.firstSeen.UTC().Format(time.RFC3339))
		l.emitAt(now, p.level, line)
	}
}
