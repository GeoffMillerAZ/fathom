package structuralrefusal

import (
	"fmt"
	"strings"
)

// Verdict is a judgment about ONE subject. Closed vocabulary: the unknown
// rate is a count over these, and two spellings of "unknown" would be two
// buckets.
type Verdict string

const (
	// VerdictMeets is the affirmative claim, the only one that carries a
	// burden of proof.
	VerdictMeets Verdict = "meets"
	// VerdictDoesNotMeet is an absence; it has nothing to cite and needs no
	// evidence.
	VerdictDoesNotMeet Verdict = "does-not-meet"
	// VerdictUnknown is the honest state of a claim nothing evidences either
	// way — and what an unevidenced "meets" becomes.
	VerdictUnknown Verdict = "unknown"
)

// Verdicts is the closed vocabulary, strongest claim first.
var Verdicts = []Verdict{VerdictMeets, VerdictDoesNotMeet, VerdictUnknown}

// Valid reports whether v is in the vocabulary.
func (v Verdict) Valid() bool {
	for _, k := range Verdicts {
		if k == v {
			return true
		}
	}
	return false
}

// Evidence is one citation: the kind of thing it resolves against and the id
// within that kind. An id whose kind nothing can resolve is not a citation;
// it is a string.
type Evidence struct {
	// Kind names what ID resolves against; the consumer's closed vocabulary.
	Kind string `json:"kind"`

	// ID names the one thing that substantiates the verdict.
	ID string `json:"id"`

	// Snapshot pins which version of the source the id resolves in. Ids are
	// stable but sources are not frozen: with the snapshot a later reader can
	// tell "this citation is stale" from "this citation is wrong"; without it
	// both look identical and the honest one is discarded with the dishonest
	// one.
	Snapshot string `json:"snapshot,omitempty"`
}

// Citable reports whether e names anything at all: a non-blank kind and id.
// Whether the kind resolves is the consumer's vocabulary.
func (e Evidence) Citable() bool {
	return strings.TrimSpace(e.Kind) != "" && strings.TrimSpace(e.ID) != ""
}

// Judgment is one subject, judged, mirroring #Judgment in schema.cue. Build
// it with New, or pass anything else through Normalize before trusting its
// Verdict.
type Judgment struct {
	// Subject is what was judged, in the asker's own words.
	Subject string `json:"subject"`

	// Verdict is the judgment after Normalize has had its say.
	Verdict Verdict `json:"verdict"`

	// Evidence is what the verdict rests on; required for "meets".
	Evidence []Evidence `json:"evidence,omitempty"`

	// Citation is the verbatim span a reader can check the claim against —
	// the human-legible half of Evidence's machine ids.
	Citation string `json:"citation,omitempty"`

	// Refused records why a claimed verdict was refused; empty when none
	// was. It is the audit trail: without it a judgment that arrived as
	// "meets" and one that was always "unknown" are indistinguishable, and
	// the refusal teaches nobody anything.
	Refused string `json:"refused,omitempty"`
}

// Rate is the headline number: how many judgments could not be evidenced
// either way, over how many there were. Mirrors #Rate.
type Rate struct {
	// Unknown counts the judgments whose normalized verdict is "unknown".
	Unknown int `json:"unknown"`

	// Total counts every judgment.
	Total int `json:"total"`
}

// Fraction returns Unknown/Total and whether there was anything to divide.
func (r Rate) Fraction() (float64, bool) {
	if r.Total <= 0 {
		return 0, false
	}
	return float64(r.Unknown) / float64(r.Total), true
}

// Render prints the rate through a sample floor: below floor judgments it
// says the evidence is thin instead of printing a percentage that three
// judgments cannot support.
func (r Rate) Render(floor int) string {
	switch {
	case r.Total <= 0:
		return "no judgments"
	case r.Total < floor:
		return fmt.Sprintf("not enough judgments yet (%d, floor %d)", r.Total, floor)
	}
	pct := 100 * float64(r.Unknown) / float64(r.Total)
	return fmt.Sprintf("%d of %d unknown (%.0f%%)", r.Unknown, r.Total, pct)
}
