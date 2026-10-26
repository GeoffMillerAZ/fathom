package assignmentkit

import (
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	"math"
	"sort"
	"time"
)

// Evaluate projects one pinned snapshot. It neither authenticates evidence nor
// reserves resources, executes a route, or authorizes an effect.
func Evaluate(a Assignment, scope Scope, in Input, now time.Time) (Result, error) {
	ad, e := Fingerprint(a)
	if e != nil {
		return Result{}, e
	}
	if !validTime(now) {
		return Result{}, invalid("now must be canonical UTC")
	}
	if len(scope.RequiredGoalIDs) == 0 || len(scope.RequiredGoalIDs) > 64 || !digest(scope.AssignmentDigest) || !digest(scope.PolicyDigest) || scope.AssignmentDigest != ad || scope.PolicyDigest != a.PolicyDigest || !uniqueIDs(scope.RequiredGoalIDs) {
		return Result{}, invalid("scope pins or required IDs")
	}
	required := map[string]bool{}
	for _, g := range a.Goals {
		if g.Kind == "required" {
			required[g.ID] = true
		}
	}
	if len(required) != len(scope.RequiredGoalIDs) {
		return Result{}, invalid("scope omits required goals")
	}
	for _, x := range scope.RequiredGoalIDs {
		if !required[x] {
			return Result{}, invalid("scope has foreign required goal")
		}
	}
	if in.AssignmentDigest != ad || in.PolicyDigest != a.PolicyDigest {
		return Result{}, invalid("input pins changed")
	}
	gd := map[string]Goal{}
	for _, g := range a.Goals {
		gd[g.ID] = g
	}
	inputs := map[string]GoalInput{}
	if len(in.Goals) > 64 {
		return Result{}, invalid("too many goal inputs")
	}
	for _, gi := range in.Goals {
		if !id(gi.GoalID) {
			return Result{}, invalid("invalid goal input id")
		}
		if _, ok := gd[gi.GoalID]; !ok || inputs[gi.GoalID].GoalID != "" {
			return Result{}, invalid("foreign or duplicate goal input")
		}
		inputs[gi.GoalID] = gi
	}
	cd := map[string]Concern{}
	for _, c := range a.Concerns {
		cd[c.ID] = c
	}
	ci := map[string]ConcernInput{}
	if len(in.Concerns) > 64 {
		return Result{}, invalid("too many concern inputs")
	}
	for _, c := range in.Concerns {
		if !id(c.ID) {
			return Result{}, invalid("invalid concern input id")
		}
		if _, ok := cd[c.ID]; !ok || ci[c.ID].ID != "" || !validConcernInput(c) {
			return Result{}, invalid("foreign, duplicate, or malformed concern input")
		}
		ci[c.ID] = c
	}
	bd := map[string]Budget{}
	for _, b := range a.Budgets {
		bd[b.ID] = b
	}
	usage := map[string]int64{}
	if len(in.Usage) > 16 {
		return Result{}, invalid("too many usage inputs")
	}
	for _, u := range in.Usage {
		if !id(u.BudgetID) {
			return Result{}, invalid("invalid budget input id")
		}
		if _, ok := bd[u.BudgetID]; !ok || u.Spent < 0 {
			return Result{}, invalid("foreign or malformed usage")
		}
		if _, dupe := usage[u.BudgetID]; dupe {
			return Result{}, invalid("duplicate usage")
		}
		usage[u.BudgetID] = u.Spent
	}
	if in.Progress != nil && (in.Progress.Attempts < 0 || in.Progress.NoProgress < 0 || in.Progress.Ambiguities < 0 || in.Progress.Conflicts < 0) {
		return Result{}, invalid("negative progress")
	}
	sd, e := hash("agentic-kit/assignment-scope/v1", canonicalScope(scope))
	if e != nil {
		return Result{}, e
	}
	out := Result{Trust: "caller-declared; evidence truth and calibration unproven", ActionAuthority: false, AssignmentDigest: ad, PolicyDigest: a.PolicyDigest, ScopeDigest: sd, EvaluatedAt: now, Goals: []GoalResult{}, Concerns: []ConcernResult{}, Budgets: []BudgetResult{}, MatchedRules: []string{}, Causes: []Cause{}}
	goals := append([]Goal{}, a.Goals...)
	sort.Slice(goals, func(i, j int) bool { return goals[i].ID < goals[j].ID })
	allRequired := true
	confidenceReady := map[string]bool{}
	for _, g := range goals {
		gi := inputs[g.ID]
		if len(gi.Observations) > len(g.Profile.Criteria) {
			return Result{}, invalid("too many observations for %q", g.ID)
		}
		obs := gi.Observations
		byObs := map[string]fitness.Observation{}
		for _, o := range obs {
			if _, dup := byObs[o.CriterionID]; dup {
				return Result{}, invalid("duplicate observation")
			}
			byObs[o.CriterionID] = o
		}
		bars := map[string]ConfidenceBar{}
		for _, b := range g.Confidence {
			bars[b.CriterionID] = b
		}
		readings := map[string]ConfidenceReading{}
		if len(gi.Readings) > len(bars) {
			return Result{}, invalid("too many readings")
		}
		for _, r := range gi.Readings {
			if _, ok := bars[r.CriterionID]; !ok || readings[r.CriterionID].CriterionID != "" || !digest(r.ObservationDigest) || !digest(r.BasisDigest) || !digest(r.SiteDigest) || math.IsNaN(r.Value) || math.IsInf(r.Value, 0) || r.Value < 0 || r.Value > 1 {
				return Result{}, invalid("foreign, duplicate or malformed reading")
			}
			if o, exists := byObs[r.CriterionID]; !exists || o.Method != fitness.EvidenceMethodHeuristic {
				return Result{}, invalid("reading without heuristic observation")
			}
			readings[r.CriterionID] = r
		}
		assessment, err := fitness.AssessProfile(g.Profile, a.SubjectDigest, obs, now)
		if err != nil {
			return Result{}, invalid("goal %q: %v", g.ID, err)
		}
		gr := GoalResult{ID: g.ID, Kind: g.Kind, Assessment: assessment, BlockedDependencies: []string{}, Confidence: []ConfidenceResult{}}
		confidenceReady[g.ID] = true
		scoreQualified := true
		for _, c := range g.Profile.Criteria {
			bar, has := bars[c.ID]
			if !has {
				continue
			}
			cr := ConfidenceResult{CriterionID: c.ID, Status: "not-applicable", Reason: "no heuristic observation", BasisDigest: bar.BasisDigest, SiteDigest: bar.SiteDigest}
			o, exists := byObs[c.ID]
			if exists && o.Method == fitness.EvidenceMethodHeuristic {
				od, err := ObservationFingerprint(o)
				if err != nil {
					return Result{}, err
				}
				cr.ObservationDigest = od
				r, found := readings[c.ID]
				if !found || r.ObservationDigest != od || r.BasisDigest != bar.BasisDigest || r.SiteDigest != bar.SiteDigest || r.Value < bar.Minimum {
					cr.Status = "unknown"
					cr.Reason = "heuristic confidence binding missing, mismatched or below calibrated minimum"
					if c.Kind == fitness.CriterionRequired {
						confidenceReady[g.ID] = false
					} else {
						scoreQualified = false
					}
				} else {
					cr.Status = "pass"
					cr.Reason = "declared reading meets bound calibration bar"
				}
			}
			gr.Confidence = append(gr.Confidence, cr)
		}
		sort.Slice(gr.Confidence, func(i, j int) bool { return gr.Confidence[i].CriterionID < gr.Confidence[j].CriterionID })
		if scoreQualified && assessment.SoftScore != nil {
			v := *assessment.SoftScore
			gr.AcceptedSoftScore = &v
		}
		out.Goals = append(out.Goals, gr)
	}
	// Evaluate dependencies after all base results. Cycles have already been rejected.
	byResult := map[string]*GoalResult{}
	for i := range out.Goals {
		byResult[out.Goals[i].ID] = &out.Goals[i]
	}
	var satisfied func(string) bool
	satisfied = func(gid string) bool {
		r := byResult[gid]
		if r.Satisfied {
			return true
		}
		if r.Assessment.RequiredStatus != fitness.RequiredPass || !confidenceReady[gid] {
			return false
		}
		for _, dep := range gd[gid].DependsOn {
			if !satisfied(dep) {
				return false
			}
		}
		r.Satisfied = true
		return true
	}
	for _, g := range goals {
		r := byResult[g.ID]
		for _, dep := range g.DependsOn {
			if !satisfied(dep) {
				r.BlockedDependencies = append(r.BlockedDependencies, dep)
			}
		}
		sort.Strings(r.BlockedDependencies)
		if satisfied(g.ID) == false && g.Kind == "required" {
			allRequired = false
			out.Causes = append(out.Causes, Cause{"required-goal", g.ID, goalCause(*r)})
		}
	}
	blocking := false
	for _, c := range a.Concerns {
		v, ok := ci[c.ID]
		state, reason := "unknown", "no concern input"
		artifacts := []string{}
		if ok {
			state, reason = v.State, v.Reason
			artifacts = append(artifacts, v.ArtifactDigests...)
			sort.Strings(artifacts)
			if state == "resolved" && (v.ObservedAt.After(now) || !now.Before(v.ValidUntil) || c.Severity == "blocking" && len(artifacts) == 0) {
				state, reason = "unknown", "resolution is future, expired, or lacks retained artifact"
			}
		}
		out.Concerns = append(out.Concerns, ConcernResult{c.ID, c.Severity, state, reason, artifacts})
		if state != "resolved" {
			out.Causes = append(out.Causes, Cause{"concern", c.ID, state})
			if c.Severity == "blocking" {
				blocking = true
			}
		}
	}
	sort.Slice(out.Concerns, func(i, j int) bool { return out.Concerns[i].ID < out.Concerns[j].ID })
	stopped, pivot, escalate := false, false, false
	for _, b := range a.Budgets {
		br := BudgetResult{ID: b.ID, Unit: b.Unit, Limit: b.Limit}
		if spent, ok := usage[b.ID]; ok {
			br.Spent = &spent
			br.Exhausted = spent >= b.Limit
			if br.Exhausted {
				stopped = true
				out.Causes = append(out.Causes, Cause{"budget-exhausted", b.ID, "spent reached limit"})
			}
		} else {
			blocking = true
			out.Causes = append(out.Causes, Cause{"usage-missing", b.ID, "usage not reported"})
		}
		out.Budgets = append(out.Budgets, br)
	}
	sort.Slice(out.Budgets, func(i, j int) bool { return out.Budgets[i].ID < out.Budgets[j].ID })
	if !now.Before(a.Deadline) {
		stopped = true
		out.Causes = append(out.Causes, Cause{"deadline", a.ID, "deadline reached"})
	}
	if in.Progress == nil {
		blocking = true
		out.Causes = append(out.Causes, Cause{"progress-missing", a.ID, "progress not reported"})
	} else {
		for _, r := range a.Rules {
			var v int64
			switch r.Counter {
			case "attempts":
				v = in.Progress.Attempts
			case "no_progress":
				v = in.Progress.NoProgress
			case "ambiguities":
				v = in.Progress.Ambiguities
			case "conflicts":
				v = in.Progress.Conflicts
			}
			if v >= r.Threshold {
				out.MatchedRules = append(out.MatchedRules, r.ID)
				out.Causes = append(out.Causes, Cause{"rule-" + r.Action, r.ID, r.Reason + "; route=" + r.Route})
				switch r.Action {
				case "stop":
					stopped = true
				case "pivot":
					pivot = true
				case "escalate":
					escalate = true
				}
			}
		}
	}
	sort.Strings(out.MatchedRules)
	sort.Slice(out.Causes, func(i, j int) bool {
		if out.Causes[i].Kind == out.Causes[j].Kind {
			return out.Causes[i].ID < out.Causes[j].ID
		}
		return out.Causes[i].Kind < out.Causes[j].Kind
	})
	ed, e := hash("agentic-kit/assignment-evidence/v1", canonicalInput(in))
	if e != nil {
		return Result{}, e
	}
	out.EvidenceDigest = ed
	switch {
	case stopped:
		out.Status = "stopped"
	case pivot:
		out.Status = "pivot-required"
	case escalate:
		out.Status = "needs-review"
	case blocking:
		out.Status = "blocked"
	case allRequired:
		out.Status = "complete"
	default:
		out.Status = "continue"
	}
	return out, nil
}
func validConcernInput(c ConcernInput) bool {
	if !id(c.ID) || (c.State != "open" && c.State != "resolved" && c.State != "unknown") || !textValid(c.Reason) || !validTime(c.ObservedAt) || !validTime(c.ValidUntil) || !c.ValidUntil.After(c.ObservedAt) || len(c.ArtifactDigests) > 16 {
		return false
	}
	seen := map[string]bool{}
	for _, d := range c.ArtifactDigests {
		if !digest(d) || seen[d] {
			return false
		}
		seen[d] = true
	}
	return true
}
func canonicalScope(s Scope) Scope {
	s.RequiredGoalIDs = append([]string{}, s.RequiredGoalIDs...)
	sort.Strings(s.RequiredGoalIDs)
	return s
}
func canonicalInput(in Input) Input {
	in.Goals = append([]GoalInput{}, in.Goals...)
	for i := range in.Goals {
		g := &in.Goals[i]
		g.Observations = append([]fitness.Observation{}, g.Observations...)
		for j := range g.Observations {
			g.Observations[j].ArtifactDigests = append([]string{}, g.Observations[j].ArtifactDigests...)
			sort.Strings(g.Observations[j].ArtifactDigests)
		}
		sort.Slice(g.Observations, func(i, j int) bool { return g.Observations[i].CriterionID < g.Observations[j].CriterionID })
		g.Readings = append([]ConfidenceReading{}, g.Readings...)
		sort.Slice(g.Readings, func(i, j int) bool { return g.Readings[i].CriterionID < g.Readings[j].CriterionID })
	}
	sort.Slice(in.Goals, func(i, j int) bool { return in.Goals[i].GoalID < in.Goals[j].GoalID })
	in.Concerns = append([]ConcernInput{}, in.Concerns...)
	for i := range in.Concerns {
		in.Concerns[i].ArtifactDigests = append([]string{}, in.Concerns[i].ArtifactDigests...)
		sort.Strings(in.Concerns[i].ArtifactDigests)
	}
	sort.Slice(in.Concerns, func(i, j int) bool { return in.Concerns[i].ID < in.Concerns[j].ID })
	in.Usage = append([]Usage{}, in.Usage...)
	sort.Slice(in.Usage, func(i, j int) bool { return in.Usage[i].BudgetID < in.Usage[j].BudgetID })
	if in.Progress != nil {
		p := *in.Progress
		in.Progress = &p
	}
	return in
}

func goalCause(r GoalResult) string {
	for _, c := range r.Confidence {
		if c.Status == "unknown" {
			return "confidence-unqualified"
		}
	}
	if len(r.BlockedDependencies) > 0 {
		return "dependency-unsatisfied"
	}
	return string(r.Assessment.RequiredStatus)
}
