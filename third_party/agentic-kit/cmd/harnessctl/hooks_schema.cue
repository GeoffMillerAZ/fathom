package harnessctl

import "list"

#HookEvaluator: close({profile!: "agent-hooks/spawn-eval/1", executable!: _abs, sha256!: _hash, timeoutMillis!: int & >=1 & <=1000})
#HookPolicy: close({path!: _abs, sha256!: _hash})
#HookProfile: close({
 version!: "harnessctl.spawn-profile/1"
 profile!: "spawn-economics/1"
 issuedAt!: _time
 expiresAt!: _time
 host!: "claude" | "codex"
 hostId!: _id
 projectId!: _id
 principalId!: _id
 requiredControlId!: _id
 advisoryControlId!: _id
 samplingKey!: _hash
 evaluator!: #HookEvaluator
 policy!: #HookPolicy
 maxBoundaryMillis!: int & >=1 & <=5000
 _differentControls: true & (requiredControlId != advisoryControlId)
})

_hookCode: "unsupported" | "input_invalid" | "controller_invalid" | "controller_changed" | "project_unregistered" | "hook_unregistered" | "profile_invalid" | "policy_invalid" | "config_incomplete" | "policy_conflict" | "state_unavailable" | "state_invalid" | "state_busy" | "state_ambiguous" | "clock_invalid" | "rearm_required" | "native_unavailable" | "native_incomplete" | "signature_invalid" | "binding_mismatch" | "storage_incomplete" | "evaluator_unavailable" | "evaluator_incomplete" | "boundary_expired" | "output_incomplete"
_hookDenialCode: "no_model" | "unknown_model" | "tier_ceiling" | "no_effort" | "fork_unmarked" | "fork_denied"
_hookCounters: {
 stateWritesAttempted!: int & >=0 & <=2
 stateWritesCommitted!: int & >=0 & <=stateWritesAttempted
 nativeStampAttempts!: int & >=0 & <=3
 nativeStampInvoked!: int & >=0 & <=nativeStampAttempts
 nativeStampCompleted!: int & >=0 & <=nativeStampInvoked
 evaluatorAttempts!: int & >=0 & <=1
 evaluatorInvoked!: int & >=0 & <=evaluatorAttempts
 evaluatorCompleted!: int & >=0 & <=evaluatorInvoked
}
_hookBase: {
 version!: "harnessctl.spawn-report/1"
 noActionAuthority!: true
 coverage!: "selected-project-spawn-events-only"
 installation!: "unobserved"
 advisorySelected!: bool
 advisoryEvaluated!: bool
 adviceMatches!: int & >=0 & <=32
} & _hookCounters
_hookCompleteCounts: {nativeStampAttempts!: 3, nativeStampInvoked!: 3, nativeStampCompleted!: 3, evaluatorAttempts!: 1, evaluatorInvoked!: 1, evaluatorCompleted!: 1}
#HookReport: (
 close(_hookBase & {status!: "hold", code!: _hookCode, requiredDecision!: "", codes!: [], class!: "", classSource!: "", advisorySelected!: false, advisoryEvaluated!: false, adviceMatches!: 0}) |
 close(_hookBase & _hookCompleteCounts & {status!: "complete", code!: "ok", requiredDecision!: "pass", codes!: ["allowed"], class!: "worker", classSource!: "worker_marker" | "default", advisorySelected!: false, advisoryEvaluated!: false, adviceMatches!: 0}) |
 close(_hookBase & _hookCompleteCounts & {status!: "complete", code!: "ok", requiredDecision!: "pass", codes!: ["allowed"], class!: "critic", classSource!: "critic_marker" | "critic_pattern", advisorySelected!: false, advisoryEvaluated!: false, adviceMatches!: 0}) |
 close(_hookBase & _hookCompleteCounts & {status!: "complete", code!: "ok", requiredDecision!: "pass", codes!: ["allowed"], class!: "worker", classSource!: "worker_marker" | "default", advisorySelected!: true, advisoryEvaluated!: true}) |
 close(_hookBase & _hookCompleteCounts & {status!: "complete", code!: "ok", requiredDecision!: "pass", codes!: ["allowed"], class!: "critic", classSource!: "critic_marker" | "critic_pattern", advisorySelected!: true, advisoryEvaluated!: true}) |
 close(_hookBase & _hookCompleteCounts & {status!: "complete", code!: "spawn_denied", requiredDecision!: "deny", codes!: [..._hookDenialCode] & list.MinItems(1) & list.MaxItems(6) & list.UniqueItems(), class!: "worker", classSource!: "worker_marker" | "default", advisoryEvaluated!: false, adviceMatches!: 0}) |
 close(_hookBase & _hookCompleteCounts & {status!: "complete", code!: "spawn_denied", requiredDecision!: "deny", codes!: [..._hookDenialCode] & list.MinItems(1) & list.MaxItems(6) & list.UniqueItems(), class!: "critic", classSource!: "critic_marker" | "critic_pattern", advisoryEvaluated!: false, adviceMatches!: 0}) |
 close(_hookBase & _hookCompleteCounts & {status!: "not-applicable", code!: "resume_not_spawn", requiredDecision!: "not-applicable", codes!: ["resume_not_spawn"], class!: "not-applicable", classSource!: "not-applicable", advisorySelected!: false, advisoryEvaluated!: false, adviceMatches!: 0}))
