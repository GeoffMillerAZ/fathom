// Package scenariobank provides versioned eval scenarios as CUE data with
// provenance discipline. It replaces ad-hoc scenario lists with a validated,
// PII-safe data contract and compiles scenario rubrics directly into the
// existing qa-harness RubricJudge machinery.
//
// # Provenance discipline
//
// Every scenario carries a Provenance tag that gates usage:
//
//   - ProductionCaptured: raw production data. MUST NOT appear in committed
//     testdata (Validate rejects it). Only permitted at runtime when the caller
//     has applied a DLP/redaction pass. A PII hazard.
//   - ProductionDerivedRedacted: production data that has passed a redaction
//     pipeline. Safe for testdata.
//   - SyntheticAdversarial: synthetic data crafted to stress-test edge cases.
//     MUST have DecisiveEligible=false — synthetic scenarios are canaries, not
//     ground truth.
//
// # RubricJudgeFrom
//
// RubricJudgeFrom compiles a Rubric into a qaharness.Judge backed by a
// qaharness.RubricJudge. Predicates are pure Go — no LLM, no closures in
// data. The compiled judge is deterministic given identical Item inputs.
//
// Supported predicate kinds (see #PredicateKind in schema.cue):
//
//   - "contains": strings.Contains(output, arg)
//   - "regex": regexp.MustCompile(arg).MatchString(output)
//   - "json_path_eq": json.Unmarshal(output) then navigate path in arg,
//     compare to predicate value (JSON pointer notation, "/" prefix)
//   - "nonempty": len(strings.TrimSpace(output)) > 0
//
// # HistoryFor
//
// HistoryFor is a pure projection over []evaljournal.Record — no stored state.
// It returns all records for a given scenarioID, in append order.
//
// # Concurrency contract
//
// Bank values are immutable after Load; share across goroutines freely.
// RubricJudgeFrom returns a qaharness.RubricJudge which is also immutable and
// concurrency-safe. HistoryFor is a pure function; safe to call concurrently.
package scenariobank

import "encoding/json"

// Provenance enumerates the permissible origin tags for a scenario.
type Provenance string

const (
	// ProductionCaptured is raw production data. Rejected in committed testdata.
	ProductionCaptured Provenance = "production-captured"

	// ProductionDerivedRedacted is production data after a redaction pass.
	// Safe for committed testdata.
	ProductionDerivedRedacted Provenance = "production-derived-redacted"

	// SyntheticAdversarial is synthetically crafted data for stress testing.
	// DecisiveEligible MUST be false for these scenarios.
	SyntheticAdversarial Provenance = "synthetic-adversarial"
)

// PredicateKind enumerates the interpretable check algorithms in the rubric
// vocabulary. The set is closed; callers must use one of these four values.
type PredicateKind string

const (
	// PredicateContains: strings.Contains(output, predicate.Arg).
	PredicateContains PredicateKind = "contains"
	// PredicateRegex: regexp.MustCompile(predicate.Arg).MatchString(output).
	PredicateRegex PredicateKind = "regex"
	// PredicateJSONPathEq: navigate predicate.Arg (JSON pointer) in parsed
	// output and compare to predicate.Value.
	PredicateJSONPathEq PredicateKind = "json_path_eq"
	// PredicateNonempty: len(strings.TrimSpace(output)) > 0.
	PredicateNonempty PredicateKind = "nonempty"
)

// Predicate is the interpretable machine-checkable claim within a Criterion.
//
// CUE mirror: #Criterion.predicate in schema.cue.
type Predicate struct {
	// Kind selects the check algorithm.
	Kind PredicateKind `json:"kind"`

	// Arg is the primary argument: substring for "contains", pattern for
	// "regex", JSON pointer path for "json_path_eq", empty for "nonempty".
	Arg string `json:"arg"`

	// Value is the expected value for "json_path_eq" comparisons; omitted for
	// other kinds.
	Value string `json:"value,omitempty"`
}

// Criterion is one weighted rubric dimension.
//
// CUE mirror: #Criterion in schema.cue.
type Criterion struct {
	// Name is a non-empty human-readable label. Required.
	Name string `json:"name"`

	// Weight is the relative importance. Must be > 0.
	Weight float64 `json:"weight"`

	// Predicate is the machine-checkable claim.
	Predicate Predicate `json:"predicate"`
}

// Rubric is an ordered set of weighted criteria evaluated by a RubricJudge.
//
// CUE mirror: #Rubric in schema.cue.
type Rubric struct {
	// Criteria is the ordered list of evaluation dimensions. May be empty.
	Criteria []Criterion `json:"criteria"`
}

// Scenario is one versioned evaluation scenario.
//
// CUE mirror: #Scenario in schema.cue.
type Scenario struct {
	// ID is a non-empty stable identifier for this scenario. Required.
	ID string `json:"id"`

	// Version is a positive integer tracking the scenario revision. Required.
	Version int `json:"version"`

	// Provenance records the origin of the scenario data.
	Provenance Provenance `json:"provenance"`

	// DecisiveEligible indicates whether records against this scenario may be
	// marked decisive. SyntheticAdversarial scenarios MUST have this false.
	DecisiveEligible bool `json:"decisive_eligible"`

	// Input is the scenario's input payload. Shape is caller-defined.
	Input json.RawMessage `json:"input"`

	// Expected is an optional gold-standard expected output.
	Expected json.RawMessage `json:"expected,omitempty"`

	// Rubric is an optional evaluation rubric.
	Rubric *Rubric `json:"rubric,omitempty"`
}

// FilterOptions controls which scenarios Load or Filter returns.
type FilterOptions struct {
	// Provenance, when non-empty, restricts to scenarios with this provenance.
	Provenance Provenance

	// IDs, when non-nil, restricts to scenarios whose ID appears in the set.
	IDs map[string]struct{}
}
