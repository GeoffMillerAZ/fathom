// learningthresholds.go — GL-12 (docs/plans/governance-learning.md, design
// doc §6 "Rules that learn"): the published, overridable thresholds GL-13's
// applyLearning reads to decide promotion (candidate → active) and
// retirement (active → retired). schema/learning/schema.cue's #Learning is
// the CUE shape; LearningThresholds is its Go mirror, loaded the same
// dual-mode way internal/review.LoadRiskTable loads schema/risk's
// #RiskTable — a repository that ships no `.fathom/learning.cue` gets the
// binary's own built-in defaults with zero configuration, and a present
// file overrides only the fields it names.
package semantic

import (
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// LearningThresholds holds the five configurable numbers design doc §6's
// promotion/retirement rule compares against. Every field is guaranteed
// positive after DefaultLearningThresholds()/LoadLearningThresholds() —
// callers never need a zero-means-unset fallback of their own.
type LearningThresholds struct {
	// PromoteCorroborations / PromoteMinSessions / PromoteWindowDays
	// (design doc §6): a candidate is promoted to active when
	// corroborations reach PromoteCorroborations from at least
	// PromoteMinSessions distinct sessions, with refutes at 0 over the
	// last PromoteWindowDays.
	PromoteCorroborations int
	PromoteMinSessions    int
	PromoteWindowDays     int

	// RetireRefutes / RetireNeverTriggeredDays (design doc §6): an active
	// rule is retired when refutes reach RetireRefutes with refutes
	// greater than corroborations, or when it goes unserved for
	// RetireNeverTriggeredDays.
	RetireRefutes            int
	RetireNeverTriggeredDays int
}

// Design doc §6's own stated numbers — DefaultLearningThresholds() and a
// config file that omits a field both resolve to these.
const (
	defaultPromoteCorroborations    = 3
	defaultPromoteMinSessions       = 2
	defaultPromoteWindowDays        = 30
	defaultRetireRefutes            = 3
	defaultRetireNeverTriggeredDays = 180
)

// DefaultLearningThresholds is the built-in table a caller uses when no
// `.fathom/learning.cue` is present (same dual-mode convention as
// internal/review.DefaultRiskTable).
func DefaultLearningThresholds() LearningThresholds {
	return LearningThresholds{
		PromoteCorroborations:    defaultPromoteCorroborations,
		PromoteMinSessions:       defaultPromoteMinSessions,
		PromoteWindowDays:        defaultPromoteWindowDays,
		RetireRefutes:            defaultRetireRefutes,
		RetireNeverTriggeredDays: defaultRetireNeverTriggeredDays,
	}
}

// learningThresholdFields maps each #Learning CUE field name to the
// LearningThresholds field LoadLearningThresholds overrides when present —
// keeping the field list in one place rather than five repeated
// lookup/assign blocks.
var learningThresholdFields = []struct {
	cueField string
	set      func(*LearningThresholds, int)
}{
	{"promote_corroborations", func(t *LearningThresholds, v int) { t.PromoteCorroborations = v }},
	{"promote_min_sessions", func(t *LearningThresholds, v int) { t.PromoteMinSessions = v }},
	{"promote_window_days", func(t *LearningThresholds, v int) { t.PromoteWindowDays = v }},
	{"retire_refutes", func(t *LearningThresholds, v int) { t.RetireRefutes = v }},
	{"retire_never_triggered_days", func(t *LearningThresholds, v int) { t.RetireNeverTriggeredDays = v }},
}

// LoadLearningThresholds reads and decodes the learning threshold config at
// path. A missing file (os.IsNotExist) returns DefaultLearningThresholds()
// with no error; a present file overrides only the fields it names, every
// absent field keeping its binary default — mirrors
// internal/review.LoadRiskTable's dual-mode discipline exactly.
func LoadLearningThresholds(path string) (LearningThresholds, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultLearningThresholds(), nil
		}
		return LearningThresholds{}, fmt.Errorf("semantic: read %s: %w", path, err)
	}

	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return LearningThresholds{}, fmt.Errorf("semantic: parse %s: %w", path, v.Err())
	}

	table := DefaultLearningThresholds()
	for _, f := range learningThresholdFields {
		fv := v.LookupPath(cue.ParsePath(f.cueField))
		if !fv.Exists() {
			continue
		}
		n, err := fv.Int64()
		if err != nil {
			return LearningThresholds{}, fmt.Errorf("semantic: %s: field %q: %w", path, f.cueField, err)
		}
		f.set(&table, int(n))
	}
	return table, nil
}
