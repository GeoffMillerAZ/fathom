package casefile

import (
	"fmt"
	"sort"

	assertion "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/assertion"
	authenticity "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/authenticity"
	conflict "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/conflict"
	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	narrativeshift "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/narrative-shift"
)

// Build assembles and returns the CaseFile for the given propositionID from
// the caller-supplied Inputs. The same inputs always produce byte-identical
// output: all slices are sorted deterministically and no time or randomness is
// consulted.
//
// propositionID must be non-empty.
//
// Build is a pure function: it does not mutate any input and has no side
// effects. It is safe to call concurrently from multiple goroutines.
func Build(propositionID string, in Inputs) (CaseFile, error) {
	if propositionID == "" {
		return CaseFile{}, fmt.Errorf("casefile: propositionID must not be empty")
	}

	// ── 1. Partition assertions for this proposition ──────────────────────────
	var supporting, contradicting, retracting, qualifying []GradedAssertion
	for _, a := range in.Assertions {
		if a.PropositionID != propositionID {
			continue
		}
		ga := gradedAssertionFrom(a)
		switch {
		case a.Stance == assertion.StanceAffirms:
			supporting = append(supporting, ga)
		case a.Stance == assertion.StanceDenies:
			contradicting = append(contradicting, ga)
		case a.Stance.IsRetraction():
			retracting = append(retracting, ga)
		case a.Stance == assertion.StanceQualifies:
			qualifying = append(qualifying, ga)
			// does_not_recall / declines — not counted in support weights, also
			// not shown in the four display buckets; silently skipped.
		}
	}

	// Sort each bucket: TierRank desc, then AssertionID asc for determinism.
	sortGraded(supporting)
	sortGraded(contradicting)
	sortGraded(retracting)
	sortGraded(qualifying)

	// ── 2. Filter conflicts to this proposition ───────────────────────────────
	var conflicts []conflict.Conflict
	for _, c := range in.Conflicts {
		if c.Subject == propositionID {
			conflicts = append(conflicts, c)
		}
	}
	// Already sorted by (Kind, Subject, Field, A.ID, B.ID) from conflict.Detect,
	// but re-sort to guarantee it even when the caller passed an arbitrary list.
	sort.Slice(conflicts, func(i, j int) bool {
		ri, rj := conflicts[i], conflicts[j]
		if ri.Kind != rj.Kind {
			return ri.Kind < rj.Kind
		}
		if ri.A.ID != rj.A.ID {
			return ri.A.ID < rj.A.ID
		}
		return ri.B.ID < rj.B.ID
	})

	// ── 3. Build authenticity digests ─────────────────────────────────────────
	// Collect the record IDs cited by assertions on this proposition.
	citedRecordIDs := make(map[string]struct{})
	for _, a := range in.Assertions {
		if a.PropositionID == propositionID && a.StatedIn != "" {
			citedRecordIDs[a.StatedIn] = struct{}{}
		}
	}
	var digests []AuthenticityDigest
	for _, asm := range in.Assessments {
		if _, cited := citedRecordIDs[asm.RecordID]; !cited {
			continue
		}
		digests = append(digests, authenticityDigestFrom(asm))
	}
	sort.Slice(digests, func(i, j int) bool {
		return digests[i].RecordID < digests[j].RecordID
	})

	// ── 4. Filter narrative shifts to this proposition ────────────────────────
	var shifts []ShiftDigest
	for _, ns := range in.Shifts {
		if ns.PropositionID == propositionID {
			shifts = append(shifts, shiftDigestFrom(ns))
		}
	}
	sort.Slice(shifts, func(i, j int) bool {
		if shifts[i].Declarant != shifts[j].Declarant {
			return shifts[i].Declarant < shifts[j].Declarant
		}
		return shifts[i].FromID < shifts[j].FromID
	})

	// ── 5. Compute support summary ────────────────────────────────────────────
	summary := computeSupportSummary(supporting, contradicting, retracting, qualifying)

	return CaseFile{
		PropositionID:       propositionID,
		Supporting:          nilIfEmpty(supporting),
		Contradicting:       nilIfEmpty(contradicting),
		Retracting:          nilIfEmpty(retracting),
		Qualifying:          nilIfEmpty(qualifying),
		Conflicts:           nilIfEmptyConflict(conflicts),
		AuthenticityDigests: nilIfEmptyDigest(digests),
		ShiftHistory:        nilIfEmptyShift(shifts),
		SupportSummary:      summary,
	}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// gradedAssertionFrom extracts the fields needed for a GradedAssertion from
// an Assertion, resolving the effective tier rank.
func gradedAssertionFrom(a assertion.Assertion) GradedAssertion {
	var tier evidencegrade.Tier
	var rank int
	if a.Grade != nil {
		tier = a.Grade.Tier
		rank = tier.Rank()
	} else if a.Via != "" {
		tier = a.Via
		rank = tier.Rank()
	} else {
		tier = "ungraded"
		rank = 0
	}
	return GradedAssertion{
		AssertionID: a.ID,
		Declarant:   a.Declarant,
		Stance:      a.Stance,
		Content:     a.Content,
		Tier:        tier,
		TierRank:    rank,
	}
}

// sortGraded sorts a GradedAssertion slice by (TierRank desc, AssertionID asc).
func sortGraded(gs []GradedAssertion) {
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].TierRank != gs[j].TierRank {
			return gs[i].TierRank > gs[j].TierRank // desc
		}
		return gs[i].AssertionID < gs[j].AssertionID // asc
	})
}

// authenticityDigestFrom condenses an authenticity.Assessment to AuthenticityDigest.
func authenticityDigestFrom(asm authenticity.Assessment) AuthenticityDigest {
	return AuthenticityDigest{
		RecordID:     asm.RecordID,
		Overall:      asm.Overall,
		Confidence:   asm.Confidence,
		Conservative: asm.Conservative,
	}
}

// shiftDigestFrom condenses a narrativeshift.NarrativeShift to ShiftDigest.
func shiftDigestFrom(ns narrativeshift.NarrativeShift) ShiftDigest {
	return ShiftDigest{
		Declarant:     ns.Declarant,
		PropositionID: ns.PropositionID,
		FromID:        ns.From.ID,
		ToID:          ns.To.ID,
		Kind:          ns.Kind,
		GapDays:       ns.GapDays,
	}
}

// computeSupportSummary derives the SupportSummary from the four stance buckets.
// See package-level doc for the exact formula.
func computeSupportSummary(
	supporting, contradicting, retracting, qualifying []GradedAssertion,
) SupportSummary {
	affCount := len(supporting)
	denCount := len(contradicting)
	retCount := len(retracting)
	qualCount := len(qualifying)

	maxPossible := (affCount + denCount) * 5
	if maxPossible == 0 {
		return SupportSummary{
			SupportScore:    0,
			AgainstScore:    0,
			Tier:            SupportTierUnresolved,
			AffirmingCount:  affCount,
			DenyingCount:    denCount,
			RetractingCount: retCount,
			QualifyingCount: qualCount,
		}
	}

	var suppWeight, contraWeight int
	for _, ga := range supporting {
		suppWeight += ga.TierRank
	}
	for _, ga := range contradicting {
		contraWeight += ga.TierRank
	}

	supportScore := float64(suppWeight) / float64(maxPossible)
	againstScore := float64(contraWeight) / float64(maxPossible)
	net := supportScore - againstScore

	var tier SupportTier
	switch {
	case net > 0.60:
		tier = SupportTierStrongSupport
	case net > 0.25:
		tier = SupportTierModerateSupport
	case net > -0.25:
		tier = SupportTierContested
	case net > -0.60:
		tier = SupportTierModerateAgainst
	default:
		tier = SupportTierStrongAgainst
	}

	return SupportSummary{
		SupportScore:    supportScore,
		AgainstScore:    againstScore,
		Tier:            tier,
		AffirmingCount:  affCount,
		DenyingCount:    denCount,
		RetractingCount: retCount,
		QualifyingCount: qualCount,
	}
}

// nilIfEmpty returns nil when s is empty, preserving the property that
// empty slices marshal to null (not []) in JSON, keeping output deterministic.
func nilIfEmpty(s []GradedAssertion) []GradedAssertion {
	if len(s) == 0 {
		return nil
	}
	return s
}

func nilIfEmptyConflict(s []conflict.Conflict) []conflict.Conflict {
	if len(s) == 0 {
		return nil
	}
	return s
}

func nilIfEmptyDigest(s []AuthenticityDigest) []AuthenticityDigest {
	if len(s) == 0 {
		return nil
	}
	return s
}

func nilIfEmptyShift(s []ShiftDigest) []ShiftDigest {
	if len(s) == 0 {
		return nil
	}
	return s
}
