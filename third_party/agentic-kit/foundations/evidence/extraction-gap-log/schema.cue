// Package extractiongaplog defines the CUE-first contract for gap records:
// the flywheel store for Tier-2 → Tier-3 escalation events. Each gap records
// a concern that fell through to Tier-3, enabling Tier-1 hit-rate projections
// and improvementflywheel.Signal emission.
//
// CUE is authoritative for shape; Go types in types.go and extractiongaplog.go
// mirror these definitions; any drift is a bug.
//
// Honest scope claim (doc'd): warmed entries are Tier-1 signal only, keyed to
// content hashes. They NEVER satisfy PreconditionSatisfied. Absence-shaped cache
// entries invalidate on any change under the concern's constraints.
package extractiongaplog

// #Outcome mirrors evidence-ladder Verdict values that can appear in a GapRecord.
#Outcome: "found" | "absent_proven" | "absent_unverified" | "unresolved"

// #GapRecord is the atomic gap-log entry.
#GapRecord: {
	// concern_id identifies the concern that fell through to Tier-3.
	concern_id: string & !=""

	// tier1_key_missed is the cache key that was absent from Tier-1, if known.
	tier1_key_missed?: string

	// scope_searched describes the scope Tier-2 searched (e.g. glob list, source ids).
	scope_searched: string

	// outcome is the eventual verdict from Tier-3, or "unresolved" on timeout.
	outcome: #Outcome

	// found_refs are the file:// or source:// references when outcome=="found".
	found_refs?: [...string]

	// occurred_at is the RFC3339 timestamp of the gap event (injected clock).
	occurred_at: string & !=""
}
