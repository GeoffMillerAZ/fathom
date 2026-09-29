package risk

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/review/risktable_schema_test.go) asserts each fails when
// uncommented into a unification (repo convention, schema/AGENTS.md).
//
// 1. Zero blast-radius threshold (must be > 0):
// bad1: #RiskTable & {
// 	blast_radius_high_callers: 0
// }
//
// 2. Negative zone-interest threshold (must be > 0):
// bad2: #RiskTable & {
// 	zone_interest_medium_rate: -1.0
// }
