// invalid.cue — demonstrates #Authority, #Treatment, and #Justification values
// that VIOLATE the schema defined in schema.cue. This file is intentionally NOT
// included in the `cue vet` path (the valid.cue sibling is the one vetted in
// CI). It is kept here as a documentation artifact so reviewers understand what
// the schema rejects. Uncomment any block and run `cue vet .` from this
// directory to observe the rejection. The equivalent Go-level rejections are
// covered by Validate() tests in authority_test.go.
package authority

// _badType: AuthorityType must be one of the seven recognised values.
// "ordinance" is not in the enumeration.
// _badType: #Authority & {
// 	ID:            "auth-bad-001"
// 	Citation:      "City Ordinance 12.3"
// 	Type:          "ordinance"
// 	Jurisdiction:  "US-NYC"
// 	Weight:        "binding"
// 	Status:        "good_law"
// 	EffectiveFrom: ""
// 	EffectiveTo:   ""
// 	SourceRecordID: ""
// 	SourceLocatorRef: ""
// 	Author: {Kind: ""}
// }

// _badWeight: Weight must be binding|persuasive|non_authoritative.
// "mandatory" is not recognised.
// _badWeight: #Authority & {
// 	ID:            "auth-bad-002"
// 	Citation:      "42 U.S.C. § 1983"
// 	Type:          "statute"
// 	Jurisdiction:  "US-Federal"
// 	Weight:        "mandatory"
// 	Status:        "good_law"
// 	EffectiveFrom: ""
// 	EffectiveTo:   ""
// 	SourceRecordID: ""
// 	SourceLocatorRef: ""
// 	Author: {Kind: ""}
// }

// _badStatus: Status must be good_law|questioned|superseded|overruled|unknown.
// "abrogated" is not recognised.
// _badStatus: #Authority & {
// 	ID:            "auth-bad-003"
// 	Citation:      "42 U.S.C. § 1983"
// 	Type:          "statute"
// 	Jurisdiction:  "US-Federal"
// 	Weight:        "binding"
// 	Status:        "abrogated"
// 	EffectiveFrom: ""
// 	EffectiveTo:   ""
// 	SourceRecordID: ""
// 	SourceLocatorRef: ""
// 	Author: {Kind: ""}
// }

// _emptyID: Authority.ID must be non-empty (_nonEmpty constraint).
// _emptyID: #Authority & {
// 	ID:            ""
// 	Citation:      "42 U.S.C. § 1983"
// 	Type:          "statute"
// 	Jurisdiction:  "US-Federal"
// 	Weight:        "binding"
// 	Status:        "good_law"
// 	EffectiveFrom: ""
// 	EffectiveTo:   ""
// 	SourceRecordID: ""
// 	SourceLocatorRef: ""
// 	Author: {Kind: ""}
// }

// _emptyCitation: Authority.Citation must be non-empty (_nonEmpty constraint).
// _emptyCitation: #Authority & {
// 	ID:            "auth-bad-004"
// 	Citation:      ""
// 	Type:          "statute"
// 	Jurisdiction:  "US-Federal"
// 	Weight:        "binding"
// 	Status:        "good_law"
// 	EffectiveFrom: ""
// 	EffectiveTo:   ""
// 	SourceRecordID: ""
// 	SourceLocatorRef: ""
// 	Author: {Kind: ""}
// }

// _badTreatmentKind: TreatmentKind must be one of the six recognised values.
// "cites" is not in the enumeration.
// _badTreatmentKind: #Treatment & {
// 	FromID: "auth-case-002"
// 	ToID:   "auth-case-001"
// 	Kind:   "cites"
// }

// _emptyTreatmentFrom: Treatment.FromID must be non-empty.
// _emptyTreatmentFrom: #Treatment & {
// 	FromID: ""
// 	ToID:   "auth-case-001"
// 	Kind:   "follows"
// }

// _emptyTreatmentTo: Treatment.ToID must be non-empty.
// _emptyTreatmentTo: #Treatment & {
// 	FromID: "auth-case-002"
// 	ToID:   ""
// 	Kind:   "follows"
// }

// _justificationEmptyConclusionID: ConclusionID must be non-empty.
// _justificationEmptyConclusionID: #Justification & {
// 	ConclusionID:  ""
// 	EvidenceRefs:  ["ev-001"]
// 	AuthorityRefs: ["auth-001"]
// }

// _justificationEmptyEvidenceRefs: EvidenceRefs must have at least one entry.
// CUE enforces this via the [_nonEmpty, ..._nonEmpty] list constraint, which
// requires at least one element.
// _justificationEmptyEvidenceRefs: #Justification & {
// 	ConclusionID:  "concl-001"
// 	EvidenceRefs:  []
// 	AuthorityRefs: ["auth-001"]
// }

// _justificationEmptyAuthorityRefs: AuthorityRefs must have at least one entry.
// _justificationEmptyAuthorityRefs: #Justification & {
// 	ConclusionID:  "concl-001"
// 	EvidenceRefs:  ["ev-001"]
// 	AuthorityRefs: []
// }
