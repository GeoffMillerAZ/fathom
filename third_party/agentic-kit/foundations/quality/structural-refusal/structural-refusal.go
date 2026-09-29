package structuralrefusal

import (
	"fmt"
	"strings"
)

// Refuse is the rule, in the value: a "meets" with no citable evidence is
// "unknown", and a verdict outside the vocabulary is "unknown". Everything
// else stands — "does-not-meet" carries no burden of proof, and "unknown" is
// never promoted. It counts citable evidence, not list length.
func Refuse(v Verdict, evidence []Evidence) Verdict {
	if !v.Valid() {
		return VerdictUnknown
	}
	if v == VerdictMeets && citable(evidence) == 0 {
		return VerdictUnknown
	}
	return v
}

// New is the constructor: the only way to build a Judgment in Go that has
// been through Normalize, so a struct literal in a handler cannot be how an
// unevidenced "meets" reaches a store.
func New(subject string, v Verdict, evidence []Evidence, citation string) Judgment {
	return Normalize(Judgment{Subject: subject, Verdict: v, Evidence: evidence, Citation: citation})
}

// Normalize enforces the shape's one structural rule and records why. It is
// idempotent: normalizing an already-refused judgment neither re-refuses it
// nor erases the reason it was refused, and a verdict that stands carries no
// refusal note.
//
// Uncitable evidence — no kind, or no id — is discarded BEFORE it is
// counted, and the discard is named in the refusal so a mis-shaped citation
// reads as "your references did not resolve" rather than "you cited
// nothing".
func Normalize(j Judgment) Judgment {
	j.Subject = strings.TrimSpace(j.Subject)
	j.Citation = strings.TrimSpace(j.Citation)

	kept := make([]Evidence, 0, len(j.Evidence))
	discarded := 0
	for _, e := range j.Evidence {
		e.Kind = strings.TrimSpace(e.Kind)
		e.ID = strings.TrimSpace(e.ID)
		e.Snapshot = strings.TrimSpace(e.Snapshot)
		if !e.Citable() {
			discarded++
			continue
		}
		kept = append(kept, e)
	}
	if len(kept) == 0 {
		kept = nil // keep the serialized form free of empty lists
	}
	j.Evidence = kept

	switch {
	case !j.Verdict.Valid():
		claimed := string(j.Verdict)
		j.Verdict = VerdictUnknown
		j.Refused = fmt.Sprintf("refused verdict %q: not one of %s", claimed, verdictList())
	case j.Verdict == VerdictMeets && len(j.Evidence) == 0:
		j.Verdict = VerdictUnknown
		j.Refused = `refused "meets": no citable evidence` + discardedNote(discarded)
	case j.Verdict != VerdictUnknown:
		j.Refused = ""
	}
	return j
}

// UnknownRate is the headline metric: the share of judgments this set could
// not evidence either way. It normalizes on the way past, so judgments that
// entered a store through a loader (which bypasses New) are counted by the
// same rule as judgments built in Go. Render the result through Rate.Render
// so a set of three says "not enough judgments yet" instead of "33%".
func UnknownRate(js []Judgment) Rate {
	r := Rate{Total: len(js)}
	for _, j := range js {
		if Normalize(j).Verdict == VerdictUnknown {
			r.Unknown++
		}
	}
	return r
}

func citable(evidence []Evidence) int {
	n := 0
	for _, e := range evidence {
		if e.Citable() {
			n++
		}
	}
	return n
}

func discardedNote(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d unresolvable reference(s) discarded)", n)
}

func verdictList() string {
	names := make([]string, len(Verdicts))
	for i, v := range Verdicts {
		names[i] = string(v)
	}
	return strings.Join(names, " | ")
}
