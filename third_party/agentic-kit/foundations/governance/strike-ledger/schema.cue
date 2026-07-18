// Package strikeledger defines the schema for strike-ledger configuration
// and snapshot records. Constraints mirror the Go types and are evaluated
// with `cue vet ./...`.
package strikeledger

// #Config constrains the strike-ledger configuration.
#Config: {
	// Threshold is the minimum number of strikes (since the last block, or
	// ever) required to trigger a block. Must be at least 1.
	Threshold: >=1 & int

	// BaseBlock is the duration string for the first block (e.g. "1m").
	BaseBlock: string & !=""

	// MaxBlock is the upper bound duration string for any single block (e.g. "1h").
	MaxBlock: string & !=""
}

// #Strike is an immutable violation record.
#Strike: {
	// ID is the hashed (privacy-preserving) identity of the actor.
	ID: string & !=""

	// Reason is a human-readable description of the violation.
	Reason: string & !=""

	// OccurredAt is the RFC 3339 timestamp when the strike was recorded.
	OccurredAt: string & !=""
}

// #SnapshotEntry holds the persisted state for a single identity.
#SnapshotEntry: {
	// Strikes is the ordered list of strike events for this identity.
	Strikes: [...#Strike]

	// BlockCount is the number of block episodes that have been applied.
	// Used for escalation tracking; must be non-negative.
	BlockCount: >=0 & int
}
