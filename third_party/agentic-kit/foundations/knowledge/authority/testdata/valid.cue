// valid.cue — concrete values that satisfy the #Authority, #Treatment, and
// #Justification constraints defined in schema.cue. Running `cue vet ./...`
// from the package root or repo root validates these against the schema.
package authority

// A well-formed binding statute with a closed effective window.
_statute1: #Authority & {
	ID:               "auth-statute-001"
	Citation:         "42 U.S.C. § 1983"
	Type:             "statute"
	Jurisdiction:     "US-Federal"
	Weight:           "binding"
	Status:           "good_law"
	EffectiveFrom:    "1871-04-20T00:00:00Z"
	EffectiveTo:      ""
	SourceRecordID:   "rec-uscode-042-1983"
	SourceLocatorRef: "section-1983"
	Author: {Kind: ""}
}

// A case-law authority with a human author.
_case1: #Authority & {
	ID:               "auth-case-001"
	Citation:         "Miranda v. Arizona, 384 U.S. 436 (1966)"
	Type:             "case"
	Jurisdiction:     "US-Supreme"
	Weight:           "binding"
	Status:           "good_law"
	EffectiveFrom:    "1966-06-13T00:00:00Z"
	EffectiveTo:      ""
	SourceRecordID:   "rec-us-reports-384"
	SourceLocatorRef: "page-436"
	Author: {
		Kind: "human"
		Human: {ID: "counsel@example.com"}
	}
}

// A persuasive secondary authority (law review article).
_secondary1: #Authority & {
	ID:               "auth-secondary-001"
	Citation:         "Prosser, Law of Torts § 45 (4th ed. 1971)"
	Type:             "secondary"
	Jurisdiction:     ""
	Weight:           "persuasive"
	Status:           "good_law"
	EffectiveFrom:    ""
	EffectiveTo:      ""
	SourceRecordID:   ""
	SourceLocatorRef: ""
	Author: {Kind: ""}
}

// A regulation that has been superseded.
_regulation1: #Authority & {
	ID:               "auth-reg-001"
	Citation:         "40 C.F.R. § 60.1 (1971 version)"
	Type:             "regulation"
	Jurisdiction:     "US-Federal"
	Weight:           "binding"
	Status:           "superseded"
	EffectiveFrom:    "1971-12-23T00:00:00Z"
	EffectiveTo:      "2023-01-01T00:00:00Z"
	SourceRecordID:   "rec-cfr-40-60-1"
	SourceLocatorRef: "section-60.1"
	Author: {Kind: ""}
}

// A well-formed follows treatment: newer_case follows older_case.
_treatment1: #Treatment & {
	FromID: "auth-case-002"
	ToID:   "auth-case-001"
	Kind:   "follows"
}

// An overrules treatment.
_treatment2: #Treatment & {
	FromID: "auth-case-003"
	ToID:   "auth-case-002"
	Kind:   "overrules"
}

// A questions treatment.
_treatment3: #Treatment & {
	FromID: "auth-case-004"
	ToID:   "auth-case-001"
	Kind:   "questions"
}

// A well-formed justification grounding a conclusion in evidence and authority.
_justification1: #Justification & {
	ConclusionID:  "concl-excessive-force-001"
	EvidenceRefs:  ["ev-span-001", "ev-span-002"]
	AuthorityRefs: ["auth-statute-001", "auth-case-001"]
}

// A minimal justification with one evidence ref and one authority ref.
_justification2: #Justification & {
	ConclusionID:  "concl-jurisdiction-001"
	EvidenceRefs:  ["ev-span-003"]
	AuthorityRefs: ["auth-case-001"]
}
