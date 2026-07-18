package dadinvestigator

import (
	"fmt"
	"sort"
	"strings"
)

// thresholds for confidence banding based on accumulated evidence weight.
const (
	weightThresholdLow    = 5
	weightThresholdMedium = 15
	weightThresholdHigh   = 30
)

// Investigator is the DAD (Decompose → Analyse → Decide) state machine.
// The zero value is usable; start by calling Step with an initial State.
// The machine is stateless — all investigation state is carried in State.
type Investigator struct{}

// Step advances the investigation by one phase transition. It consumes the
// supplied evidence items and returns the updated State and a Decision.
//
// The Decision is non-terminal for all non-terminal phases; callers should
// keep calling Step until Decision.Terminal is true. The machine converges
// deterministically: given the same (state, evidence) inputs the outputs are
// always identical.
//
// Phase transitions:
//
//	Decompose  → Analyse   (catalogues evidence into Pending)
//	Analyse    → Decide    (scores Pending items into Analysed)
//	Decide     → Decompose (if more evidence exists and confidence < High)
//	           → Found     (if confidence reaches High)
//	           → Exhausted (if no pending evidence and confidence < High)
//
// If state.Phase is already terminal (Found or Exhausted) Step returns state
// unchanged with the same terminal Decision.
func (inv Investigator) Step(state State, evidence []EvidenceItem) (State, Decision) {
	// Already terminal — return as-is.
	if state.Phase == PhaseFound || state.Phase == PhaseExhausted {
		return state, terminalDecision(state)
	}

	next := state // shallow copy; slices replaced below

	switch state.Phase {
	case PhaseDecompose, "": // treat zero-value as Decompose
		return inv.doDecompose(next, evidence)
	case PhaseAnalyse:
		return inv.doAnalyse(next, evidence)
	case PhaseDecide:
		return inv.doDecide(next, evidence)
	default:
		// Unknown phase: treat as Decompose.
		next.Phase = PhaseDecompose
		return inv.doDecompose(next, evidence)
	}
}

// doDecompose catalogues all supplied evidence into Pending, resets Analysed,
// and transitions to PhaseAnalyse.
func (inv Investigator) doDecompose(state State, evidence []EvidenceItem) (State, Decision) {
	// Merge new evidence with any remaining Pending items.
	merged := make([]EvidenceItem, 0, len(state.Pending)+len(evidence))
	merged = append(merged, state.Pending...)
	merged = append(merged, evidence...)

	// Deduplicate by ID, preserving first occurrence.
	seen := make(map[string]struct{}, len(merged))
	deduped := merged[:0]
	for _, item := range merged {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		deduped = append(deduped, item)
	}

	next := State{
		Phase:             PhaseAnalyse,
		Pending:           deduped,
		Analysed:          nil,
		AccumulatedWeight: state.AccumulatedWeight,
		Steps:             state.Steps,
	}
	return next, Decision{
		Finding:    fmt.Sprintf("decompose: catalogued %d evidence items", len(deduped)),
		Confidence: ConfidenceNone,
		Terminal:   false,
	}
}

// doAnalyse scores all Pending items and moves them to Analysed, then
// transitions to PhaseDecide.
func (inv Investigator) doAnalyse(state State, evidence []EvidenceItem) (State, Decision) {
	// Additional evidence arriving during analyse is queued for next decompose.
	pending := make([]EvidenceItem, len(state.Pending))
	copy(pending, state.Pending)
	pending = append(pending, evidence...)

	scored := make([]ScoredEvidence, 0, len(state.Pending))
	totalWeight := 0
	for _, item := range state.Pending {
		w := item.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
		scored = append(scored, ScoredEvidence{
			Item:  item,
			Score: clamp(w*10, 0, 100),
		})
	}

	// Sort by Score descending for stable ordering.
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Item.ID < scored[j].Item.ID
	})

	next := State{
		Phase:             PhaseDecide,
		Pending:           evidence, // only new evidence not yet decomposed
		Analysed:          scored,
		AccumulatedWeight: state.AccumulatedWeight + totalWeight,
		Steps:             state.Steps,
	}
	return next, Decision{
		Finding:    fmt.Sprintf("analyse: scored %d items, total weight %d", len(scored), next.AccumulatedWeight),
		Confidence: bandFromWeight(next.AccumulatedWeight),
		Terminal:   false,
	}
}

// doDecide synthesises a Decision from the Analysed items. If confidence
// reaches High the machine terminates in PhaseFound. If no further evidence
// is pending it terminates in PhaseExhausted. Otherwise it loops back to
// PhaseDecompose.
func (inv Investigator) doDecide(state State, evidence []EvidenceItem) (State, Decision) {
	conf := bandFromWeight(state.AccumulatedWeight)

	// Build a finding summary from top-scored analysed items.
	finding := buildFinding(state.Analysed, conf)

	// Merge any new evidence with whatever was pending.
	pending := make([]EvidenceItem, 0, len(state.Pending)+len(evidence))
	pending = append(pending, state.Pending...)
	pending = append(pending, evidence...)

	if conf == ConfidenceHigh {
		next := State{
			Phase:             PhaseFound,
			Pending:           pending,
			Analysed:          state.Analysed,
			AccumulatedWeight: state.AccumulatedWeight,
			Steps:             state.Steps + 1,
		}
		return next, Decision{
			Finding:    finding,
			Confidence: conf,
			Terminal:   true,
		}
	}

	if len(pending) == 0 {
		next := State{
			Phase:             PhaseExhausted,
			Pending:           nil,
			Analysed:          state.Analysed,
			AccumulatedWeight: state.AccumulatedWeight,
			Steps:             state.Steps + 1,
		}
		return next, Decision{
			Finding:    finding,
			Confidence: conf,
			Terminal:   true,
		}
	}

	// More evidence exists — loop back to Decompose.
	next := State{
		Phase:             PhaseDecompose,
		Pending:           pending,
		Analysed:          state.Analysed,
		AccumulatedWeight: state.AccumulatedWeight,
		Steps:             state.Steps + 1,
	}
	return next, Decision{
		Finding:    finding,
		Confidence: conf,
		Terminal:   false,
	}
}

// bandFromWeight converts an accumulated weight to a Confidence band.
func bandFromWeight(w int) Confidence {
	switch {
	case w >= weightThresholdHigh:
		return ConfidenceHigh
	case w >= weightThresholdMedium:
		return ConfidenceMedium
	case w >= weightThresholdLow:
		return ConfidenceLow
	default:
		return ConfidenceNone
	}
}

// buildFinding constructs a short finding string from top-scored items.
func buildFinding(analysed []ScoredEvidence, conf Confidence) string {
	if len(analysed) == 0 {
		return fmt.Sprintf("decide: no evidence analysed; confidence=%s", conf)
	}
	top := analysed
	if len(top) > 3 {
		top = top[:3]
	}
	ids := make([]string, len(top))
	for i, se := range top {
		ids[i] = se.Item.ID
	}
	return fmt.Sprintf("decide: confidence=%s; top evidence: %s", conf, strings.Join(ids, ", "))
}

// terminalDecision reconstructs a Decision for an already-terminal state.
func terminalDecision(state State) Decision {
	conf := bandFromWeight(state.AccumulatedWeight)
	return Decision{
		Finding:    buildFinding(state.Analysed, conf),
		Confidence: conf,
		Terminal:   true,
	}
}

// clamp returns v clamped to [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
