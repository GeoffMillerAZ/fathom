// Package strikeledger provides an append-only violation ledger with
// escalating-backoff blocklisting. It is concurrency-safe, deterministic,
// and uses an injected clock so callers retain full control over time in
// tests. No file I/O is performed; persistence is via Snapshot/Restore.
package strikeledger

import "time"

// Config controls the strike-ledger behaviour.
type Config struct {
	// Threshold is the number of strikes (since the last block was applied, or
	// ever for a first-time offender) that triggers a block.
	Threshold int

	// BaseBlock is the duration of the first block. Subsequent blocks are
	// exponentially longer: BaseBlock * 2^priorBlocks, capped at MaxBlock.
	BaseBlock time.Duration

	// MaxBlock is the upper bound on any single block duration.
	MaxBlock time.Duration
}

// Strike is an immutable record of a single violation event.
type Strike struct {
	// ID is the hashed identity of the actor.
	ID string

	// Reason is a human-readable description of the violation.
	Reason string

	// OccurredAt is the injected clock time when the strike was recorded.
	OccurredAt time.Time
}

// Status is the result of a Strike call.
type Status struct {
	// Strikes is the total number of strikes on record for the identity.
	Strikes int

	// Blocked indicates whether the identity is currently blocked.
	Blocked bool

	// Until is the time at which the current block expires. Zero if not blocked.
	Until time.Time
}

// blockRecord captures a single block episode for replay / escalation.
type blockRecord struct {
	// AppliedAt is when the block was applied.
	AppliedAt time.Time

	// Until is when the block expires.
	Until time.Time
}

// Snapshot is a plain-value representation of the ledger state, suitable for
// serialisation or in-memory hand-off between restarts. It contains no file
// handles, channels, or mutexes.
type Snapshot struct {
	// Entries maps identity → full event/block history for that identity.
	Entries map[string]SnapshotEntry
}

// SnapshotEntry holds the persisted state for a single identity.
type SnapshotEntry struct {
	// Strikes is the append-only list of strike events.
	Strikes []Strike

	// Blocks is the list of block episodes in the order they were applied.
	Blocks []blockRecord
}
