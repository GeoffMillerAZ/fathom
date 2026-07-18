// Package authority provides legal authority as a first-class node distinct
// from factual evidence. Where evidence answers "is this fact true?" (graded by
// epistemic tier), authority answers "does the law or rule support this
// conclusion?" (weighted by binding vs. persuasive + whether the authority is
// still good law).
//
// Scope: this package is the in-memory graph of authority nodes, treatment
// relationships (citation graph), and the Justification link that grounds a
// legal conclusion in both facts and law. Ingesting real statutes or case-law
// from external databases is an app/pack concern; this foundation is purely
// structural and deterministic.
//
// CUE contracts live in schema.cue; Go types here and behaviour in graph.go /
// validate.go mirror those contracts. Drift between the two is a bug.
//
// Concurrency contract: Graph is not safe for concurrent use; callers that
// share a Graph across goroutines must serialise access externally. Authority,
// Treatment, and Justification are plain value types safe to read concurrently.
package authority

import (
	"time"

	ap "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// AuthorityType classifies the legal source of an authority node.
//
// CUE mirror: #AuthorityType in schema.cue.
type AuthorityType string

const (
	// TypeStatute is a legislative act (federal, state, or local statute).
	TypeStatute AuthorityType = "statute"

	// TypeRegulation is an administrative / executive regulation or rule.
	TypeRegulation AuthorityType = "regulation"

	// TypeCase is a judicial decision (case law / precedent).
	TypeCase AuthorityType = "case"

	// TypeConstitutional is a constitutional provision (federal or state).
	TypeConstitutional AuthorityType = "constitutional"

	// TypeContractTerm is a contract clause that operates as law-of-the-case
	// between the contracting parties.
	TypeContractTerm AuthorityType = "contract_term"

	// TypePolicy is an internal or institutional policy with normative force.
	TypePolicy AuthorityType = "policy"

	// TypeSecondary is a secondary authority: law review articles, treatises,
	// restatements, model codes. Persuasive only; never binding on its own.
	TypeSecondary AuthorityType = "secondary"
)

// Weight describes the precedential strength of an authority relative to the
// deciding tribunal.
//
// CUE mirror: #Weight in schema.cue.
type Weight string

const (
	// WeightBinding is mandatory authority that the deciding court or body
	// must follow (e.g. on-point precedent from a higher court in the same
	// jurisdiction).
	WeightBinding Weight = "binding"

	// WeightPersuasive is authority that may be consulted and cited but that
	// the deciding body is free to reject (e.g. out-of-jurisdiction decisions,
	// secondary sources).
	WeightPersuasive Weight = "persuasive"

	// WeightNonAuthoritative is cited for context or background only; it carries
	// no precedential or persuasive weight in the technical sense.
	WeightNonAuthoritative Weight = "non_authoritative"
)

// Status records whether an authority is still good law at the time of
// evaluation.
//
// CUE mirror: #Status in schema.cue.
type Status string

const (
	// StatusGoodLaw means the authority has not been negatively treated and
	// remains valid precedent or operative law.
	StatusGoodLaw Status = "good_law"

	// StatusQuestioned means one or more authorities have criticised or
	// questioned the authority without fully overruling it.
	StatusQuestioned Status = "questioned"

	// StatusSuperseded means the authority has been replaced by a later
	// statute, regulation, or constitutional amendment (does not apply to
	// case law — use StatusOverruled for case law).
	StatusSuperseded Status = "superseded"

	// StatusOverruled means a court of equal or superior jurisdiction has
	// explicitly overruled the decision.
	StatusOverruled Status = "overruled"

	// StatusUnknown means the currency of the authority has not been verified.
	StatusUnknown Status = "unknown"
)

// TreatmentKind is the type of legal treatment applied to a cited authority.
// Treatments are directed: FromID cites or treats ToID.
//
// CUE mirror: #TreatmentKind in schema.cue.
type TreatmentKind string

const (
	// KindFollows means FromID applies the same rule as ToID.
	KindFollows TreatmentKind = "follows"

	// KindDistinguishes means FromID differentiates itself from ToID on the
	// facts without rejecting the rule.
	KindDistinguishes TreatmentKind = "distinguishes"

	// KindCriticizes means FromID questions the reasoning of ToID without
	// formally overruling it.
	KindCriticizes TreatmentKind = "criticizes"

	// KindOverrules means FromID explicitly overrules ToID (binding authority
	// only; persuasive overrule has no legal effect on status).
	KindOverrules TreatmentKind = "overrules"

	// KindReaffirms means FromID explicitly endorses the rule in ToID.
	KindReaffirms TreatmentKind = "reaffirms"

	// KindQuestions means FromID expresses doubt about the correctness of ToID.
	KindQuestions TreatmentKind = "questions"
)

// Authority is a legal authority node. It records everything needed to
// identify, weight, and evaluate the currency of a legal source.
//
// CUE mirror: #Authority in schema.cue.
type Authority struct {
	// ID is a non-empty unique identifier for this authority within the graph.
	ID string

	// Citation is the human-readable legal citation (e.g. "42 U.S.C. § 1983",
	// "Miranda v. Arizona, 384 U.S. 436 (1966)"). Must be non-empty.
	Citation string

	// Type classifies the legal source.
	Type AuthorityType

	// Jurisdiction is the issuing or governing jurisdiction (e.g. "US-SDNY",
	// "CA-9th-Cir", "UK-Supreme"). May be empty for secondary authorities.
	Jurisdiction string

	// Weight is the precedential strength of this authority relative to the
	// deciding tribunal.
	Weight Weight

	// Status is the currency of the authority (good law, questioned, etc.).
	Status Status

	// EffectiveFrom is the earliest date the authority was in force. The zero
	// value means "from the beginning of time" (open start of the window).
	EffectiveFrom time.Time

	// EffectiveTo is the latest date the authority was in force. The zero value
	// means the authority is still in force today (open end of the window).
	EffectiveTo time.Time

	// SourceRecordID references the ingested source record that produced this
	// authority node. May be empty for manually entered authorities.
	SourceRecordID string

	// SourceLocatorRef is the locator within SourceRecordID (e.g. page/section).
	// May be empty.
	SourceLocatorRef string

	// Author records the human or AI producer of this authority node. The zero
	// value (empty Kind) is valid — it means provenance was not captured.
	// When Author.Kind is set, Validate enforces Author.Validate().
	Author ap.Author
}

// Treatment is a directed citation/treatment edge in the authority graph.
// FromID cites or legally treats ToID. Both endpoints are referenced by ID;
// they need not pre-exist in the graph when a Treatment is recorded.
//
// CUE mirror: #Treatment in schema.cue.
type Treatment struct {
	// FromID is the ID of the authority that issues the treatment.
	FromID string

	// ToID is the ID of the authority being treated.
	ToID string

	// Kind is the nature of the legal treatment.
	Kind TreatmentKind
}

// Justification grounds a legal conclusion in both factual evidence and legal
// authority. It is the citation link that Throughline's outputs trace back to
// sources.
//
// Design note: a conclusion's strength is bounded by BOTH its weakest
// necessary evidence tier (see evidence-grade) AND whether its cited
// authorities are good law and binding. Neither axis alone is sufficient.
//
// CUE mirror: #Justification in schema.cue.
type Justification struct {
	// ConclusionID is the non-empty identifier of the claim or conclusion being
	// justified.
	ConclusionID string

	// EvidenceRefs is the ordered list of evidence span IDs that support the
	// conclusion. Must be non-empty — a conclusion without factual grounding is
	// a legal opinion only and must be handled separately.
	EvidenceRefs []string

	// AuthorityRefs is the ordered list of Authority IDs that provide legal
	// support. Must be non-empty — a conclusion must cite at least one legal
	// authority.
	AuthorityRefs []string
}
