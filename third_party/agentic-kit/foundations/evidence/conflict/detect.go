package conflict

import (
	"sort"

	assertion "github.com/geoffmilleraz/agentic-kit/foundations/evidence/assertion"
	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
)

// Detect performs pure, deterministic conflict detection over the provided
// assertions and spans. It returns nil when both slices are empty or when no
// conflicts are found. No caps are applied — all detected conflicts are returned.
//
// Output is sorted by (Kind, Subject, Field, A.ID, B.ID) for determinism.
// Each unordered pair is reported exactly once.
func Detect(assertions []assertion.Assertion, spans []evidencespan.Span) []Conflict {
	if len(assertions) == 0 && len(spans) == 0 {
		return nil
	}

	seen := make(map[pairKey]struct{})
	var result []Conflict

	emit := func(c Conflict) {
		k := pairKey{c.Kind, c.Subject, c.Field, c.A.ID, c.B.ID}
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		result = append(result, c)
	}

	detectAssertionConflicts(assertions, emit)
	detectClaimVsEvidence(assertions, spans, emit)

	if len(result) == 0 {
		return nil
	}

	// Sort output deterministically: (Kind, Subject, Field, A.ID, B.ID).
	sort.Slice(result, func(i, j int) bool {
		ri, rj := result[i], result[j]
		if ri.Kind != rj.Kind {
			return ri.Kind < rj.Kind
		}
		if ri.Subject != rj.Subject {
			return ri.Subject < rj.Subject
		}
		if ri.Field != rj.Field {
			return ri.Field < rj.Field
		}
		if ri.A.ID != rj.A.ID {
			return ri.A.ID < rj.A.ID
		}
		return ri.B.ID < rj.B.ID
	})

	return result
}

// detectAssertionConflicts handles self_inconsistency, recantation,
// claim_vs_claim, and qualification passes over the grouped assertion set.
func detectAssertionConflicts(assertions []assertion.Assertion, emit func(Conflict)) {
	// Index assertions by PropositionID for O(n) grouping.
	byProp := make(map[string][]assertion.Assertion)
	for _, a := range assertions {
		byProp[a.PropositionID] = append(byProp[a.PropositionID], a)
	}

	// Sort proposition IDs for deterministic outer iteration order.
	propIDs := make([]string, 0, len(byProp))
	for pid := range byProp {
		propIDs = append(propIDs, pid)
	}
	sort.Strings(propIDs)

	for _, pid := range propIDs {
		group := byProp[pid]

		// Sort within group by ID for deterministic pairwise scanning.
		sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })

		detectQualifications(pid, group, emit)
		detectPairwiseAssertionConflicts(pid, group, emit)
	}
}

// detectQualifications emits KindQualification for any qualifies-stance
// assertion co-present with an affirms/denies assertion on the same proposition.
func detectQualifications(pid string, group []assertion.Assertion, emit func(Conflict)) {
	hasAffirmOrDeny := false
	var qualAssertions []assertion.Assertion
	for _, a := range group {
		if a.Stance == assertion.StanceAffirms || a.Stance == assertion.StanceDenies {
			hasAffirmOrDeny = true
		}
		if a.Stance == assertion.StanceQualifies {
			qualAssertions = append(qualAssertions, a)
		}
	}
	if !hasAffirmOrDeny {
		return
	}
	for _, qa := range qualAssertions {
		// Pair with the first (lowest-ID) affirm/deny for determinism.
		for _, a := range group {
			if a.Stance != assertion.StanceAffirms && a.Stance != assertion.StanceDenies {
				continue
			}
			refA, refB := orderRefs(
				Ref{Kind: RefKindAssertion, ID: qa.ID},
				Ref{Kind: RefKindAssertion, ID: a.ID},
			)
			sev := severity(effectiveTierRank(qa), effectiveTierRank(a))
			emit(Conflict{
				Kind:     KindQualification,
				Subject:  pid,
				A:        refA,
				B:        refB,
				Severity: sev,
				Reason:   "declarant " + qa.Declarant + " qualifies a proposition that also has an affirming or denying assertion",
			})
			break // one pair per qualifier
		}
	}
}

// detectPairwiseAssertionConflicts scans all pairs within a proposition group
// for self_inconsistency, recantation, and claim_vs_claim.
func detectPairwiseAssertionConflicts(pid string, group []assertion.Assertion, emit func(Conflict)) {
	for i := 0; i < len(group); i++ {
		for j := i + 1; j < len(group); j++ {
			a, b := group[i], group[j]
			refA := Ref{Kind: RefKindAssertion, ID: a.ID}
			refB := Ref{Kind: RefKindAssertion, ID: b.ID}
			// IDs are already sorted a.ID <= b.ID from the outer sort.

			sev := severity(effectiveTierRank(a), effectiveTierRank(b))

			if a.Declarant == b.Declarant {
				detectSameDeclarantPair(pid, a, b, refA, refB, sev, emit)
			} else {
				detectDifferentDeclarantPair(pid, a, b, refA, refB, sev, emit)
			}
		}
	}
}

// detectSameDeclarantPair emits recantation or self_inconsistency for a pair
// of assertions from the same declarant.
func detectSameDeclarantPair(
	pid string,
	a, b assertion.Assertion,
	refA, refB Ref,
	sev float64,
	emit func(Conflict),
) {
	if a.Stance.IsRetraction() || b.Stance.IsRetraction() {
		// One side is a retraction — classify as recantation.
		// StatedAt ordering determines which side retracted; we report
		// the pair regardless since StatedAt may be zero in lenient mode.
		retractor := a
		if b.Stance.IsRetraction() {
			retractor = b
		}
		emit(Conflict{
			Kind:     KindRecantation,
			Subject:  pid,
			A:        refA,
			B:        refB,
			Severity: sev,
			Reason:   "declarant " + retractor.Declarant + " retracted a prior assertion on proposition " + pid,
		})
		return
	}
	if a.Content != b.Content {
		// Neither is a retraction; different content → self_inconsistency.
		emit(Conflict{
			Kind:     KindSelfInconsistency,
			Subject:  pid,
			A:        refA,
			B:        refB,
			Severity: sev,
			Reason:   "declarant " + a.Declarant + " made incompatible assertions on proposition " + pid,
		})
	}
	// Identical content → no conflict.
}

// detectDifferentDeclarantPair emits claim_vs_claim for a pair of assertions
// from different declarants when the content differs.
// Qualifying assertions are excluded: they are already captured as
// KindQualification and represent hedging rather than direct contradiction.
func detectDifferentDeclarantPair(
	pid string,
	a, b assertion.Assertion,
	refA, refB Ref,
	sev float64,
	emit func(Conflict),
) {
	if a.Stance.IsRetraction() || b.Stance.IsRetraction() {
		return // retractions are not direct cross-declarant contradictions
	}
	if a.Stance == assertion.StanceQualifies || b.Stance == assertion.StanceQualifies {
		return // qualifications are handled separately
	}
	if a.Content == b.Content {
		return // agreement — no conflict
	}
	emit(Conflict{
		Kind:     KindClaimVsClaim,
		Subject:  pid,
		A:        refA,
		B:        refB,
		Severity: sev,
		Reason:   "declarants " + a.Declarant + " and " + b.Declarant + " make incompatible assertions on proposition " + pid,
	})
}

// DetectWithPropositions is the proposition-aware variant of [Detect].
//
// It performs all assertion-vs-assertion conflict passes (identical to Detect),
// then replaces the naive PropositionID==EntityID claim-vs-evidence join with a
// structurally correct three-way join:
//
//	Assertion.PropositionID == Proposition.ID
//	Proposition.Subject     == Span.EntityID
//	(Span.Field is preserved on the emitted Conflict for field-level context)
//
// This join is correct when the two sets of IDs are in different namespaces
// (e.g. PropositionIDs are "prop-{hex}" digests; Span.EntityIDs are record IDs).
// The legacy [Detect] join (PropositionID == EntityID) silently finds nothing in
// that situation; DetectWithPropositions surfaces the zero-join as
// [Coverage.Degenerate] == true.
//
// propositions may be nil or empty — the assertion-vs-assertion passes still run
// and Coverage will reflect the empty proposition set.
//
// Output Conflicts are sorted by (Kind, Subject, Field, A.ID, B.ID).
// Each unordered pair is reported exactly once.
//
// The returned [DetectResult].Coverage is always populated; callers MUST check
// Coverage.Degenerate() and treat it as an operational alarm.
func DetectWithPropositions(
	assertions []assertion.Assertion,
	propositions []assertion.Proposition,
	spans []evidencespan.Span,
) DetectResult {
	seen := make(map[pairKey]struct{})
	var conflicts []Conflict

	emit := func(c Conflict) {
		k := pairKey{c.Kind, c.Subject, c.Field, c.A.ID, c.B.ID}
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		conflicts = append(conflicts, c)
	}

	detectAssertionConflicts(assertions, emit)
	cov := detectClaimVsEvidenceViaPropositions(assertions, propositions, spans, emit)

	if len(conflicts) > 0 {
		sort.Slice(conflicts, func(i, j int) bool {
			ri, rj := conflicts[i], conflicts[j]
			if ri.Kind != rj.Kind {
				return ri.Kind < rj.Kind
			}
			if ri.Subject != rj.Subject {
				return ri.Subject < rj.Subject
			}
			if ri.Field != rj.Field {
				return ri.Field < rj.Field
			}
			if ri.A.ID != rj.A.ID {
				return ri.A.ID < rj.A.ID
			}
			return ri.B.ID < rj.B.ID
		})
	}

	cov.AssertionCount = len(assertions)
	cov.PropositionCount = len(propositions)
	cov.SpanCount = len(spans)

	return DetectResult{
		Conflicts: conflicts,
		Coverage:  cov,
	}
}

// detectClaimVsEvidenceViaPropositions performs the proposition-bridged join for
// DetectWithPropositions. It returns a Coverage with the matching statistics
// (AssertionCount/PropositionCount/SpanCount are filled in by the caller).
func detectClaimVsEvidenceViaPropositions(
	assertions []assertion.Assertion,
	propositions []assertion.Proposition,
	spans []evidencespan.Span,
	emit func(Conflict),
) Coverage {
	var cov Coverage

	if len(propositions) == 0 || len(spans) == 0 {
		return cov
	}

	// Build proposition index: propID → Proposition.Subject.
	propSubject := make(map[string]string, len(propositions))
	for _, p := range propositions {
		propSubject[p.ID] = p.Subject
	}

	// Build span index: EntityID → sorted []Span.
	spanIdx := make(map[string][]evidencespan.Span)
	for _, s := range spans {
		spanIdx[s.EntityID] = append(spanIdx[s.EntityID], s)
	}
	for eid := range spanIdx {
		sort.Slice(spanIdx[eid], func(i, j int) bool {
			return spanIdx[eid][i].ID < spanIdx[eid][j].ID
		})
	}

	// Count distinct Proposition.Subject values and their match status against span entity IDs.
	subjectMatched := make(map[string]bool)
	for _, p := range propositions {
		if p.Subject == "" {
			continue
		}
		if _, seen := subjectMatched[p.Subject]; seen {
			continue
		}
		_, matched := spanIdx[p.Subject]
		subjectMatched[p.Subject] = matched
	}
	for _, matched := range subjectMatched {
		if matched {
			cov.PropositionSubjectsMatched++
		} else {
			cov.PropositionSubjectsUnmatched++
		}
	}

	// Count distinct Span.EntityID values and their match status against proposition subjects.
	subjectSet := make(map[string]struct{}, len(subjectMatched))
	for subj := range subjectMatched {
		subjectSet[subj] = struct{}{}
	}
	entityMatched := make(map[string]bool)
	for _, s := range spans {
		if s.EntityID == "" {
			continue
		}
		if _, seen := entityMatched[s.EntityID]; seen {
			continue
		}
		_, matched := subjectSet[s.EntityID]
		entityMatched[s.EntityID] = matched
	}
	for _, matched := range entityMatched {
		if matched {
			cov.SpanEntityIDsMatched++
		} else {
			cov.SpanEntityIDsUnmatched++
		}
	}

	// Sort assertions by ID for deterministic outer iteration.
	sorted := make([]assertion.Assertion, len(assertions))
	copy(sorted, assertions)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	for _, a := range sorted {
		if a.Stance.IsRetraction() {
			continue
		}
		subject, ok := propSubject[a.PropositionID]
		if !ok {
			continue
		}
		matchedSpans, ok := spanIdx[subject]
		if !ok {
			continue
		}
		rankA := effectiveTierRank(a)
		refAssertion := Ref{Kind: RefKindAssertion, ID: a.ID}
		for _, s := range matchedSpans {
			if a.Content == s.Value {
				continue // agreement — no conflict
			}
			refSpan := Ref{Kind: RefKindSpan, ID: s.ID}
			spanGrade := s.EffectiveGrade()
			sev := severity(rankA, spanGrade.Tier.Rank())
			refA, refB := orderRefs(refAssertion, refSpan)
			emit(Conflict{
				Kind:     KindClaimVsEvidence,
				Subject:  subject,
				Field:    s.Field,
				A:        refA,
				B:        refB,
				Severity: sev,
				Reason: "assertion " + a.ID + " (content: " + a.Content +
					") conflicts with span " + s.ID + " (value: " + s.Value +
					") on entity " + s.EntityID + ", field " + s.Field,
			})
		}
	}

	return cov
}

// detectClaimVsEvidence emits KindClaimVsEvidence for each (assertion, span)
// pair where Assertion.PropositionID == Span.EntityID and the content differs.
// See package-level doc for the matching convention rationale.
func detectClaimVsEvidence(
	assertions []assertion.Assertion,
	spans []evidencespan.Span,
	emit func(Conflict),
) {
	// Build span index keyed by EntityID; sort each bucket by ID for determinism.
	spanIdx := make(map[string][]evidencespan.Span)
	for _, s := range spans {
		spanIdx[s.EntityID] = append(spanIdx[s.EntityID], s)
	}
	for eid := range spanIdx {
		sort.Slice(spanIdx[eid], func(i, j int) bool {
			return spanIdx[eid][i].ID < spanIdx[eid][j].ID
		})
	}

	// Sort assertions by ID for deterministic outer iteration.
	sorted := make([]assertion.Assertion, len(assertions))
	copy(sorted, assertions)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	for _, a := range sorted {
		if a.Stance.IsRetraction() {
			// A retraction carries no positive content claim; skip.
			continue
		}
		matchedSpans, ok := spanIdx[a.PropositionID]
		if !ok {
			continue
		}
		rankA := effectiveTierRank(a)
		refAssertion := Ref{Kind: RefKindAssertion, ID: a.ID}
		for _, s := range matchedSpans {
			if a.Content == s.Value {
				continue // agreement — no conflict
			}
			refSpan := Ref{Kind: RefKindSpan, ID: s.ID}
			spanGrade := s.EffectiveGrade()
			sev := severity(rankA, spanGrade.Tier.Rank())
			refA, refB := orderRefs(refAssertion, refSpan)
			emit(Conflict{
				Kind:     KindClaimVsEvidence,
				Subject:  a.PropositionID,
				Field:    s.Field,
				A:        refA,
				B:        refB,
				Severity: sev,
				Reason: "assertion " + a.ID + " (content: " + a.Content +
					") conflicts with span " + s.ID + " (value: " + s.Value +
					") on entity " + s.EntityID + ", field " + s.Field,
			})
		}
	}
}
