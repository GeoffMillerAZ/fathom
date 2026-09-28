package learning

// A well-formed learning table: every threshold overridden.
withEverything: #Learning & {
	promote_corroborations:      5
	promote_min_sessions:        3
	promote_window_days:         45
	retire_refutes:              4
	retire_never_triggered_days: 120
}

// A single-field override — the loader fills the rest from the binary's
// defaults.
partial: #Learning & {
	retire_refutes: 2
}

// The legitimate "no overrides" empty form — a repository that ships a
// learning.cue purely to document intent to override later.
empty: #Learning & {}
