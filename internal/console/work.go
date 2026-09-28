package console

// work.go — the work-intelligence page projections (docs/design/
// work-intelligence.md §3): one entry point per page section, so the
// console and any CLI verb that shows the same thing call the same
// projection (P10).

// WorkModel is U1, U2 and U5 over one selection's proven changes.
type WorkModel struct {
	Changes      int               `json:"proven_changes"`
	CostPerZone  []ZoneCost        `json:"cost_per_zone"`
	FirstAttempt FirstAttemptModel `json:"first_attempt"`
	Interest     InterestModel     `json:"interest_rate"`
	Scorecard    ScorecardModel    `json:"scorecard"`
}

// WorkIntelligence projects proven changes (EffortPerProvenChange) and
// the CI runs bound to their commits into U1, U2 and U5, the scorecard
// behind G2's causal gate.
func WorkIntelligence(changes []ProvenChange, ci []CIRun) WorkModel {
	return WorkModel{
		Changes:      len(changes),
		CostPerZone:  costPerZone(changes, ci),
		FirstAttempt: firstAttemptSplit(changes),
		Interest:     InterestRate(changes),
		Scorecard:    scorecardCausalGate(ModelScorecard(changes)),
	}
}

// FrictionView is U3: the map and its remedies, proposals only.
type FrictionView struct {
	Map      FrictionMap `json:"map"`
	Remedies []Remedy    `json:"remedies"`
}

// FrictionWithRemedies counts the friction signals and computes the
// pattern table's proposals over them.
func FrictionWithRemedies(in FrictionInputs, rem RemedyInputs) FrictionView {
	m := FrictionSignals(in)
	r := frictionRemedies(m, rem)
	if r == nil {
		r = []Remedy{}
	}
	return FrictionView{Map: m, Remedies: r}
}
