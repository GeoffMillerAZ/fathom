// invalid.cue — NEGATIVE fixtures, documented (not live-vetted).
//
// These shapes intentionally violate schema.cue. They are kept commented so the
// repo gate (`cue vet ./...`) stays green; their REJECTION is asserted by the Go
// tests (Validate / Assess must reject each). To verify a case at the CUE layer
// manually, uncomment it and run:
//   cue vet ./testdata   (expect errors)
//
// 1. _badEmptyElementID — Element.ID violates _nonEmpty:
//      ID: "" -> field is required to be non-empty
//
// 2. _badEmptyElementName — Element.Name violates _nonEmpty:
//      Name: "" -> field is required to be non-empty
//
// 3. _badUnknownStatus — Status violates #Status enum:
//      Status: "maybe" -> conflicting values "maybe" and "met"|"under_evidenced"|"unmet"
//
// 4. _badConfidenceAboveOne — Confidence violates <=1:
//      Confidence: 1.5 -> value must be <= 1
//
// 5. _badConfidenceNegative — Confidence violates >=0:
//      Confidence: -0.1 -> value must be >= 0
//
// 6. _badMetWithGap — met + non-empty Gap violates _metImpliesNoGap:
//      Status: "met", Gap: "some gap text" -> _metImpliesNoGap: conflicting values false and true
//
// 7. _badUnmetWithTier — unmet + non-empty WeakestTier violates _unmetImpliesNoTier:
//      Status: "unmet", WeakestTier: "documentary" -> _unmetImpliesNoTier: conflicting values false and true

package elementcoverage

import "github.com/geoffmilleraz/agentic-kit/foundations/investigation/element-coverage:elementcoverage"

// _sentinel is a live reference to the import so `cue vet` does not report
// "imported and not used". All negative fixtures are in comments above.
_sentinel: elementcoverage.#Status & ("met" | "under_evidenced" | "unmet")

// _badEmptyElementID: empty ID violates the _nonEmpty constraint.
// _badEmptyElementID: elementcoverage.#Element & {
// 	ID:       ""
// 	Name:     "Some element"
// 	Required: true
// }

// _badEmptyElementName: empty Name violates the _nonEmpty constraint.
// _badEmptyElementName: elementcoverage.#Element & {
// 	ID:       "e1"
// 	Name:     ""
// 	Required: false
// }

// _badUnknownStatus: Status "maybe" is not in the #Status enum.
// _badUnknownStatus: elementcoverage.#ElementCoverage & {
// 	ElementID:       "e1"
// 	Status:          "maybe"
// 	WeakestTier:     "documentary"
// 	Confidence:      0.5
// 	EvidenceSpanIDs: ["span-x"]
// 	Gap:             ""
// }

// _badConfidenceAboveOne: Confidence 1.5 violates <=1.
// _badConfidenceAboveOne: elementcoverage.#ElementCoverage & {
// 	ElementID:       "e1"
// 	Status:          "met"
// 	WeakestTier:     "documentary"
// 	Confidence:      1.5
// 	EvidenceSpanIDs: ["span-x"]
// 	Gap:             ""
// }

// _badConfidenceNegative: Confidence -0.1 violates >=0.
// _badConfidenceNegative: elementcoverage.#ElementCoverage & {
// 	ElementID:       "e1"
// 	Status:          "under_evidenced"
// 	WeakestTier:     "inferred"
// 	Confidence:      -0.1
// 	EvidenceSpanIDs: ["span-x"]
// 	Gap:             "some gap"
// }

// _badMetWithGap: Status "met" with a non-empty Gap violates _metImpliesNoGap.
// The cross-field guard encodes: Status != "met" || Gap == ""
// When Status="met" and Gap="some gap text", the constraint evaluates to false.
// _badMetWithGap: elementcoverage.#ElementCoverage & {
// 	ElementID:       "e1"
// 	Status:          "met"
// 	WeakestTier:     "documentary"
// 	Confidence:      0.9
// 	EvidenceSpanIDs: ["span-x"]
// 	Gap:             "some gap text"
// }

// _badUnmetWithTier: Status "unmet" with a non-empty WeakestTier violates _unmetImpliesNoTier.
// No evidence means no tier can be derived; a non-empty WeakestTier is contradictory.
// _badUnmetWithTier: elementcoverage.#ElementCoverage & {
// 	ElementID:       "e1"
// 	Status:          "unmet"
// 	WeakestTier:     "documentary"
// 	Confidence:      0
// 	EvidenceSpanIDs: []
// 	Gap:             "no evidence for \"Element one\""
// }
