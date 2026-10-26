package assignmentkit

import (
	fitness "github.com/geoffmilleraz/agentic-kit/foundations/quality/fitness-function-registry:fitnessfunctionregistry"
	"list"
	"time"
)

#ID:               fitness.#AssessmentID
#Digest:           fitness.#AssessmentDigest
#Text:             string & =~"[^\\t\\n\\v\\f\\r\\p{Z}\\x{0085}]"
#Time:             time.Time & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$" & !~"^0000-" & !~"^0001-01-01T00:00:00(\\.0+)?Z$"
#NonnegativeInt64: int & >=0 & <=9223372036854775807
#PositiveInt64:    int & >0 & <=9223372036854775807

// Keep assessment law in its foundation; assignment adds only its own stricter
// temporal/text envelope and independently required goal composition.
#Method:    fitness.#EvidenceMethod
#Outcome:   fitness.#FindingOutcome
#Criterion: fitness.#Criterion
#Profile: fitness.#Profile & {
	criteria: [...#Criterion]
	_criterionIDs: [for c in criteria {c.id}] & list.UniqueItems()
	_requiredCriteria: [for c in criteria if c.kind == "required" {c.id}] & list.MinItems(1)
}
#Observation: fitness.#Observation & {
	reason:      #Text
	observed_at: #Time
	valid_until: #Time
}
#ConfidenceBar: {criterion_id: #ID, minimum: number & >0 & <=1, basis_digest: #Digest, site_digest: #Digest}
#Reference: {id: #ID, revision_digest: #Digest}
#Goal: {id: #ID, title: #Text, if len(title) > 2048 {title: ""}, kind: "required" | "stretch", depends_on: null | (list.MaxItems(64) & list.UniqueItems() & [...#ID]), profile: #Profile, confidence: null | (list.MaxItems(256) & [...#ConfidenceBar]), if confidence != null {_confidenceIDs: [for b in confidence {b.criterion_id}] & list.UniqueItems()}}
#Concern: {id: #ID, description: #Text, if len(description) > 2048 {description: ""}, severity: "blocking" | "advisory"}
#Budget: {id: #ID, unit: #ID, limit: #PositiveInt64}
#Rule: {id: #ID, counter: "attempts" | "no_progress" | "ambiguities" | "conflicts", threshold: #PositiveInt64, action: "escalate" | "pivot" | "stop", route: #ID, reason: #Text, if len(reason) > 2048 {reason: ""}}
#Assignment: {
	version: 1
	id:      #ID
	title:   #Text
	if len(title) > 2048 {title: ""}
	intent_refs: null | (list.MaxItems(16) & [...#Reference])
	policy_digest:  #Digest
	subject_digest: #Digest
	goals: (list.MaxItems(64) & [#Goal, ...#Goal])
	concerns: null | (list.MaxItems(64) & [...#Concern])
	budgets: (list.MaxItems(16) & [#Budget, ...#Budget])
	deadline: #Time
	rules: null | (list.MaxItems(32) & [...#Rule])
	_goalIDs: [for g in goals {g.id}] & list.UniqueItems()
	_aggregateCriteria: [for g in goals for c in g.profile.criteria {c}] & list.MaxItems(256)
	_budgetIDs: [for b in budgets {b.id}] & list.UniqueItems()
	if intent_refs != null {_intentIDs: [for r in intent_refs {r.id}] & list.UniqueItems()}
	if concerns != null {_concernIDs: [for c in concerns {c.id}] & list.UniqueItems()}
	if rules != null {_ruleIDs: [for r in rules {r.id}] & list.UniqueItems()}
}
#Scope: {assignment_digest: #Digest, policy_digest: #Digest, required_goal_ids: (list.MaxItems(64) & list.UniqueItems() & [#ID, ...#ID])}
#ConfidenceReading: {criterion_id: #ID, observation_digest: #Digest, basis_digest: #Digest, site_digest: #Digest, value: number & >=0 & <=1}
#GoalInput: {goal_id: #ID, observations: null | (list.MaxItems(256) & [...#Observation]), readings: null | (list.MaxItems(256) & [...#ConfidenceReading]), if observations != null {_observationIDs: [for o in observations {o.criterion_id}] & list.UniqueItems()}, if readings != null {_readingIDs: [for r in readings {r.criterion_id}] & list.UniqueItems()}}
#ConcernInput: {id: #ID, state: "open" | "resolved" | "unknown", reason: #Text, if len(reason) > 2048 {reason: ""}, artifact_digests: null | (list.MaxItems(16) & list.UniqueItems() & [...#Digest]), observed_at: #Time, valid_until: #Time}
#Usage: {budget_id: #ID, spent: #NonnegativeInt64}
#Progress: {attempts: #NonnegativeInt64, no_progress: #NonnegativeInt64, ambiguities: #NonnegativeInt64, conflicts: #NonnegativeInt64}
#Input: {assignment_digest: #Digest, policy_digest: #Digest, goals: null | (list.MaxItems(64) & [...#GoalInput]), concerns: null | (list.MaxItems(64) & [...#ConcernInput]), usage: null | (list.MaxItems(16) & [...#Usage]), progress: #Progress | null, if goals != null {_goalInputIDs: [for g in goals {g.goal_id}] & list.UniqueItems()}, if concerns != null {_concernInputIDs: [for c in concerns {c.id}] & list.UniqueItems()}, if usage != null {_usageIDs: [for u in usage {u.budget_id}] & list.UniqueItems()}}
#AssessmentTrace: fitness.#AssessmentTrace
#Assessment: fitness.#Assessment & {
	evaluated_at: #Time
}
#ConfidenceResult: {criterion_id: #ID, status: "pass" | "unknown" | "not-applicable", reason: string, observation_digest: "" | #Digest, basis_digest: #Digest, site_digest: #Digest}
#GoalResult: {id: #ID, kind: "required" | "stretch", satisfied: bool, blocked_dependencies: (list.MaxItems(64) & [...#ID]), assessment: #Assessment, confidence: null | (list.MaxItems(256) & [...#ConfidenceResult]), accepted_soft_score: number & >=0 & <=1 | null}
#ConcernResult: {id: #ID, severity: "blocking" | "advisory", state: "open" | "resolved" | "unknown", reason: string, artifact_digests: (list.MaxItems(16) & [...#Digest])}
#BudgetResult: {id: #ID, unit: #ID, limit: #PositiveInt64, spent: #NonnegativeInt64 | null, exhausted: bool}
#Cause: {kind: string & !="", id: #ID, detail: string}
#Result: {
	trust:             string & !=""
	action_authority:  false
	assignment_digest: #Digest
	policy_digest:     #Digest
	scope_digest:      #Digest
	evidence_digest:   #Digest
	evaluated_at:      #Time
	status:            "stopped" | "pivot-required" | "needs-review" | "blocked" | "complete" | "continue"
	goals: (list.MaxItems(64) & [...#GoalResult])
	concerns: (list.MaxItems(64) & [...#ConcernResult])
	budgets: (list.MaxItems(16) & [...#BudgetResult])
	matched_rules: (list.MaxItems(32) & [...#ID])
	causes: (list.MaxItems(256) & [...#Cause])
}
