// Package extractiongaplog — valid test fixtures.
// Running `cue vet ./...` and `cue vet ./testdata/` both validate these against schema.cue.
package extractiongaplog

// gapResolved: a gap record where the outcome was found.
gapResolved: #GapRecord & {
	concern_id:      "concern-001"
	tier1_key_missed: "key:concern-001:abc123"
	scope_searched:  "foundations/**/*.go"
	outcome:         "found"
	found_refs:      ["file://foundations/main.go#L10-L12"]
	occurred_at:     "2026-07-10T09:00:00Z"
}

// gapUnresolved: a gap record where the escalation timed out.
gapUnresolved: #GapRecord & {
	concern_id:    "concern-002"
	scope_searched: "docs/**/*.md"
	outcome:       "unresolved"
	occurred_at:   "2026-07-10T10:00:00Z"
}

// gapAbsentProven: a gap resolved as proven absent.
gapAbsentProven: #GapRecord & {
	concern_id:      "concern-003"
	tier1_key_missed: "key:concern-003:def456"
	scope_searched:  "runtimes/**/*.go"
	outcome:         "absent_proven"
	occurred_at:     "2026-07-10T11:00:00Z"
}

// gapAbsentUnverified: absence claimed but manifest was incomplete.
gapAbsentUnverified: #GapRecord & {
	concern_id:    "concern-004"
	scope_searched: "kits/**/*.go"
	outcome:       "absent_unverified"
	occurred_at:   "2026-07-10T12:00:00Z"
}
