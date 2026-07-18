package scenariobank

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	evaljournal "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-journal"
	qaharness "github.com/geoffmilleraz/agentic-kit/foundations/quality/qa-harness"
)

// knownProvenances holds every permissible Provenance value.
var knownProvenances = map[Provenance]struct{}{
	ProductionCaptured:        {},
	ProductionDerivedRedacted: {},
	SyntheticAdversarial:      {},
}

// knownPredicateKinds holds every permissible PredicateKind value.
var knownPredicateKinds = map[PredicateKind]struct{}{
	PredicateContains:   {},
	PredicateRegex:      {},
	PredicateJSONPathEq: {},
	PredicateNonempty:   {},
}

// Validate checks all invariants on s. It is called by Load and may be
// called directly by callers constructing scenarios programmatically.
//
// Rules:
//   - ID must not be empty.
//   - Version must be >= 1.
//   - Provenance must be one of the three known values.
//   - SyntheticAdversarial scenarios MUST have DecisiveEligible=false.
//   - Input must be non-empty and valid JSON.
//   - production-captured scenarios are ALWAYS invalid in committed code
//     (the PII rule). This function rejects them — callers that deliberately
//     handle production data at runtime must bypass Validate, which is the
//     intentional friction.
func (s Scenario) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("scenariobank: Scenario.ID must not be empty")
	}
	if s.Version < 1 {
		return fmt.Errorf("scenariobank: Scenario.Version must be >= 1 (got %d)", s.Version)
	}
	if _, ok := knownProvenances[s.Provenance]; !ok {
		return fmt.Errorf("scenariobank: Provenance %q is not one of production-captured|production-derived-redacted|synthetic-adversarial", s.Provenance)
	}
	if s.Provenance == ProductionCaptured {
		return fmt.Errorf("scenariobank: production-captured scenarios are not permitted in committed testdata (PII hazard); apply a redaction pass and use production-derived-redacted")
	}
	if s.Provenance == SyntheticAdversarial && s.DecisiveEligible {
		return fmt.Errorf("scenariobank: synthetic-adversarial scenarios must have decisive_eligible=false")
	}
	if len(s.Input) == 0 {
		return fmt.Errorf("scenariobank: Scenario.Input must not be empty")
	}
	if !json.Valid(s.Input) {
		return fmt.Errorf("scenariobank: Scenario.Input is not valid JSON")
	}
	if s.Expected != nil && !json.Valid(s.Expected) {
		return fmt.Errorf("scenariobank: Scenario.Expected is not valid JSON")
	}
	if s.Rubric != nil {
		if err := validateRubric(*s.Rubric); err != nil {
			return err
		}
	}
	return nil
}

func validateRubric(r Rubric) error {
	for i, c := range r.Criteria {
		if c.Name == "" {
			return fmt.Errorf("scenariobank: Rubric.Criteria[%d].Name must not be empty", i)
		}
		if c.Weight <= 0 {
			return fmt.Errorf("scenariobank: Rubric.Criteria[%d].Weight must be > 0 (got %g)", i, c.Weight)
		}
		if _, ok := knownPredicateKinds[c.Predicate.Kind]; !ok {
			return fmt.Errorf("scenariobank: Rubric.Criteria[%d].Predicate.Kind %q is not one of contains|regex|json_path_eq|nonempty", i, c.Predicate.Kind)
		}
		// Pre-compile regex predicates at validation time so errors surface early.
		if c.Predicate.Kind == PredicateRegex {
			if _, err := regexp.Compile(c.Predicate.Arg); err != nil {
				return fmt.Errorf("scenariobank: Rubric.Criteria[%d].Predicate.Arg is not a valid regex: %w", i, err)
			}
		}
	}
	return nil
}

// Bank is an immutable, validated set of Scenarios. Construct with Load.
// Safe for concurrent use after construction.
type Bank struct {
	scenarios []Scenario
}

// Load decodes a JSON array of Scenario objects and validates each one. It
// returns an error if the JSON is malformed or if any scenario fails Validate.
//
// Wire format: a JSON array where each element is a Scenario object.
func Load(data []byte) (Bank, error) {
	var raw []Scenario
	if err := json.Unmarshal(data, &raw); err != nil {
		return Bank{}, fmt.Errorf("scenariobank: Load: JSON unmarshal: %w", err)
	}
	for i, s := range raw {
		if err := s.Validate(); err != nil {
			return Bank{}, fmt.Errorf("scenariobank: scenario[%d]: %w", i, err)
		}
	}
	out := make([]Scenario, len(raw))
	copy(out, raw)
	return Bank{scenarios: out}, nil
}

// All returns all scenarios in the bank, in load order. The returned slice is
// a defensive copy.
func (b Bank) All() []Scenario {
	out := make([]Scenario, len(b.scenarios))
	copy(out, b.scenarios)
	return out
}

// Filter returns scenarios matching opts, in load order. The returned slice is
// a defensive copy. Zero opts returns all scenarios.
func (b Bank) Filter(opts FilterOptions) []Scenario {
	var out []Scenario
	for _, s := range b.scenarios {
		if opts.Provenance != "" && s.Provenance != opts.Provenance {
			continue
		}
		if len(opts.IDs) > 0 {
			if _, ok := opts.IDs[s.ID]; !ok {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

// RubricJudgeFrom compiles a Rubric into a qaharness.Judge backed by a
// qaharness.RubricJudge. Each Criterion's Predicate is compiled into a
// deterministic RubricCriterion.Check function.
//
// Supported kinds:
//   - "contains": strings.Contains(item.Output, criterion.Predicate.Arg)
//   - "regex": compiled regexp matches item.Output
//   - "json_path_eq": navigate criterion.Predicate.Arg as a "/" prefix JSON
//     pointer path through item.Output (parsed as JSON object/array); compare
//     the string representation to criterion.Predicate.Value
//   - "nonempty": len(strings.TrimSpace(item.Output)) > 0
//
// The returned judge is a qaharness.RubricJudge — immutable, concurrency-safe.
func RubricJudgeFrom(r Rubric) (qaharness.Judge, error) {
	criteria := make([]qaharness.RubricCriterion, 0, len(r.Criteria))
	for i, c := range r.Criteria {
		check, err := compileCheck(c.Predicate)
		if err != nil {
			return nil, fmt.Errorf("scenariobank: RubricJudgeFrom: criterion[%d] %q: %w", i, c.Name, err)
		}
		name := c.Name
		criteria = append(criteria, qaharness.RubricCriterion{
			Name:   name,
			Weight: c.Weight,
			Check:  check,
		})
	}
	return qaharness.RubricJudge{Criteria: criteria}, nil
}

// compileCheck turns a Predicate into a Check function.
func compileCheck(p Predicate) (func(qaharness.Item) bool, error) {
	switch p.Kind {
	case PredicateContains:
		arg := p.Arg
		return func(item qaharness.Item) bool {
			return strings.Contains(item.Output, arg)
		}, nil

	case PredicateRegex:
		re, err := regexp.Compile(p.Arg)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", p.Arg, err)
		}
		return func(item qaharness.Item) bool {
			return re.MatchString(item.Output)
		}, nil

	case PredicateJSONPathEq:
		path := p.Arg
		want := p.Value
		return func(item qaharness.Item) bool {
			got, ok := jsonPathLookup(item.Output, path)
			if !ok {
				return false
			}
			return got == want
		}, nil

	case PredicateNonempty:
		return func(item qaharness.Item) bool {
			return len(strings.TrimSpace(item.Output)) > 0
		}, nil

	default:
		return nil, fmt.Errorf("unknown predicate kind %q", p.Kind)
	}
}

// jsonPathLookup navigates path (simple "/" delimited, "/" prefix) through the
// JSON in src and returns the string form of the leaf value. Only supports
// object key navigation — array indices and complex JMESPath are out of scope
// for the interpretable predicate vocabulary.
func jsonPathLookup(src, path string) (string, bool) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "", false
	}
	parts := strings.Split(path, "/")

	var node interface{}
	if err := json.Unmarshal([]byte(src), &node); err != nil {
		return "", false
	}

	for _, key := range parts {
		m, ok := node.(map[string]interface{})
		if !ok {
			return "", false
		}
		node, ok = m[key]
		if !ok {
			return "", false
		}
	}

	switch v := node.(type) {
	case string:
		return v, true
	case float64:
		return fmt.Sprintf("%g", v), true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

// HistoryFor is a pure projection over recs that returns all records with
// ScenarioID == scenarioID, in the order they appear in recs.
//
// It does not allocate when there are no matching records (returns nil).
// HistoryFor is a pure function; call it concurrently on the same slice freely.
func HistoryFor(scenarioID string, recs []evaljournal.Record) []evaljournal.Record {
	var out []evaljournal.Record
	for _, r := range recs {
		if r.ScenarioID == scenarioID {
			out = append(out, r)
		}
	}
	return out
}

// Compile-time check: RubricJudge satisfies qaharness.Judge.
var _ qaharness.Judge = qaharness.RubricJudge{}
