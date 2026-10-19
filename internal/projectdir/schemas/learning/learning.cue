// Package learning defines the CUE schema for a repository's OPTIONAL
// promotion/retirement threshold table (GL-12, docs/plans/
// governance-learning.md; design doc §6 "Rules that learn"): the
// published, overridable thresholds GL-13's applyLearning reads
// (`.fathom/learning.cue`). A repository that ships none gets the
// binary's own built-in defaults (design doc §6's own stated numbers) —
// the same dual-mode convention as schema/risk/schema.cue.
//
// Nothing here changes WHAT is learned or HOW a promotion/retirement is
// announced (GL-X1: every state change is announced with its signals,
// never silent) — only the numeric thresholds a candidate or active rule
// is measured against.
package learning

// #Learning is the top-level shape of a learning threshold file. Every
// field is optional: a partially-specified file overrides only the
// thresholds it names, the loader fills the rest from the binary's
// defaults (design doc §6).
#Learning: {
	// promote_corroborations is the corroboration count a candidate rule
	// must reach, from at least promote_min_sessions distinct sessions,
	// to be promoted to active (design doc §6: "corroborations reach at
	// least 3 from at least 2 distinct sessions").
	promote_corroborations?: int & >0

	// promote_min_sessions is the distinct-session floor for the same
	// promotion condition.
	promote_min_sessions?: int & >0

	// promote_window_days is the window (ending now) over which refutes
	// must be zero for a candidate to promote (design doc §6: "refutes
	// are 0 over 30 days").
	promote_window_days?: int & >0

	// retire_refutes is the refute count, with refutes strictly greater
	// than corroborations, above which an active rule is retired (design
	// doc §6: "refutes reach at least 3 with refutes greater than
	// corroborations").
	retire_refutes?: int & >0

	// retire_never_triggered_days is how long an active rule may go
	// unserved before it is retired for that reason alone (design doc §6:
	// "never triggered for 180 days").
	retire_never_triggered_days?: int & >0
}
