package improvementflywheel

import (
	"fmt"
	"sort"
)

// maxExamples is the maximum number of Detail strings kept per observation.
const maxExamples = 5

// observationKey returns the composite map key for a (Type, Key) pair.
func observationKey(t PatternType, key string) string {
	return fmt.Sprintf("%s\x00%s", t, key)
}

// Flywheel accumulates signals and produces improvement proposals.
// A zero-value Flywheel is ready for use.
type Flywheel struct {
	observations map[string]*PatternObservation
}

// Observe records a Signal and updates the internal PatternObservation for its
// (Type, Key) group. Observe is not safe for concurrent use.
func (f *Flywheel) Observe(sig Signal) {
	if f.observations == nil {
		f.observations = make(map[string]*PatternObservation)
	}
	k := observationKey(sig.Type, sig.Key)
	obs, exists := f.observations[k]
	if !exists {
		obs = &PatternObservation{
			Type: sig.Type,
			Key:  sig.Key,
		}
		f.observations[k] = obs
	}
	obs.Frequency++

	// Accumulate workflow IDs (deduplicated).
	if sig.WorkflowID != "" && !containsString(obs.WorkflowIDs, sig.WorkflowID) {
		obs.WorkflowIDs = append(obs.WorkflowIDs, sig.WorkflowID)
		sort.Strings(obs.WorkflowIDs)
	}

	// Keep the first maxExamples non-empty Detail strings.
	if sig.Detail != "" && len(obs.Examples) < maxExamples {
		obs.Examples = append(obs.Examples, sig.Detail)
	}
}

// Proposals returns a deduplicated, deterministically sorted slice of
// Proposals derived from all current observations.
//
// Sort order:
//  1. Descending Frequency (most-seen first).
//  2. Ascending PatternType string (tie-break).
//  3. Ascending Key string (final tie-break).
func (f *Flywheel) Proposals() []Proposal {
	if len(f.observations) == 0 {
		return nil
	}

	proposals := make([]Proposal, 0, len(f.observations))
	for _, obs := range f.observations {
		proposals = append(proposals, proposalFromObservation(obs))
	}

	sort.Slice(proposals, func(i, j int) bool {
		pi, pj := proposals[i], proposals[j]
		if pi.Frequency != pj.Frequency {
			return pi.Frequency > pj.Frequency // descending
		}
		if pi.PatternType != pj.PatternType {
			return string(pi.PatternType) < string(pj.PatternType)
		}
		// Extract key from ObservationKey for final tie-break.
		return proposals[i].ObservationKey < proposals[j].ObservationKey
	})

	return proposals
}

// Observations returns a copy of the current observations, sorted
// deterministically (ascending Type, ascending Key).
func (f *Flywheel) Observations() []PatternObservation {
	if len(f.observations) == 0 {
		return nil
	}
	out := make([]PatternObservation, 0, len(f.observations))
	for _, obs := range f.observations {
		cp := *obs
		cp.WorkflowIDs = copyStrings(obs.WorkflowIDs)
		cp.Examples = copyStrings(obs.Examples)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return string(out[i].Type) < string(out[j].Type)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// proposalFromObservation derives a Proposal from a PatternObservation.
func proposalFromObservation(obs *PatternObservation) Proposal {
	id := fmt.Sprintf("proposal-%s-%s", obs.Type, obs.Key)
	key := fmt.Sprintf("%s/%s", obs.Type, obs.Key)

	target := changeTargetFor(obs.Type)
	summary := fmt.Sprintf("Pattern %q (%s) observed %d time(s) — consider %s update",
		obs.Key, obs.Type, obs.Frequency, target)

	return Proposal{
		ID:                    id,
		ObservationKey:        key,
		PatternType:           obs.Type,
		ChangeTarget:          target,
		Summary:               summary,
		Frequency:             obs.Frequency,
		SupportingWorkflowIDs: copyStrings(obs.WorkflowIDs),
	}
}

// changeTargetFor maps a PatternType to a ChangeTarget.
func changeTargetFor(pt PatternType) ChangeTarget {
	switch pt {
	case PatternFalsePositive:
		return TargetCatalogEntry
	case PatternMissedClaim:
		return TargetPromptSection
	case PatternRepeatedCorrection:
		return TargetProsePolicy
	case PatternOperatorFriction:
		return TargetFoundationBehavior
	default:
		return TargetCUEConstraint
	}
}

// containsString reports whether s appears in slice.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// copyStrings returns a new slice with the same strings as src.
func copyStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
