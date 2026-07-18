// invalid.cue — demonstrates #Conflict values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (the valid.cue
// sibling is the one vetted in CI). It is kept here as a documentation
// artifact so reviewers understand what the schema rejects.
package conflict

// _badKind: "hearsay" is not a recognised ConflictKind.
// _badKind: #Conflict & {
// 	Kind:     "hearsay"
// 	Subject:  "prop-x"
// 	Field:    ""
// 	A: {Kind: "assertion", ID: "asrt-001"}
// 	B: {Kind: "assertion", ID: "asrt-002"}
// 	Severity: 0.5
// 	Reason:   "invalid kind"
// }

// _badRefKind: "document" is not a recognised RefKind.
// _badRefKind: #Conflict & {
// 	Kind:     "claim_vs_claim"
// 	Subject:  "prop-x"
// 	Field:    ""
// 	A: {Kind: "document", ID: "doc-001"}
// 	B: {Kind: "assertion", ID: "asrt-002"}
// 	Severity: 0.5
// 	Reason:   "invalid ref kind"
// }

// _badSeverityHigh: Severity > 1 violates the >=0 & <=1 constraint.
// _badSeverityHigh: #Conflict & {
// 	Kind:     "claim_vs_claim"
// 	Subject:  "prop-x"
// 	Field:    ""
// 	A: {Kind: "assertion", ID: "asrt-001"}
// 	B: {Kind: "assertion", ID: "asrt-002"}
// 	Severity: 1.5
// 	Reason:   "severity out of range"
// }

// _badSeverityLow: Severity < 0 violates the >=0 & <=1 constraint.
// _badSeverityLow: #Conflict & {
// 	Kind:     "claim_vs_claim"
// 	Subject:  "prop-x"
// 	Field:    ""
// 	A: {Kind: "assertion", ID: "asrt-001"}
// 	B: {Kind: "assertion", ID: "asrt-002"}
// 	Severity: -0.1
// 	Reason:   "severity out of range"
// }

// _badEmptySubject: Subject must be non-empty.
// _badEmptySubject: #Conflict & {
// 	Kind:     "claim_vs_claim"
// 	Subject:  ""
// 	Field:    ""
// 	A: {Kind: "assertion", ID: "asrt-001"}
// 	B: {Kind: "assertion", ID: "asrt-002"}
// 	Severity: 0.5
// 	Reason:   "empty subject"
// }

// _badEmptyRefID: Ref.ID must be non-empty.
// _badEmptyRefID: #Conflict & {
// 	Kind:     "claim_vs_claim"
// 	Subject:  "prop-x"
// 	Field:    ""
// 	A: {Kind: "assertion", ID: ""}
// 	B: {Kind: "assertion", ID: "asrt-002"}
// 	Severity: 0.5
// 	Reason:   "empty ref ID"
// }
