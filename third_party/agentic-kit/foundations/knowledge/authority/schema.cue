// Package authority defines the CUE-first contracts for legal authority nodes,
// treatment (citation) edges, and justification links in the authority graph.
// Go types in types.go mirror these definitions; any drift between the two is
// a bug.
//
// Design principle: Authority answers "does the law or rule support this
// conclusion?" — distinct from Evidence, which answers "is this fact true?".
// A conclusion's strength is bounded by BOTH its weakest necessary evidence
// tier AND whether its authorities are good law and binding.
package authority

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #AuthorityType classifies the legal source of an authority node.
#AuthorityType:
	"statute" |
	"regulation" |
	"case" |
	"constitutional" |
	"contract_term" |
	"policy" |
	"secondary"

// #Weight describes the precedential strength of an authority.
//
//   - binding:           mandatory; the deciding body must follow it.
//   - persuasive:        instructive; the deciding body may reject it.
//   - non_authoritative: context only; no precedential weight.
#Weight:
	"binding" |
	"persuasive" |
	"non_authoritative"

// #Status records whether an authority is still good law.
#Status:
	"good_law" |
	"questioned" |
	"superseded" |
	"overruled" |
	"unknown"

// #TreatmentKind is the nature of a citation/treatment edge.
// Treatments are directed: FromID cites or legally treats ToID.
#TreatmentKind:
	"follows" |
	"distinguishes" |
	"criticizes" |
	"overrules" |
	"reaffirms" |
	"questions"

// #Authority is a legal authority node.
#Authority: {
	// ID is a non-empty unique identifier for this authority within the graph.
	ID: _nonEmpty

	// Citation is the human-readable legal citation. Must be non-empty.
	Citation: _nonEmpty

	// Type classifies the legal source.
	Type: #AuthorityType

	// Jurisdiction is the issuing or governing jurisdiction. May be empty for
	// secondary authorities.
	Jurisdiction: string

	// Weight is the precedential strength.
	Weight: #Weight

	// Status is the currency of the authority.
	Status: #Status

	// EffectiveFrom is an RFC 3339 timestamp for the start of the in-force
	// window. Empty string means open start ("from the beginning of time").
	EffectiveFrom: string

	// EffectiveTo is an RFC 3339 timestamp for the end of the in-force window.
	// Empty string means the authority is still in force today (open end).
	EffectiveTo: string

	// SourceRecordID references the ingested source record. May be empty.
	SourceRecordID: string

	// SourceLocatorRef is the locator within SourceRecordID. May be empty.
	SourceLocatorRef: string

	// Author records who or what produced this node. Optional — the Kind field
	// distinguishes human ("human") from AI ("ai"); empty means not captured.
	// When populated, the full #Author discriminated-union shape applies.
	Author: {
		Kind: string
		...
	} | *{Kind: ""}
}

// #Treatment is a directed citation/treatment edge in the authority graph.
// FromID cites or legally treats ToID.
#Treatment: {
	// FromID is the ID of the treating authority. Must be non-empty.
	FromID: _nonEmpty

	// ToID is the ID of the authority being treated. Must be non-empty.
	ToID: _nonEmpty

	// Kind is the nature of the legal treatment.
	Kind: #TreatmentKind
}

// #Justification grounds a legal conclusion in both factual evidence and legal
// authority. A conclusion's strength is bounded by BOTH its weakest necessary
// evidence tier AND whether its cited authorities are good law and binding.
#Justification: {
	// ConclusionID is the non-empty identifier of the claim being justified.
	ConclusionID: _nonEmpty

	// EvidenceRefs is the non-empty list of evidence span IDs. At least one
	// is required — a conclusion without factual grounding is a legal opinion
	// only.
	EvidenceRefs: [_nonEmpty, ..._nonEmpty]

	// AuthorityRefs is the non-empty list of Authority IDs providing legal
	// support. At least one is required.
	AuthorityRefs: [_nonEmpty, ..._nonEmpty]
}
