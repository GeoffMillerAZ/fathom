package honestygate

import "fmt"

// defaults applied when Gate fields are zero-valued.
const (
	defaultMinEvidenceAllowed   = 2
	defaultMinEvidenceHedge     = 1
	defaultMinConfidenceAllowed = 0.7
)

// Evaluate applies deterministic honesty rules to the claim and returns a
// Decision. The rules, in priority order:
//
//  1. EvidenceCount < minEvidenceHedge  → Block (insufficient evidence).
//  2. EvidenceCount >= minEvidenceAllowed AND Confidence >= minConfidenceAllowed → Allowed.
//  3. EvidenceCount >= minEvidenceAllowed (but confidence below threshold)       → Hedge.
//  4. Otherwise (evidence in range [minEvidenceHedge, minEvidenceAllowed))       → Hedge.
func (g Gate) Evaluate(c Claim) Decision {
	minAllowed := g.MinEvidenceAllowed
	if minAllowed <= 0 {
		minAllowed = defaultMinEvidenceAllowed
	}
	minHedge := g.MinEvidenceHedge
	if minHedge <= 0 {
		minHedge = defaultMinEvidenceHedge
	}
	minConf := g.MinConfidenceAllowed
	if minConf <= 0 {
		minConf = defaultMinConfidenceAllowed
	}

	ev := c.EvidenceCount
	conf := c.Confidence

	switch {
	case ev < minHedge:
		return Decision{
			Verdict: VerdictBlock,
			Reason: fmt.Sprintf(
				"claim has %d supporting evidence items; minimum for publication is %d",
				ev, minHedge,
			),
		}
	case ev >= minAllowed && conf >= minConf:
		return Decision{
			Verdict: VerdictAllowed,
			Reason: fmt.Sprintf(
				"claim is supported by %d evidence items with confidence %.2f",
				ev, conf,
			),
		}
	case ev >= minAllowed:
		return Decision{
			Verdict: VerdictHedge,
			Reason: fmt.Sprintf(
				"claim has %d evidence items but confidence %.2f is below %.2f; qualify with uncertainty language",
				ev, conf, minConf,
			),
		}
	default:
		return Decision{
			Verdict: VerdictHedge,
			Reason: fmt.Sprintf(
				"claim has %d evidence items (below required %d for allowed); qualify with uncertainty language",
				ev, minAllowed,
			),
		}
	}
}
