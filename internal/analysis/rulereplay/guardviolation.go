package rulereplay

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// GuardViolation is the guardspec.Violation this record was observed as: the
// same rule, kind, severity, edge and cited message, so a consumer of guard
// verdicts (guardsarif's SARIF, SR-47w) reads a replayed violation exactly as
// it reads a live one.
func (v ViolationRecord) GuardViolation() guardspec.Violation {
	return guardspec.Violation{
		RuleID:   v.RuleID,
		Kind:     guardspec.LayerRuleKind(v.Kind),
		Severity: guardspec.Severity(v.Severity),
		From:     v.From,
		To:       v.To,
		Message:  v.Message,
	}
}

// IntroducedViolations is every violation the replayed range introduced —
// records with New set, oldest commit first, each message naming the commit
// that introduced it. An inherited violation is the same finding carried
// forward, not a second one, so it is left out; a violation fixed and later
// reintroduced appears once per introduction.
func (r Report) IntroducedViolations() []guardspec.Violation {
	var out []guardspec.Violation
	for _, c := range r.Commits {
		for _, v := range c.Violations {
			if !v.New {
				continue
			}
			g := v.GuardViolation()
			g.Message = fmt.Sprintf("%s [introduced at %s: %s]", g.Message, shortSHA(c.SHA), c.Subject)
			out = append(out, g)
		}
	}
	return out
}
