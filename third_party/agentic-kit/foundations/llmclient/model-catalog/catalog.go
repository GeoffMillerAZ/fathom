package modelcatalog

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Catalog is an immutable, validated set of ModelEntries. Construct one with
// Load; the zero value is not ready for use.
//
// A Catalog is immutable after Load and safe for concurrent use without
// additional locking.
type Catalog struct {
	models map[string]ModelEntry // keyed by id; populated by Load
}

// Load parses and validates JSON data conforming to #ModelCatalog in schema.cue.
// It returns an error if the JSON is malformed or if any entry violates the
// schema constraints (empty id, empty family, negative cost_tier, or an id
// field that does not match its map key).
//
// The returned Catalog is immutable; share it across goroutines without locking.
func Load(data []byte) (Catalog, error) {
	var raw catalogJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return Catalog{}, fmt.Errorf("modelcatalog: JSON unmarshal: %w", err)
	}

	models := make(map[string]ModelEntry, len(raw.Models))
	for key, entry := range raw.Models {
		if entry.ID == "" {
			return Catalog{}, fmt.Errorf("modelcatalog: entry %q has empty id", key)
		}
		if entry.Family == "" {
			return Catalog{}, fmt.Errorf("modelcatalog: entry %q has empty family", key)
		}
		if entry.CostTier < 0 {
			return Catalog{}, fmt.Errorf("modelcatalog: entry %q has negative cost_tier %d", key, entry.CostTier)
		}
		if entry.ID != key {
			return Catalog{}, fmt.Errorf("modelcatalog: entry key %q does not match id field %q", key, entry.ID)
		}
		models[key] = entry
	}

	return Catalog{models: models}, nil
}

// Entry returns the ModelEntry for the given id and ok=true, or the zero value
// and ok=false when no such id exists in the catalog.
func (c Catalog) Entry(id string) (ModelEntry, bool) {
	e, ok := c.models[id]
	return e, ok
}

// Validate performs semantic consistency checks over the catalog and returns any
// problems found. An empty slice means no problems were detected. Validate does
// not return an error; structural errors are rejected by Load. The checks are:
//
//   - dangling_lineage_ref: a lineage entry names a family that no model in the
//     catalog belongs to. (The lineage may intentionally reference an external
//     catalog; callers decide whether this is a problem.)
//   - all_deprecated_family: every member of a family is deprecated.
//
// Problems are returned in a deterministic order (sorted by Kind then Detail)
// so callers can compare or golden-test the output.
func (c Catalog) Validate() []Problem {
	// Collect the set of known families.
	knownFamilies := make(map[string]struct{})
	for _, e := range c.models {
		knownFamilies[e.Family] = struct{}{}
	}

	// Count non-deprecated members per family.
	activePerFamily := make(map[string]int)
	membersPerFamily := make(map[string]int)
	for _, e := range c.models {
		membersPerFamily[e.Family]++
		if !e.Deprecated {
			activePerFamily[e.Family]++
		}
	}

	var problems []Problem

	// Check lineage references.
	for _, e := range c.models {
		for _, lfamily := range e.Lineage {
			if _, ok := knownFamilies[lfamily]; !ok {
				problems = append(problems, Problem{
					Kind:   "dangling_lineage_ref",
					Detail: fmt.Sprintf("model %q lineage references unknown family %q", e.ID, lfamily),
				})
			}
		}
	}

	// Check all-deprecated families.
	for family, total := range membersPerFamily {
		if total > 0 && activePerFamily[family] == 0 {
			problems = append(problems, Problem{
				Kind:   "all_deprecated_family",
				Detail: fmt.Sprintf("family %q has %d member(s), all deprecated", family, total),
			})
		}
	}

	// Sort deterministically: kind first, then detail.
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].Kind != problems[j].Kind {
			return problems[i].Kind < problems[j].Kind
		}
		return problems[i].Detail < problems[j].Detail
	})
	return problems
}

// ValidateIndependence checks that judgeID and producerID are independent enough
// to serve as a judge/producer pair. It returns a Violation for each constraint
// that is not satisfied.
//
// Checks performed:
//   - unknown_model: either judgeID or producerID is absent from the catalog.
//     Unknown models are always a Violation; they are never silently passed.
//   - shared_family: judge and producer share the same family tag.
//   - shared_lineage: judge and producer share at least one entry in their
//     combined lineage sets (transitive: if judge.Lineage and producer.Lineage
//     overlap, or if one's family appears in the other's lineage).
//   - same_session: opts.SameSession is true (caller asserts shared session).
//   - shared_history: opts.SharedHistory is true (caller asserts shared history).
//
// Violations are returned in a deterministic order (sorted by Kind then Detail).
// The caller MUST treat any non-empty []Violation as a hard error before routing
// requests or accepting verdicts — violations are designed to be construction-time
// fatal.
func ValidateIndependence(judgeID, producerID string, c Catalog, opts IndependenceOpts) []Violation {
	var violations []Violation

	judgeEntry, judgeOK := c.models[judgeID]
	producerEntry, producerOK := c.models[producerID]

	if !judgeOK {
		violations = append(violations, Violation{
			Kind:   "unknown_model",
			Detail: fmt.Sprintf("judge model %q is not in the catalog", judgeID),
		})
	}
	if !producerOK {
		violations = append(violations, Violation{
			Kind:   "unknown_model",
			Detail: fmt.Sprintf("producer model %q is not in the catalog", producerID),
		})
	}

	// If either model is unknown we cannot perform family/lineage checks.
	if judgeOK && producerOK {
		if judgeEntry.Family == producerEntry.Family {
			violations = append(violations, Violation{
				Kind:   "shared_family",
				Detail: fmt.Sprintf("judge %q and producer %q share family %q", judgeID, producerID, judgeEntry.Family),
			})
		}

		if sharedLineage, overlap := lineageOverlap(judgeEntry, producerEntry); overlap {
			violations = append(violations, Violation{
				Kind:   "shared_lineage",
				Detail: fmt.Sprintf("judge %q and producer %q share lineage family %q", judgeID, producerID, sharedLineage),
			})
		}
	}

	if opts.SameSession {
		violations = append(violations, Violation{
			Kind:   "same_session",
			Detail: fmt.Sprintf("judge %q and producer %q share the same session context", judgeID, producerID),
		})
	}
	if opts.SharedHistory {
		violations = append(violations, Violation{
			Kind:   "shared_history",
			Detail: fmt.Sprintf("judge %q has access to producer %q generation history", judgeID, producerID),
		})
	}

	// Sort deterministically: kind first, then detail.
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Kind != violations[j].Kind {
			return violations[i].Kind < violations[j].Kind
		}
		return violations[i].Detail < violations[j].Detail
	})
	return violations
}

// lineageOverlap returns the first shared lineage family and true when the
// judge and producer share any lineage entry. The check is symmetric and
// includes cross-family-to-lineage overlap:
//
//   - judge.Lineage ∩ producer.Lineage
//   - judge.Family ∈ producer.Lineage
//   - producer.Family ∈ judge.Lineage
//
// This covers the transitive case: if model A is distilled from family X,
// and model B is also from family X (directly or via lineage), they are
// considered lineage-related even if their immediate Family tags differ.
func lineageOverlap(judge, producer ModelEntry) (shared string, ok bool) {
	// Build producer lineage set (including producer.Family for cross checks).
	producerSet := make(map[string]struct{}, len(producer.Lineage)+1)
	producerSet[producer.Family] = struct{}{}
	for _, f := range producer.Lineage {
		producerSet[f] = struct{}{}
	}

	// Check judge.Family against producer lineage.
	if _, found := producerSet[judge.Family]; found {
		// Same-family is already caught by the shared_family check; only
		// report lineage when the family match is through lineage, not direct.
		if judge.Family != producer.Family {
			return judge.Family, true
		}
	}

	// Check each judge lineage entry against the producer set.
	for _, lf := range judge.Lineage {
		if _, found := producerSet[lf]; found {
			return lf, true
		}
	}

	return "", false
}
