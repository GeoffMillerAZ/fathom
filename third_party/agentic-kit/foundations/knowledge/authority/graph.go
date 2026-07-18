package authority

import (
	"fmt"
	"sort"
	"time"
)

// Graph is the in-memory authority graph. Authorities are nodes; Treatments
// are directed edges. The graph tracks citation/treatment relationships and
// can derive the effective legal status of any authority by walking inbound
// treatments.
//
// Graph is NOT safe for concurrent use. Callers that share a Graph across
// goroutines must serialise access externally.
//
// The clock function injected at construction (see NewGraph, NewGraphWithClock)
// is reserved for future time-sensitive operations such as "current" effective
// status queries. InForce queries use their own explicit at parameter rather
// than the injected clock, to maintain full determinism in tests.
type Graph struct {
	authorities map[string]Authority // keyed by Authority.ID
	treatments  []Treatment          // all recorded treatments
	clock       func() time.Time
}

// NewGraph returns an empty, ready-to-use Graph with the supplied clock.
// Inject a deterministic clock in tests (e.g. func() time.Time { return epoch }).
// Pass nil to use time.Now().UTC().
func NewGraph(clock func() time.Time) *Graph {
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Graph{
		authorities: make(map[string]Authority),
		clock:       clock,
	}
}

// Add inserts or replaces an Authority in the graph. The authority must pass
// Validate(); an error is returned without modifying the graph if it does not.
func (g *Graph) Add(a Authority) error {
	if err := a.Validate(); err != nil {
		return fmt.Errorf("authority: Graph.Add: %w", err)
	}
	g.authorities[a.ID] = a
	return nil
}

// Treat records a directed treatment edge. Both FromID and ToID need not
// pre-exist in the graph when Treat is called; the edge is stored regardless.
// The Treatment must pass Validate(); an error is returned if it does not.
func (g *Graph) Treat(t Treatment) error {
	if err := t.Validate(); err != nil {
		return fmt.Errorf("authority: Graph.Treat: %w", err)
	}
	g.treatments = append(g.treatments, t)
	return nil
}

// InForce reports whether the authority identified by id is in force at the
// given instant at. An authority is in force when:
//   - id exists in the graph, AND
//   - at falls within the half-open interval [EffectiveFrom, EffectiveTo].
//     A zero EffectiveFrom means "from the beginning of time".
//     A zero EffectiveTo means "still in force today (open end)".
//
// Returns false for unknown ids without panicking.
func (g *Graph) InForce(id string, at time.Time) bool {
	a, ok := g.authorities[id]
	if !ok {
		return false
	}
	return inWindow(a.EffectiveFrom, a.EffectiveTo, at)
}

// inWindow reports whether t falls within the effective window defined by
// [from, to]. Either bound may be zero for an open boundary.
func inWindow(from, to, t time.Time) bool {
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && t.After(to) {
		return false
	}
	return true
}

// statusSeverity returns a numeric severity for a Status so that downgrades can
// be computed without a bespoke if-chain. Higher numbers are worse:
//
//	overruled(4) > superseded(3) > questioned(2) > good_law(1) > unknown(0)
//
// StatusUnknown sits below good_law because "currency not verified" is a less
// specific negative signal than "actively questioned by a binding authority".
func statusSeverity(s Status) int {
	switch s {
	case StatusOverruled:
		return 4
	case StatusSuperseded:
		return 3
	case StatusQuestioned:
		return 2
	case StatusGoodLaw:
		return 1
	default: // StatusUnknown and any future unknown variant
		return 0
	}
}

// EffectiveStatus derives the current legal status of the authority identified
// by id by starting from its stored Status and walking all inbound treatments.
//
// Severity ordering (for downgrade-only logic):
//
//	overruled(4) > superseded(3) > questioned(2) > good_law(1) > unknown(0)
//
// Status rules:
//   - Only binding sources can change status; persuasive / non-authoritative
//     treatments are ignored.
//   - A binding authority that overrules id → StatusOverruled. This is terminal
//     (no further downgrade is possible), but all binding overrulers are still
//     walked so that every overruling authority appears in the reasons trace.
//   - A binding authority that questions or criticizes id:
//   - If the current derived severity < 2 (below questioned): downgrade to
//     StatusQuestioned.
//   - In all cases (including already-questioned, superseded, overruled):
//     record a reason so the trace is complete — no silent truncation.
//   - KindFollows, KindDistinguishes, KindReaffirms: no status change. Reaffirms
//     from a binding source does not upgrade a negative status.
//
// The second return value is a sorted, deterministic list of human-readable
// reasons explaining each significant treatment. Includes all binding overrulers
// and all binding questions/criticizes treatments regardless of whether they
// caused a severity change.
//
// Returns (StatusUnknown, nil) for an unknown id.
func (g *Graph) EffectiveStatus(id string) (Status, []string) {
	a, ok := g.authorities[id]
	if !ok {
		return StatusUnknown, nil
	}

	derived := a.Status
	var reasons []string

	// Collect all inbound treatments (where ToID == id), sorted by (FromID, Kind)
	// for determinism so the output is stable across multiple calls.
	inbound := g.inboundTreatments(id)

	for _, tr := range inbound {
		// Determine the weight of the treating authority. If the treating
		// authority is not in the graph we cannot establish binding weight —
		// treat it as non-authoritative (no effect on status).
		from, fromExists := g.authorities[tr.FromID]
		if !fromExists {
			continue
		}
		if from.Weight != WeightBinding {
			continue // only binding sources can change status
		}

		switch tr.Kind {
		case KindOverrules:
			// Binding overrule is the strongest possible downgrade — it sets the
			// derived status to overruled regardless of where it started.
			// We do NOT early-return here; we continue walking to collect every
			// overruling authority in the reasons trace (no silent truncation).
			derived = StatusOverruled
			reasons = append(reasons, fmt.Sprintf(
				"overruled by binding authority %q (%s)", tr.FromID, from.Citation,
			))

		case KindQuestions, KindCriticizes:
			// Always record the treatment in the reasons trace — a binding
			// authority expressing doubt is a legally significant fact regardless
			// of whether it changes the derived severity.
			verb := "questioned"
			if tr.Kind == KindCriticizes {
				verb = "criticized"
			}
			reasons = append(reasons, fmt.Sprintf(
				"%s by binding authority %q (%s)", verb, tr.FromID, from.Citation,
			))
			// Downgrade toward questioned only if the current derived severity is
			// strictly below questioned (severity < 2). Statuses already at or
			// above questioned (superseded, overruled) are not changed — we only
			// ever move toward worse, never toward better.
			if statusSeverity(derived) < statusSeverity(StatusQuestioned) {
				derived = StatusQuestioned
			}

		// KindFollows, KindDistinguishes, KindReaffirms: these do not change
		// status — they acknowledge the authority without downgrading it.
		// Reaffirms from a binding source does not restore a negative status.
		default:
			// no status change, no reason recorded
		}
	}

	// Sort reasons for determinism — inbound treatments are walked in a
	// deterministic order but reasons from KindOverrules and KindQuestions /
	// KindCriticizes may interleave, so a final sort guarantees stable output.
	sort.Strings(reasons)

	return derived, reasons
}

// inboundTreatments returns all Treatment edges where ToID == id, sorted
// deterministically by (FromID, Kind) to ensure stable iteration order.
func (g *Graph) inboundTreatments(id string) []Treatment {
	var result []Treatment
	for _, tr := range g.treatments {
		if tr.ToID == id {
			result = append(result, tr)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].FromID != result[j].FromID {
			return result[i].FromID < result[j].FromID
		}
		return result[i].Kind < result[j].Kind
	})
	return result
}

// Authorities returns a defensive copy of all Authority nodes in the graph,
// sorted by ID for determinism. Mutations to the returned slice do not affect
// the graph's internal state.
func (g *Graph) Authorities() []Authority {
	out := make([]Authority, 0, len(g.authorities))
	for _, a := range g.authorities {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// Treatments returns a defensive copy of all Treatment edges in the graph, in
// insertion order. Mutations to the returned slice do not affect the graph's
// internal state.
func (g *Graph) Treatments() []Treatment {
	out := make([]Treatment, len(g.treatments))
	copy(out, g.treatments)
	return out
}
