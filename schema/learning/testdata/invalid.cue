package learning

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/semantic/learningthresholds_schema_test.go) asserts each
// fails when uncommented into a unification (repo convention,
// schema/AGENTS.md).
//
// 1. Zero promotion corroboration threshold (must be > 0):
// bad1: #Learning & {
// 	promote_corroborations: 0
// }
//
// 2. Negative retirement refute threshold (must be > 0):
// bad2: #Learning & {
// 	retire_refutes: -1
// }
//
// 3. A window given as a non-int:
// bad3: #Learning & {
// 	promote_window_days: 30.5
// }
