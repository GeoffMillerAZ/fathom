package expertchat

import "fmt"

// RefusalReason names why Run refused to answer. See the package doc,
// "Refusal taxonomy".
type RefusalReason string

const (
	// RefusalNoEvidence: the retriever returned nothing, or returned only
	// claims that are never citable (StatusContradicted/StatusRefuted).
	RefusalNoEvidence RefusalReason = "no-evidence"

	// RefusalBelowTierThreshold: citable evidence exists, but none of it
	// meets Deps.Policy.TierThreshold.
	RefusalBelowTierThreshold RefusalReason = "below-tier-threshold"

	// RefusalStaleOnlyEvidence: Deps.Policy.RequireFresh is set, and every
	// tier-eligible claim is StatusStale.
	RefusalStaleOnlyEvidence RefusalReason = "stale-only-evidence"

	// RefusalFabricatedCitation: the model's response, after one retry, still
	// cited a claim ID not present in the supplied evidence set. Not one of
	// the plan's original three reasons — see the package doc for why it
	// gets its own name rather than being folded into RefusalNoEvidence.
	RefusalFabricatedCitation RefusalReason = "fabricated-citation"

	// RefusalEvidenceDoesNotAnswer: the model was given eligible, citable
	// evidence and HONESTLY reported (the systemPrompt-mandated literal
	// "CITES: (none)") that none of it answers the query. This is an
	// immediate, non-retried refusal — retrying an honest "(none)" would
	// pressure the model into fabricating a citation just to satisfy the
	// loop, exactly the dishonesty this whole package exists to prevent.
	// Deliberately distinct from RefusalNoEvidence: evidence WAS retrieved
	// and WAS eligible here, it just didn't answer the question — collapsing
	// the two would misreport what actually happened.
	RefusalEvidenceDoesNotAnswer RefusalReason = "evidence-does-not-answer"
)

// Refusal is the typed alternative to a CitedAnswer: Run returns one,
// wrapped as an error, whenever it will not answer. Refusal implements the
// error interface so callers use the idiomatic Go pattern:
//
//	answer, err := expertchat.Run(ctx, deps, query)
//	var refusal *expertchat.Refusal
//	if errors.As(err, &refusal) {
//	    // handle refusal.Reason / refusal.Detail
//	}
type Refusal struct {
	// Reason classifies why Run refused. One of the RefusalXxx constants.
	Reason RefusalReason

	// Query is the query that was refused.
	Query string

	// Detail is a human-readable reason naming what was searched — e.g. how
	// many items were retrieved, what tier/freshness policy was in force, and
	// what filtering left nothing eligible. Never empty.
	Detail string
}

// Error implements the error interface.
func (r *Refusal) Error() string {
	return fmt.Sprintf("expertchat: refused to answer %q (%s): %s", r.Query, r.Reason, r.Detail)
}
