// Package retrievalmetrics provides pure, deterministic retrieval quality
// metrics for the evidence-spine RAG layer. All functions are stateless and
// safe for concurrent use.
//
// # Metric applicability
//
// Not every metric is meaningful for every concern archetype. The table below
// is the authoritative applicability matrix:
//
//	Archetype    Precision  Recall  MRR  FalseAbsentRate  CoverageFraction
//	-----------  ---------  ------  ---  ---------------  ----------------
//	existence       ✓         ✓      ✓         ✗                ✗
//	absence         ✗         ✗      ✗         ✓                ✓
//	threshold       ✓         ✓      ✓         ✗                ✗
//	contradicts     ✓         ✓      ✓         ✗                ✗
//
// Each function returns a MetricResult with Applicable = false (and a Reason)
// rather than returning NaN or 0 when the metric is not defined for the inputs.
// Callers must check Applicable before using Value.
//
// # MRR and empty gold sets
//
// MRR is undefined on an empty gold set (no expected results). Rather than
// returning NaN or panicking, MRR returns MetricResult{Applicable: false,
// Reason: "MRR undefined: gold set is empty"}. Absence archetypes always
// produce this because their gold set is always empty (the expected answer is
// "nothing found"). Do not call MRR for absence archetypes.
//
// # FalseAbsentRate and determinism
//
// FalseAbsentRate is measured deterministically from CanaryRecord inputs.
// Each CanaryRecord records whether the search engine found a canary fixture
// that was planted in the corpus. A canary that was not found (CanaryFound ==
// false) is a false absent: the engine would have missed a real hit had one
// been planted there. The production proxy for this rate is the Tier-3
// overturn rate (cases where a verdict of ABSENT was later overturned by
// review), but the deterministic measurement uses canary outcomes only.
//
// # Non-guarantees
//
// These metrics describe retrieval quality, not claim truth. A high-Precision
// retriever may still miss correct findings; a perfect-Recall retriever may
// surface irrelevant chunks. Metric values are only meaningful relative to
// the quality of the gold/expected sets provided by the caller.
//
// Concurrency: all exported functions are pure and safe for concurrent use.
// No package-level state exists.
//
// CUE contracts live in schema.cue; this file mirrors those definitions.
package retrievalmetrics

import (
	evidencesignature "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-signature"
)

// MetricResult is the result of a single metric computation.
//
// Applicable is false when the metric is not defined for the given inputs
// (e.g., MRR on an empty gold set, FalseAbsentRate on a non-absence concern).
// When Applicable is false, Value is 0 and Reason explains why.
// When Applicable is true, Value is in [0, 1] and Reason is empty.
//
// CUE mirror: #MetricResult in schema.cue.
type MetricResult struct {
	// Value is the metric value in [0, 1]. Only meaningful when Applicable is true.
	Value float64

	// Applicable is false when this metric is not defined for the inputs.
	// Callers MUST check Applicable before using Value.
	Applicable bool

	// Reason is the human-readable explanation for why Applicable is false.
	// Empty when Applicable is true.
	Reason string
}

// notApplicable constructs a MetricResult for a non-applicable metric.
func notApplicable(reason string) MetricResult {
	return MetricResult{Applicable: false, Reason: reason}
}

// applicable constructs a MetricResult for a computed metric value.
func applicable(value float64) MetricResult {
	return MetricResult{Value: value, Applicable: true}
}

// CanaryRecord records whether a canary fixture was found by the search engine
// during a specific search run. The population of CanaryRecord values is the
// ground truth for FalseAbsentRate.
//
// CUE mirror: #CanaryRecord in schema.cue.
type CanaryRecord struct {
	// SignatureID is the ConcernSignature.ID this canary belongs to.
	SignatureID string

	// CanaryFound is true if the search engine found the canary's pattern in
	// the expected file during the search run.
	CanaryFound bool
}

// Precision computes the fraction of retrieved results that are relevant.
//
//	Precision = |found ∩ expected| / |found|
//
// Returns not-applicable when:
//   - archetype is absence (absence concerns have no positive gold set;
//     precision is undefined for a zero-hit expected outcome).
//   - found is empty (precision is undefined on an empty result set).
//
// Otherwise returns a value in [0, 1].
func Precision(archetype evidencesignature.Archetype, expected, found []string) MetricResult {
	if archetype == evidencesignature.ArchetypeAbsence {
		return notApplicable("Precision not applicable to absence archetype: absence concerns have no positive gold set")
	}
	if len(found) == 0 {
		return notApplicable("Precision undefined: retrieved set is empty")
	}
	hits := intersection(expected, found)
	return applicable(float64(hits) / float64(len(found)))
}

// Recall computes the fraction of relevant results that were retrieved.
//
//	Recall = |found ∩ expected| / |expected|
//
// Returns not-applicable when:
//   - archetype is absence (same reason as Precision).
//   - expected is empty (recall is undefined on an empty gold set).
//
// Otherwise returns a value in [0, 1].
func Recall(archetype evidencesignature.Archetype, expected, found []string) MetricResult {
	if archetype == evidencesignature.ArchetypeAbsence {
		return notApplicable("Recall not applicable to absence archetype: absence concerns have no positive gold set")
	}
	if len(expected) == 0 {
		return notApplicable("Recall undefined: gold set is empty")
	}
	hits := intersection(expected, found)
	return applicable(float64(hits) / float64(len(expected)))
}

// MRR computes the Mean Reciprocal Rank of the first relevant result in found.
//
//	MRR = mean_over_queries(1/rank_of_first_relevant_result)
//
// This single-query form returns 1/rank of the first found item that appears
// in expected, or 0 if no relevant item was found.
//
// Returns not-applicable when:
//   - archetype is absence (MRR is undefined on absence concerns whose gold
//     set is always empty by definition).
//   - expected is empty (MRR is undefined on an empty gold set — it must
//     never return NaN).
//
// Otherwise returns a value in [0, 1].
func MRR(archetype evidencesignature.Archetype, expected, found []string) MetricResult {
	if archetype == evidencesignature.ArchetypeAbsence {
		return notApplicable("MRR not applicable to absence archetype: gold set is always empty for absence concerns")
	}
	if len(expected) == 0 {
		return notApplicable("MRR undefined: gold set is empty")
	}
	expectedSet := toSet(expected)
	for i, id := range found {
		if _, ok := expectedSet[id]; ok {
			return applicable(1.0 / float64(i+1))
		}
	}
	// No relevant result found — reciprocal rank is 0, not undefined.
	return applicable(0.0)
}

// FalseAbsentRate computes the fraction of canary records where the canary was
// NOT found by the search engine.
//
//	FalseAbsentRate = |{r : !r.CanaryFound}| / |records|
//
// A high FalseAbsentRate means the search engine would miss real hits with
// non-trivial probability. This rate is the ground truth for absence-quality
// in test environments. In production, the proxy is the Tier-3 overturn rate
// (cases where an ABSENT verdict was later overturned by human review).
//
// Returns not-applicable when:
//   - archetype is not absence (FalseAbsentRate is only meaningful for absence
//     concerns that depend on canary validation for their ABSENT_PROVEN guarantee).
//   - records is empty (rate is undefined on an empty sample).
//
// Otherwise returns a value in [0, 1]. A value of 0 means all canaries were
// found (best case). A value of 1 means none were found (worst case — the
// engine is blind).
func FalseAbsentRate(archetype evidencesignature.Archetype, records []CanaryRecord) MetricResult {
	if archetype != evidencesignature.ArchetypeAbsence {
		return notApplicable("FalseAbsentRate only applicable to absence archetype")
	}
	if len(records) == 0 {
		return notApplicable("FalseAbsentRate undefined: canary record set is empty")
	}
	missed := 0
	for _, r := range records {
		if !r.CanaryFound {
			missed++
		}
	}
	return applicable(float64(missed) / float64(len(records)))
}

// CoverageFraction computes the fraction of files in a CoverageManifest's
// scope that were fully searched (i.e., included in manifest.Files and not
// in any skipped category).
//
//	CoverageFraction = |manifest.Files| / (|manifest.Files| + |manifest.Skipped.AllSkipped()|)
//
// This measures live-tree coverage: what fraction of the scope was actually
// walked and searched. The scope is defined by the manifest's own file list
// plus all skipped files.
//
// Returns not-applicable when:
//   - archetype is not absence (coverage fraction is only meaningful when an
//     absence verdict depends on having walked the entire scope).
//   - the total file count (searched + skipped) is zero (fraction undefined).
//
// Otherwise returns a value in [0, 1]. A value of 1.0 means every file in the
// scope was searched (and manifest.Complete() == true is likely). A value < 1
// means some files were skipped and ABSENT_PROVEN may be downgraded.
func CoverageFraction(archetype evidencesignature.Archetype, manifest evidencesignature.CoverageManifest) MetricResult {
	if archetype != evidencesignature.ArchetypeAbsence {
		return notApplicable("CoverageFraction only applicable to absence archetype")
	}
	searched := len(manifest.Files)
	skipped := len(manifest.Skipped.AllSkipped())
	total := searched + skipped
	if total == 0 {
		return notApplicable("CoverageFraction undefined: no files in scope (manifest is empty)")
	}
	return applicable(float64(searched) / float64(total))
}

// ---------------------------------------------------------------------------
// Set-operation helpers (pure, no allocation beyond the set map).
// ---------------------------------------------------------------------------

// intersection returns the count of elements in both a and b.
func intersection(a, b []string) int {
	aSet := toSet(a)
	count := 0
	for _, v := range b {
		if _, ok := aSet[v]; ok {
			count++
		}
	}
	return count
}

// toSet converts a string slice to a set (map[string]struct{}).
func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}
