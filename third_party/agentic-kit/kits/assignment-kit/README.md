# Assignment Kit

Experimental pure assessment for a bounded, versioned assignment. `schema.cue` owns shape; Go checks graph, identity and assessment behavior. Run `go run ./examples/assignment-kit` for plain JSON reports from two synthetic offline workflows.

Go and CUE both reuse the fitness-function-registry's canonical profile, observation
and assessment contracts. Assignment Kit adds its own goal composition, canonical
time/text envelope and identity floors; it does not maintain a second assessment schema.

## Smallest useful assignment

One required goal with one required `fitnessfunctionregistry.Criterion`, a finite budget and a UTC deadline is enough. Supply an independently written `Scope` with the expected required goal ID, assignment fingerprint and policy digest. Pass a complete `Input` with the same pins, one observation, exact budget usage and progress counters to `Evaluate(a, scope, input, now)`.

```go
fingerprint, err := assignmentkit.Fingerprint(a)
if err != nil { return err }
scope := assignmentkit.Scope{AssignmentDigest: fingerprint, PolicyDigest: a.PolicyDigest, RequiredGoalIDs: []string{"accept"}}
input.AssignmentDigest, input.PolicyDigest = fingerprint, a.PolicyDigest
report, err := assignmentkit.Evaluate(a, scope, input, now) // now is injected canonical UTC
```

`Scope.RequiredGoalIDs` must come from receiver policy, not from enumerating the assignment under test. That independent roster catches an omitted required goal. Keep the old assignment and report receipts when revising a brief; review the new fingerprint before more work.

## Advanced composition

A required goal may depend on another required goal. Stretch goals are visible without blocking required completion. Criteria use the existing profile evaluator, so failed requirements cannot be offset by scores. Heuristic confidence bars require an externally qualified calibration basis and site digest; readings bind the exact canonical observation digest from `ObservationFingerprint`. A raw model score or an unverified basis does not establish reliability. `GoalResult.Assessment` is the unchanged foundation assessment of original observations; `GoalResult.Confidence` is a separate acceptance overlay. An unknown required bar prevents `Satisfied`, and `AcceptedSoftScore` is nil when a scored bar is unknown. Concerns can block completion; budgets and deadlines stop; threshold rules recommend escalation, pivot or stop. `stopped` takes precedence over `pivot-required`, `needs-review`, `blocked`, `complete` and `continue`; every matched cause remains in the report.

| Evidence or state | Result |
|---|---|
| Missing, stale, future, wrong method, wrong subject or definition | Criterion unknown |
| All required criteria N/A or a failed dependency | Goal unsatisfied |
| Missing or mismatched calibrated reading on heuristic evidence | Criterion unknown |
| Open, unknown, expired or unsubstantiated blocking concern | Completion blocked |
| Missing usage or progress | Completion blocked |
| Exhausted budget, expired deadline or stop rule | Stopped, never success |

## Adoption boundary

1. Independently author expected required scope and authenticate policy, source, artifacts, immutable intent-reference revisions and calibration outside this kit. References contain a label and SHA-256 revision digest, not a grant.
2. Validate and fingerprint each assignment revision. Store append-only assignment, input and report receipts under the application's access and retention policy.
3. Reserve aggregate delegated, failed and retried spend **before** effects. The kit sees only caller-reported usage and cannot enforce a shared budget.
4. Treat `Status == "complete"` as declared acceptance for this snapshot. The report grants no permission, verifies no artifact content, authenticates no source, and performs no stop, pivot or human contact.
5. Qualify catalog definitions through `catalogkit.Resolve` and exact `LookupStableKey`; bind the complete `NodeDigest` in criterion definitions. A moved or changed definition needs a newly reviewed assignment.

The executable examples provide command-safety qualification and bounded investigation without a provider, storage, harness, network, Fathom or hidden configuration. They are synthetic references, not measured consumer adoption. This kit does not infer all requirements, execute a workflow, allocate child budgets, verify parent-child grants or authenticate delegation.
