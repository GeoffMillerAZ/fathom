// Package strikeledger — valid test data that must unify with schema.cue.
package strikeledger

import "github.com/geoffmilleraz/agentic-kit/foundations/governance/strike-ledger:strikeledger"

// validConfig demonstrates a well-formed ledger configuration.
validConfig: strikeledger.#Config & {
	Threshold: 3
	BaseBlock: "1m"
	MaxBlock:  "1h"
}

// validStrike demonstrates a well-formed strike record.
validStrike: strikeledger.#Strike & {
	ID:         "a3f7b2c1d0e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0"
	Reason:     "excessive request rate"
	OccurredAt: "2024-11-01T09:00:00Z"
}

// validSnapshotEntry demonstrates a well-formed snapshot entry.
validSnapshotEntry: strikeledger.#SnapshotEntry & {
	Strikes: [
		{
			ID:         "a3f7b2c1d0e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0"
			Reason:     "spam"
			OccurredAt: "2024-11-01T09:00:00Z"
		},
		{
			ID:         "a3f7b2c1d0e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0"
			Reason:     "rate limit violation"
			OccurredAt: "2024-11-01T09:05:00Z"
		},
	]
	BlockCount: 1
}

// validEmptySnapshotEntry demonstrates a snapshot entry with no strikes (new user).
validEmptySnapshotEntry: strikeledger.#SnapshotEntry & {
	Strikes:    []
	BlockCount: 0
}
