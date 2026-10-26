# Use Assignment Kit

Use an assignment when a task needs explicit acceptance and stopping rules. Use
[Project Intent](project-intent/README.md) for durable project goals and decisions,
and [Catalog Kit](../kits/catalog-kit/README.md) for reusable definitions. Do not
create a parallel progress ledger. Build status remains in TRACKING and work orders.

The experimental [library](../kits/assignment-kit/README.md) is a pure evaluator.
In a consuming module, pin the reviewed kit commit through that project's normal
Go/CUE dependency process. Adoption does not need a global tool or hook installation.
It collects no evidence, runs no tool or model, stores nothing and grants no permission.
Its `complete` means the declared requirements are satisfied by the supplied eligible
evidence at this snapshot. It cannot establish that the requirements cover the user's
whole intent, the observations are authentic, or a source is true.

## Start small

An assignment needs one required goal with an assessment profile, one finite budget,
a deadline, exact subject/policy pins, and an independently reviewed expected scope.
Supply the profile's criterion observations, measured budget usage and progress.
Pass evaluation time explicitly. Missing evidence stays visible.

```go
import assignment "github.com/geoffmilleraz/agentic-kit/kits/assignment-kit"

// a, scope and input are typed, receiver-reviewed values. Do not derive scope
// from a.Goals when claiming to detect an omitted user requirement.
result, err := assignment.Evaluate(a, scope, input, evaluationTime)
if err != nil {
    // Invalid input is never a completed task or permission to continue.
    return err
}
// Persist the scoped receipt under your application's access/retention policy.
// Your dispatcher owns any stop, escalation, approval or external effect.
```

Try the complete synthetic catalog-backed workflows from the repository root:

```sh
go run ./examples/assignment-kit
go test -race ./kits/assignment-kit ./examples/assignment-kit
cue vet ./kits/assignment-kit/...
```

These examples are backendless reference data, not a live command-safety deployment
or an investigation into real evidence. No provider, Fathom, sibling checkout or
installed hook is needed. Import the canonical CUE schema into your own CUE module
through your normal dependency process. Do not paste a second schema into your app.

## Author the agreement before doing work

1. Name the outcome, subject revision, governing intent references and policy digest.
   Separate user requirements from assumptions. Have the receiving owner review the
   expected required goal IDs separately from the authored implementation.
2. Make required goals conjunctive. Add independently useful stretch goals separately.
   Each goal's profile declares permitted evidence methods and hard criteria. A
   score, completed task count or stretch success cannot offset a failed requirement.
3. Name blocking concerns, advisory concerns, finite budgets/deadline and routes for
   escalation, pivoting and stopping. Use exact integer resource units. Include failed,
   retried and delegated work in usage. Missing usage is unknown, not zero.
4. If heuristic evidence needs a confidence bar, bind it to the exact observation,
   calibrated basis and site/context. Qualify that basis independently. Raw model
   confidence and general benchmark accuracy do not establish reliability here.
5. Evaluate actual observations. Preserve failed, unknown and conflicting evidence.
   Keep original assessment traces and confidence-bar results distinguishable.
   A digest binds supplied bytes; it does not authenticate them.

Use stable catalog keys for reusable criteria/templates. Exact lookup, current node
and complete definition pins belong to the adoption step. The receiver still validates
domain semantics and independently expected requirements. A renamed catalog path,
changed criterion or new policy does not refresh old evidence or consent automatically.

## Act on the result

| Result | Meaning and next step |
|---|---|
| `complete` | All required acceptance holds in this declared snapshot; dispatcher separately checks approval/publication authority. Stretch outcomes remain visible. |
| `continue` | Required acceptance is not yet established; use the goal/criterion traces to select bounded next work. |
| `blocked` | Blocking concerns, missing usage/progress or other declared blockers need resolution; do not infer permission from a good score. |
| `needs-review` | A deterministic escalation rule matched; send its exact evidence and route to the owning human/workflow only if authorized. |
| `pivot-required` | Stop the current approach for owner review; an approved changed assignment is a new digest. |
| `stopped` | A deadline, budget or stop rule ended this attempt. Retain partial results and causes; this is not success. |
| Error | Invalid/foreign/duplicate/oversized data or mismatched pins; repair the input under the current owner before reevaluating. |

The strongest matched disposition wins, while all causes remain in the receipt.
Budget/deadline equality exhausts the limit. Reserving spend and stopping concurrent
workers are application responsibilities, not capabilities of a snapshot evaluator.
Do not automatically replay an uncertain external effect after a lost response.

## Integrate once, retain the boundaries

For a coding assignment, link the project goal and work-order IDs, required gate,
consumer impact and exact evidence artifacts. For an investigation, require coverage,
source quality, counterevidence and explicit unresolved questions. For prose, require
supported facts and policy while treating voice/tone as soft criteria unless the
domain deliberately makes them mandatory. HITLMAN can render these typed outcomes
and ask for missing evidence; the app still owns RBAC, disclosure and approval.

Never amend an active assignment in place to remove a failed requirement or erase
spend. Review the new assignment/policy/scope pins and retain both attempts. A parent
reference establishes lineage, not delegated permission or a child budget. A child
receipt must satisfy an independently owned parent criterion before it counts.

Scope, bounds and negative acceptance: [AS specification](designs/assignment-kit.md).
Agent procedure: [assignment-brief](../.claude/skills/assignment-brief/SKILL.md).
