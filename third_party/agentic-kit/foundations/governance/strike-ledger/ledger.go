package strikeledger

import (
	"sync"
	"time"
)

// entry holds the mutable in-memory state for one identity.
type entry struct {
	strikes []Strike      // append-only
	blocks  []blockRecord // append-only
}

// Ledger is a concurrency-safe, append-only violation ledger with
// escalating-backoff blocklisting. Use New to create a Ledger.
type Ledger struct {
	cfg  Config
	now  func() time.Time
	mu   sync.Mutex
	data map[string]*entry
}

// New creates a ready-to-use Ledger. The now func is called for every
// timestamp so that callers can inject a controllable clock.
func New(cfg Config, now func() time.Time) *Ledger {
	return &Ledger{
		cfg:  cfg,
		now:  now,
		data: make(map[string]*entry),
	}
}

// Strike records a violation for id with the given reason and returns the
// current status. If the number of strikes since the last block equals or
// exceeds Config.Threshold, a new block is applied with an exponentially
// longer duration (clamped at Config.MaxBlock).
func (l *Ledger) Strike(id, reason string) Status {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	e := l.getOrCreate(id)

	// Append the new strike (immutable after this point).
	e.strikes = append(e.strikes, Strike{
		ID:         id,
		Reason:     reason,
		OccurredAt: now,
	})

	// Check whether we should apply a new block.
	// Count strikes since the last block was applied (or all strikes if none).
	strSinceLastBlock := l.strikesSinceLastBlock(e)
	if strSinceLastBlock >= l.cfg.Threshold {
		// Compute block duration: BaseBlock * 2^priorBlocks, capped at MaxBlock.
		dur := l.blockDuration(len(e.blocks))
		until := now.Add(dur)
		e.blocks = append(e.blocks, blockRecord{AppliedAt: now, Until: until})
	}

	return l.statusLocked(e, now)
}

// Blocked reports whether id is currently blocked and, if so, when the block
// expires. Returns (false, zero) if the identity is unknown or not blocked.
func (l *Ledger) Blocked(id string) (bool, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.data[id]
	if !ok {
		return false, time.Time{}
	}
	now := l.now()
	s := l.statusLocked(e, now)
	return s.Blocked, s.Until
}

// Strikes returns the total number of strikes ever recorded for id.
// Returns 0 for unknown identities.
func (l *Ledger) Strikes(id string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.data[id]
	if !ok {
		return 0
	}
	return len(e.strikes)
}

// Events returns an immutable copy of all strike events for id, in the order
// they were recorded. Returns an empty (non-nil) slice for unknown identities.
func (l *Ledger) Events(id string) []Strike {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.data[id]
	if !ok {
		return []Strike{}
	}
	out := make([]Strike, len(e.strikes))
	copy(out, e.strikes)
	return out
}

// Snapshot returns a plain-value representation of the entire ledger state.
// It is safe to call concurrently and produces a deep copy.
func (l *Ledger) Snapshot() Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries := make(map[string]SnapshotEntry, len(l.data))
	for id, e := range l.data {
		strikes := make([]Strike, len(e.strikes))
		copy(strikes, e.strikes)
		blocks := make([]blockRecord, len(e.blocks))
		copy(blocks, e.blocks)
		entries[id] = SnapshotEntry{Strikes: strikes, Blocks: blocks}
	}
	return Snapshot{Entries: entries}
}

// Restore loads the ledger state from a Snapshot, replacing any existing data.
// It is intended for in-pod restart durability. No file I/O is performed.
func (l *Ledger) Restore(snap Snapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.data = make(map[string]*entry, len(snap.Entries))
	for id, se := range snap.Entries {
		strikes := make([]Strike, len(se.Strikes))
		copy(strikes, se.Strikes)
		blocks := make([]blockRecord, len(se.Blocks))
		copy(blocks, se.Blocks)
		l.data[id] = &entry{strikes: strikes, blocks: blocks}
	}
}

// ---- internal helpers -------------------------------------------------------

func (l *Ledger) getOrCreate(id string) *entry {
	e, ok := l.data[id]
	if !ok {
		e = &entry{}
		l.data[id] = e
	}
	return e
}

// statusLocked computes the current Status for e. Must be called with l.mu held.
func (l *Ledger) statusLocked(e *entry, now time.Time) Status {
	blocked := false
	var until time.Time

	// The most recent block is always at the end; check it first.
	if len(e.blocks) > 0 {
		last := e.blocks[len(e.blocks)-1]
		if now.Before(last.Until) {
			blocked = true
			until = last.Until
		}
	}
	return Status{
		Strikes: len(e.strikes),
		Blocked: blocked,
		Until:   until,
	}
}

// strikesSinceLastBlock counts strikes added strictly after the most recent
// block was applied. If there are no blocks, all strikes are counted.
//
// "Strictly after" (not equal) ensures that the strikes which triggered the
// block itself are not double-counted when checking whether a subsequent block
// should be applied.
func (l *Ledger) strikesSinceLastBlock(e *entry) int {
	if len(e.blocks) == 0 {
		return len(e.strikes)
	}
	lastBlock := e.blocks[len(e.blocks)-1]
	count := 0
	for _, s := range e.strikes {
		if s.OccurredAt.After(lastBlock.AppliedAt) {
			count++
		}
	}
	return count
}

// blockDuration computes BaseBlock * 2^priorBlocks, clamped to MaxBlock.
func (l *Ledger) blockDuration(priorBlocks int) time.Duration {
	dur := l.cfg.BaseBlock
	for i := 0; i < priorBlocks; i++ {
		dur *= 2
		if dur >= l.cfg.MaxBlock {
			return l.cfg.MaxBlock
		}
	}
	if dur > l.cfg.MaxBlock {
		return l.cfg.MaxBlock
	}
	return dur
}
