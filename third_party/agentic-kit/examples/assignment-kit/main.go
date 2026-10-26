// Two synthetic, offline reference workflows. Run: go run ./examples/assignment-kit
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	assignment "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/assignment-kit"
	catalogkit "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
)

const (
	artifact = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	policy   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	subject  = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

var at = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// These are complete synthetic criterion definitions. Their synopsis is part of
// NodeDigest, so changing the rule changes the bound assignment definition.
var recipes = []catalog.AuthoredNode{
	{Slug: "command-input", StableKey: "command.input-bound", Title: "Command input boundary", Synopsis: "Qualify parsed command identity and arguments against the exact submitted proposal; no unreviewed shell expansion or omitted segment.", Source: "synthetic/assignment-kit.cue"},
	{Slug: "command-privacy", StableKey: "command.privacy", Title: "Command privacy boundary", Synopsis: "Show that a fixture command neither reads retained secrets nor exposes sensitive bytes in its proposed output.", Source: "synthetic/assignment-kit.cue"},
	{Slug: "investigation-coverage", StableKey: "investigation.coverage", Title: "Expected question coverage", Synopsis: "Account for every independently named required question, including missing and unresolved questions.", Source: "synthetic/assignment-kit.cue"},
	{Slug: "investigation-support", StableKey: "investigation.source-support", Title: "Source supported claim", Synopsis: "Bind each decisive claim to a retained, in-scope source artifact and its review time.", Source: "synthetic/assignment-kit.cue"},
	{Slug: "investigation-counter", StableKey: "investigation.counterevidence", Title: "Counterevidence review", Synopsis: "Record a bounded search for contradictory evidence and retain the disposition of found conflicts.", Source: "synthetic/assignment-kit.cue"},
	{Slug: "investigation-summary", StableKey: "investigation.summary", Title: "Readable synthesis", Synopsis: "Optional concise summary that does not replace the required source and counterevidence checks.", Source: "synthetic/assignment-kit.cue"},
}

func definitions(nodes []catalog.AuthoredNode) (map[string]string, error) {
	bundle, e := catalogkit.Resolve(catalogkit.Spec{Version: 1, Layers: []catalogkit.Layer{{Name: "synthetic", Nodes: nodes}}}, catalogkit.Policy{})
	if e != nil {
		return nil, e
	}
	out := make(map[string]string, len(nodes))
	for _, n := range nodes {
		exact, e := catalogkit.LookupStableKey(bundle, n.StableKey)
		if e != nil {
			return nil, e
		}
		out[n.StableKey] = catalogkit.NodeDigest(exact)
	}
	return out, nil
}
func requiredProfile(id, domain, definitionDigest string) fitness.Profile {
	return fitness.Profile{Version: 1, ID: id, Domain: domain, Criteria: []fitness.Criterion{{ID: "evidence", DefinitionDigest: definitionDigest, Kind: fitness.CriterionRequired, AllowedMethods: []fitness.EvidenceMethod{fitness.EvidenceMethodDeterministic}}}}
}
func goal(id, title, kind, domain, definitionDigest string) assignment.Goal {
	return assignment.Goal{ID: id, Title: title, Kind: kind, Profile: requiredProfile(id+"-profile", domain, definitionDigest)}
}
func observation(definitionDigest string, outcome fitness.FindingOutcome, reason string) fitness.Observation {
	return fitness.Observation{CriterionID: "evidence", DefinitionDigest: definitionDigest, SubjectDigest: subject, Outcome: outcome, Method: fitness.EvidenceMethodDeterministic, Reason: reason, ArtifactDigests: []string{artifact}, ObservedAt: at.Add(-time.Minute), ValidUntil: at.Add(time.Minute)}
}
func goalInput(id, definitionDigest string, outcome fitness.FindingOutcome, reason string) assignment.GoalInput {
	return assignment.GoalInput{GoalID: id, Observations: []fitness.Observation{observation(definitionDigest, outcome, reason)}}
}
func pinned(a assignment.Assignment, requiredIDs []string, in assignment.Input) (assignment.Assignment, assignment.Scope, assignment.Input, error) {
	fp, e := assignment.Fingerprint(a)
	if e != nil {
		return assignment.Assignment{}, assignment.Scope{}, assignment.Input{}, e
	}
	s := assignment.Scope{AssignmentDigest: fp, PolicyDigest: policy, RequiredGoalIDs: append([]string{}, requiredIDs...)}
	in.AssignmentDigest = fp
	in.PolicyDigest = policy
	return a, s, in, nil
}

func commandQualification(defs map[string]string) (assignment.Assignment, assignment.Scope, assignment.Input, error) {
	a := assignment.Assignment{Version: 1, ID: "command-qualification", Title: "Qualify one offline command proposal", IntentRefs: []assignment.Reference{{ID: "reviewed-brief", RevisionDigest: artifact}}, PolicyDigest: policy, SubjectDigest: subject, Goals: []assignment.Goal{
		goal("input-bound", "Input and argument boundary", "required", "command", defs["command.input-bound"]),
		goal("privacy", "Sensitive data boundary", "required", "command", defs["command.privacy"]),
	}, Budgets: []assignment.Budget{{ID: "qualifications", Unit: "attempts", Limit: 3}}, Deadline: at.Add(time.Hour), Rules: []assignment.Rule{{ID: "repeated-ambiguity", Counter: "ambiguities", Threshold: 2, Action: "escalate", Route: "human-review", Reason: "Proposal meaning remains ambiguous"}}}
	in := assignment.Input{Goals: []assignment.GoalInput{
		goalInput("input-bound", defs["command.input-bound"], fitness.FindingPass, "Synthetic parsed proposal matched exact fixture"),
		goalInput("privacy", defs["command.privacy"], fitness.FindingPass, "Synthetic fixture contains no sensitive read or output"),
	}, Usage: []assignment.Usage{{BudgetID: "qualifications", Spent: 1}}, Progress: &assignment.Progress{Attempts: 1}}
	// Receiver-owned expected roster, written independently of a.Goals.
	return pinned(a, []string{"input-bound", "privacy"}, in)
}

func resolvedConcern() assignment.ConcernInput {
	return assignment.ConcernInput{ID: "source-ambiguity", State: "resolved", Reason: "Synthetic source conflict reconciled", ArtifactDigests: []string{artifact}, ObservedAt: at.Add(-time.Minute), ValidUntil: at.Add(time.Minute)}
}
func investigation(defs map[string]string) (assignment.Assignment, assignment.Scope, assignment.Input, error) {
	a := assignment.Assignment{Version: 1, ID: "bounded-investigation", Title: "Investigate a bounded question set", IntentRefs: []assignment.Reference{{ID: "reviewed-question-set", RevisionDigest: artifact}}, PolicyDigest: policy, SubjectDigest: subject, Goals: []assignment.Goal{
		goal("coverage", "Expected question coverage", "required", "investigation", defs["investigation.coverage"]),
		goal("source-support", "Source support for decisive claims", "required", "investigation", defs["investigation.source-support"]),
		goal("counterevidence", "Counterevidence disposition", "required", "investigation", defs["investigation.counterevidence"]),
		goal("summary", "Optional readable synthesis", "stretch", "investigation", defs["investigation.summary"]),
	}, Concerns: []assignment.Concern{{ID: "source-ambiguity", Description: "Conflicting sources require a retained resolution", Severity: "blocking"}}, Budgets: []assignment.Budget{{ID: "searches", Unit: "attempts", Limit: 3}}, Deadline: at.Add(time.Hour), Rules: []assignment.Rule{
		{ID: "stalled", Counter: "no_progress", Threshold: 2, Action: "pivot", Route: "revise-search", Reason: "No accepted evidence after repeated searches"},
		{ID: "ambiguous", Counter: "ambiguities", Threshold: 2, Action: "escalate", Route: "reviewer", Reason: "Sources remain ambiguous"},
	}}
	in := assignment.Input{Goals: []assignment.GoalInput{
		goalInput("coverage", defs["investigation.coverage"], fitness.FindingPass, "Synthetic expected roster accounted for"),
		goalInput("source-support", defs["investigation.source-support"], fitness.FindingPass, "Synthetic claim has retained source fixture"),
		goalInput("counterevidence", defs["investigation.counterevidence"], fitness.FindingPass, "Synthetic contradictory source check completed"),
		goalInput("summary", defs["investigation.summary"], fitness.FindingFail, "Synthetic summary needs revision"),
	}, Concerns: []assignment.ConcernInput{resolvedConcern()}, Usage: []assignment.Usage{{BudgetID: "searches", Spent: 1}}, Progress: &assignment.Progress{Attempts: 1}}
	// Receiver-owned expected roster, written independently of a.Goals.
	return pinned(a, []string{"coverage", "source-support", "counterevidence"}, in)
}

func snapshots() (map[string]assignment.Result, error) {
	defs, e := definitions(recipes)
	if e != nil {
		return nil, e
	}
	out := map[string]assignment.Result{}
	put := func(name string, a assignment.Assignment, s assignment.Scope, in assignment.Input) error {
		r, e := assignment.Evaluate(a, s, in, at)
		if e != nil {
			return fmt.Errorf("%s: %w", name, e)
		}
		out[name] = r
		return nil
	}
	ca, cs, ci, e := commandQualification(defs)
	if e != nil {
		return nil, e
	}
	if e = put("command/complete", ca, cs, ci); e != nil {
		return nil, e
	}
	ci.Goals = ci.Goals[:1]
	if e = put("command/missing-required-unknown", ca, cs, ci); e != nil {
		return nil, e
	}
	ia, is, ii, e := investigation(defs)
	if e != nil {
		return nil, e
	}
	if e = put("investigation/complete-stretch-failed", ia, is, ii); e != nil {
		return nil, e
	}
	ii.Goals[3].Observations[0].Outcome = fitness.FindingPass
	ii.Goals[3].Observations[0].Reason = "Synthetic summary revised"
	ii.Concerns[0].State = "open"
	ii.Concerns[0].Reason = "Conflicting sources unresolved"
	if e = put("investigation/unresolved-ambiguity", ia, is, ii); e != nil {
		return nil, e
	}
	ii.Concerns[0] = resolvedConcern()
	if e = put("investigation/resolved-complete", ia, is, ii); e != nil {
		return nil, e
	}
	ii.Progress.NoProgress = 2
	if e = put("investigation/no-progress-pivot", ia, is, ii); e != nil {
		return nil, e
	}
	ii.Progress.NoProgress = 0
	ii.Progress.Ambiguities = 2
	if e = put("investigation/ambiguity-escalation", ia, is, ii); e != nil {
		return nil, e
	}
	ii.Progress.Ambiguities = 0
	ii.Usage[0].Spent = 3
	if e = put("investigation/exhausted-budget-stop", ia, is, ii); e != nil {
		return nil, e
	}
	return out, nil
}
func main() {
	out, e := snapshots()
	if e != nil {
		fail(e)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if e := enc.Encode(struct {
		Synthetic     bool                         `json:"synthetic"`
		EvidenceClaim string                       `json:"evidence_claim"`
		Snapshots     map[string]assignment.Result `json:"snapshots"`
	}{true, "Offline fixtures only; no production evidence or action authority", out}); e != nil {
		fail(e)
	}
}
func fail(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
