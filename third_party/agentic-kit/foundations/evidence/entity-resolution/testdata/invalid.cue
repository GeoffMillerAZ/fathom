// invalid.cue — demonstrates #Candidate and #Match values that VIOLATE the schema.
//
// Per repo convention (see Taskfile cue-vet), negative cases live as commented
// documentation here; the executable rejection contracts are the Go validation
// tests. testdata/ IS vetted by the gate, so these bindings stay commented —
// uncomment one locally to watch `cue vet ./testdata/` reject it.
//
// Each case includes a comment explaining which constraint it violates.
package entityresolution

// ── Invalid Candidate cases ────────────────────────────────────────────────────

// _badScoreHigh: Score > 1 violates the >=0 & <=1 constraint.
// _badScoreHigh: #Candidate & {
// 	LeftID: "entity-a"
// 	RightID: "entity-b"
// 	Score:   1.5
// 	Method:  "name-similarity"
// 	Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 	Grade: {Tier: "documentary", Confidence: 0.9}
// }

// _badScoreLow: Score < 0 violates the >=0 & <=1 constraint.
// _badScoreLow: #Candidate & {
// 	LeftID:  "entity-a"
// 	RightID: "entity-b"
// 	Score:   -0.1
// 	Method:  "name-similarity"
// 	Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 	Grade: {Tier: "documentary", Confidence: 0.9}
// }

// _badEmptyLeftID: empty LeftID violates the _nonEmpty constraint.
// _badEmptyLeftID: #Candidate & {
// 	LeftID:  ""
// 	RightID: "entity-b"
// 	Score:   0.5
// 	Method:  "name-similarity"
// 	Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 	Grade: {Tier: "documentary", Confidence: 0.9}
// }

// _badEmptyRightID: empty RightID violates the _nonEmpty constraint.
// _badEmptyRightID: #Candidate & {
// 	LeftID:  "entity-a"
// 	RightID: ""
// 	Score:   0.5
// 	Method:  "name-similarity"
// 	Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 	Grade: {Tier: "documentary", Confidence: 0.9}
// }

// _badEmptyMethod: empty Method violates the _nonEmpty constraint.
// _badEmptyMethod: #Candidate & {
// 	LeftID:  "entity-a"
// 	RightID: "entity-b"
// 	Score:   0.5
// 	Method:  ""
// 	Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 	Grade: {Tier: "documentary", Confidence: 0.9}
// }

// _badCanonicalOrder: LeftID > RightID violates the canonical ordering invariant
// (RightID: _nonEmpty & >LeftID). The pair ("z-entity", "a-entity") is invalid
// because "z-entity" > "a-entity"; the canonical form must be ("a-entity", "z-entity").
// _badCanonicalOrder: #Candidate & {
// 	LeftID:  "z-entity"
// 	RightID: "a-entity"
// 	Score:   0.9
// 	Method:  "name-similarity"
// 	Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 	Grade: {Tier: "documentary", Confidence: 0.9}
// }

// _badGradeTier: unknown Tier value violates the #Tier enum constraint.
// _badGradeTier: #Candidate & {
// 	LeftID:  "entity-a"
// 	RightID: "entity-b"
// 	Score:   0.5
// 	Method:  "name-similarity"
// 	Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 	Grade: {Tier: "speculative", Confidence: 0.5}
// }

// ── Invalid ReviewState case ───────────────────────────────────────────────────

// _badReviewState: invalid state string violates the #ReviewState enum.
// _badReviewState: #Match & {
// 	Candidate: {
// 		LeftID:  "entity-a"
// 		RightID: "entity-b"
// 		Score:   0.5
// 		Method:  "name-similarity"
// 		Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 		Grade: {Tier: "documentary", Confidence: 0.9}
// 	}
// 	State:     "pending"
// 	CreatedAt: "2026-06-08T12:00:00Z"
// }

// ── Invalid Match cases ────────────────────────────────────────────────────────

// _badMissingReviewedBy: accepted match with empty ReviewedBy violates the
// cross-field constraint: when State is "accepted", ReviewedBy must be _nonEmpty.
// _badMissingReviewedBy: #Match & {
// 	Candidate: {
// 		LeftID:  "entity-a"
// 		RightID: "entity-b"
// 		Score:   0.5
// 		Method:  "name-similarity"
// 		Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 		Grade: {Tier: "documentary", Confidence: 0.9}
// 	}
// 	State:      "accepted"
// 	CreatedAt:  "2026-06-08T12:00:00Z"
// 	ReviewedBy: ""
// 	ReviewedAt: "2026-06-08T13:00:00Z"
// }

// _badMissingReviewedAt: rejected match with no ReviewedAt.
//
// Note: CUE's if-conditional constraints (used in #Match) restrict the VALUE of
// optional fields when present, but do not enforce the field's PRESENCE when the
// field is declared optional (?). The Go layer (Match.Validate) authoritatively
// enforces that ReviewedAt is non-zero when State is "accepted" or "rejected".
// This case is documented here as a prose contract; CUE enforcement of field
// presence requires a closed-struct disjunction which is architecturally verbose
// for this type. The Go test TestMatchValidate_ReviewedAtRequiredWhenDecided is
// the executable contract.
//
// _badMissingReviewedAt_DOCUMENTED_NOT_REJECTED_BY_CUE: #Match & {
// 	Candidate: {
// 		LeftID:  "entity-a"
// 		RightID: "entity-b"
// 		Score:   0.5
// 		Method:  "name-similarity"
// 		Author: {Kind: "human", Human: {ID: "analyst-1"}}
// 		Grade: {Tier: "documentary", Confidence: 0.9}
// 	}
// 	State:      "rejected"
// 	CreatedAt:  "2026-06-08T12:00:00Z"
// 	ReviewedBy: "analyst-1"
// 	// ReviewedAt deliberately omitted
// }
