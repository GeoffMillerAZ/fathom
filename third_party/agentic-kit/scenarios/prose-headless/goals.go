package proseheadless

import (
	"errors"
	"fmt"
	"time"

	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	assignmentkit "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/assignment-kit"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

// ErrAssessment identifies a malformed goal-assessment request.
var ErrAssessment = errors.New("proseheadless: invalid goal assessment")

// CriterionObservation is trusted host/human evidence about one goal's single
// acceptance criterion for one document version. It is never model output: the
// host produces it from a deterministic check, an evaluator it configured, or a
// named human. Exposure of a goal to the generator (DocNode manifests) is not an
// observation and cannot substitute for one.
type CriterionObservation struct {
	GoalID          string    `json:"goal_id"`
	SubjectDigest   string    `json:"subject_digest"` // the document version the host observed
	PolicyEpoch     string    `json:"policy_epoch"`   // the policy epoch the host observed under
	Outcome         string    `json:"outcome"`        // pass, fail, unknown, not-applicable
	Method          string    `json:"method"`         // deterministic, heuristic, empirical, human
	Reason          string    `json:"reason"`
	ArtifactDigests []string  `json:"artifact_digests"`
	ObservedAt      time.Time `json:"observed_at"`
	ValidUntil      time.Time `json:"valid_until"`
}

// AssessmentConfig carries host policy. Methods maps a goal's Method string to the
// evidence methods eligible for it; a goal whose Method is unmapped is refused.
// PolicyEpoch pins the policy the assessment is valid under.
type AssessmentConfig struct {
	PolicyEpoch string
	Methods     map[string][]fitness.EvidenceMethod
	Now         func() time.Time
}

// GoalAssessment is Assignment Kit's canonical result for the session goals.
// Complete is true only when Evaluate reports "complete": every independently
// expected required goal has current, subject-bound, method-eligible passing
// evidence. Advisory goals map to stretch goals and never block completion.
type GoalAssessment struct {
	Complete bool
	Status   string
	Result   assignmentkit.Result
}

func criterionDigest(g prose.Goal) string {
	return prose.ContentDigest(g.ID + "\x00" + g.Statement + "\x00" + g.Acceptance + "\x00" + g.Method)
}

// AssessGoals composes Assignment Kit per criterion. expectedRequired is the
// caller-owned scope, independent of the authored session goals: an expected goal
// absent from the session, or a session-required goal absent from the expectation,
// is refused before evaluation. subjectDigest binds evidence to one exact document
// version, so evidence about another version is a foreign subject.
func AssessGoals(cfg AssessmentConfig, s prose.DocumentSession, subjectDigest string, expectedRequired []string, obs []CriterionObservation) (GoalAssessment, error) {
	if cfg.Now == nil || cfg.PolicyEpoch == "" || len(subjectDigest) != 64 {
		return GoalAssessment{}, fmt.Errorf("%w: clock, policy epoch and subject digest required", ErrAssessment)
	}
	now := cfg.Now().UTC()
	policy := prose.ContentDigest("assessment-policy\x00" + cfg.PolicyEpoch)
	a := assignmentkit.Assignment{
		Version: 1, ID: "doc-" + s.ID, Title: "Document goals " + s.ID, PolicyDigest: policy, SubjectDigest: subjectDigest,
		Budgets: []assignmentkit.Budget{{ID: "assessments", Unit: "unit", Limit: 1000}}, Deadline: now.Add(24 * time.Hour),
	}
	goals := map[string]prose.Goal{}
	for _, g := range s.Goals {
		methods := cfg.Methods[g.Method]
		if len(methods) == 0 {
			return GoalAssessment{}, fmt.Errorf("%w: goal %q method %q has no eligible evidence methods", ErrAssessment, g.ID, g.Method)
		}
		kind := "stretch"
		if g.Class == prose.GoalRequired {
			kind = "required"
		}
		goals[g.ID] = g
		a.Goals = append(a.Goals, assignmentkit.Goal{ID: g.ID, Title: g.Statement, Kind: kind, Profile: fitness.Profile{
			Version: 1, ID: g.ID + "-profile", Domain: "prose",
			Criteria: []fitness.Criterion{{ID: "accept", DefinitionDigest: criterionDigest(g), Kind: fitness.CriterionRequired, AllowedMethods: methods}},
		}})
	}
	for _, id := range expectedRequired {
		if g, ok := goals[id]; !ok || g.Class != prose.GoalRequired {
			return GoalAssessment{}, fmt.Errorf("%w: expected required goal %q is not a required session goal", ErrAssessment, id)
		}
	}
	for _, g := range s.Goals {
		if g.Class == prose.GoalRequired && !contains(expectedRequired, g.ID) {
			return GoalAssessment{}, fmt.Errorf("%w: required goal %q is outside the independent expectation", ErrAssessment, g.ID)
		}
	}
	fp, err := assignmentkit.Fingerprint(a)
	if err != nil {
		return GoalAssessment{}, fmt.Errorf("%w: %v", ErrAssessment, err)
	}
	in := assignmentkit.Input{AssignmentDigest: fp, PolicyDigest: policy, Usage: []assignmentkit.Usage{{BudgetID: "assessments", Spent: 0}}, Progress: &assignmentkit.Progress{}}
	byGoal := map[string][]fitness.Observation{}
	for _, o := range obs {
		if o.PolicyEpoch != cfg.PolicyEpoch {
			return GoalAssessment{}, fmt.Errorf("%w: observation policy is stale", ErrAssessment)
		}
		g, ok := goals[o.GoalID]
		if !ok {
			return GoalAssessment{}, fmt.Errorf("%w: observation for unknown goal %q", ErrAssessment, o.GoalID)
		}
		if len(o.SubjectDigest) != 64 {
			return GoalAssessment{}, fmt.Errorf("%w: observation for %q must name its subject digest", ErrAssessment, o.GoalID)
		}
		byGoal[o.GoalID] = append(byGoal[o.GoalID], fitness.Observation{
			CriterionID: "accept", DefinitionDigest: criterionDigest(g), SubjectDigest: o.SubjectDigest,
			Outcome: fitness.FindingOutcome(o.Outcome), Method: fitness.EvidenceMethod(o.Method), Reason: o.Reason, ArtifactDigests: o.ArtifactDigests, ObservedAt: o.ObservedAt, ValidUntil: o.ValidUntil,
		})
	}
	for _, g := range s.Goals {
		if o, ok := byGoal[g.ID]; ok {
			in.Goals = append(in.Goals, assignmentkit.GoalInput{GoalID: g.ID, Observations: o})
		}
	}
	res, err := assignmentkit.Evaluate(a, assignmentkit.Scope{AssignmentDigest: fp, PolicyDigest: policy, RequiredGoalIDs: expectedRequired}, in, now)
	if err != nil {
		return GoalAssessment{}, fmt.Errorf("%w: %v", ErrAssessment, err)
	}
	return GoalAssessment{Complete: res.Status == "complete", Status: res.Status, Result: res}, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
