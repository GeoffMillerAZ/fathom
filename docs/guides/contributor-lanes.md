# Contributor lanes

Status: Proposed 2026-11-14 (architect ruling on owner direction). Decision draft: ADR-111.
Plan: [contributor-model](../plans/contributor-model.md). CT-D1 to CT-D8 are the architect's
contributor rulings. This guide extends [parallel-lanes](../design/parallel-lanes.md), especially
PL-D1 to PL-D12 and section 5. It does not replace its contracts or reduce the gate.

## Findings

- CT-F1 The existing ladder omits steward work and asks for a contributor guide, owners file and
  queue helper first (`docs/design/parallel-lanes.md:168`, section 5).
- CT-F2 Issue #139 describes manual claims and a future `task lane:ready` in sections 2 and 6.
  The supplied queue snapshot contains existing labels, not evidence that their readiness is grounded
  (issue #140 and the supplied open-issue snapshot, each issue's labels).
- CT-F3 Free independent readings start early, while paid proof retains entry gates
  (`docs/vision/46-proof-at-the-moment-of-change.md`, sections 14 and 14b).
- CT-F4 The autonomy revisit condition includes a second maintainer
  (`docs/intent/autonomy.md`, final line). Contributor arrival requires reviewing that boundary.
  Its current free pushes to main conflict with CT-D5 and must be amended by the architect.

## Decisions

### CT-D1 Roles

| Role | Responsibility |
|---|---|
| architect | Rule on scope and conflicts, check in at wave boundaries and weekly |
| integrator | Verify the merge conditions, batch integration, move main and tag |
| steward | Write grounded issues, independently review other issues, release stale claims, no merge right |
| lane worker | One agent session on any permitted machine, implement one handed contract |
| critic | Independently read the change, its proof and its risks before merge |

One person may hold several roles. A steward cannot ground an issue they authored.
A worker cannot provide their own critic read. Role assignment is operator configuration.
CODEOWNERS review routing grants no assignment or merge permission.

### CT-D2 An issue is the lane record

Copy the lane form. Include plan paths and row ids, class, exact owned files and allowed new-file
patterns, read-only files, handed contract files and base commit, proof command, dependencies,
machine requirement, size of one lane-day or less, advised model tier and reserved ids.
Use L-R reading, L-F named files, L-P leaf package, L-D data, L-E engine or L-S existing surface
as defined in `docs/design/parallel-lanes.md`, section 3. L-S stays with the main thread.

A second steward grounds each issue before applying `ready`:

1. Resolve every row id and its dependencies from the actual plan. Deferred is not done.
2. Inspect each existing file, symbol and test cited by the premise. Mark new owned files as new.
3. Run the handed proof at the exact base. Record the command, exit and relevant red assertion.
   Missing credentials, dependencies or an absent executable do not establish a valid red proof.
4. For L-R, prove the registered acceptance check is red because the required reading is absent or
   below its preregistered bar. A zero-exit measurement command alone does not establish readiness.
5. Compare owned paths, including globs and permitted additions, against every claimed lane.
6. Record independent reviewer role, base SHA, evidence spans, unmet needs and machine compatibility.

If a premise is false or the proof is already green, remove readiness and send it back for repair
or closure with evidence. Do not widen the worker's scope. A draft issue can exist well ahead of
its wave, but it cannot be ready while its dependencies or contracts are missing.
A newly claimed overlap invalidates the grounding until rechecked.

### CT-D3 Claim and expiry

Use existing labels from the queue snapshot: `lane`, `ready`, `claimed`, `blocked`,
`needs-ruling`, `engine`, `language`, `macos`, `spend`, `p0`, `p1`, `p2` and `size-s/m/l`.
RQ5 adds `needs-grounding` for issues awaiting independent confirmation. Do not invent labels for roles or Windows support. Machine requirements live in the issue body.
Take the highest priority eligible issue in the queue's order. Obey dependency and spend gates.

A claim requires all three: an assignee, `claimed` replacing `ready`, and this public-safe comment:

```text
CLAIM machine-role=helper branch=lane/<issue>-<slug> base=<sha> session=<opaque-token>
```

The machine role is a role, never a hostname. The opaque session token identifies one session,
not a person. GitHub supplies account identity. One claim per session. Preserve the queue's
first-CLAIM-after-last-RELEASE ordering, then re-read the assignee and labels. If there is a race,
stop work until a steward resolves it. If two engine issues race, the higher issue number releases
as prescribed in issue #139 section 1. The helper is not a transaction lock.
An outside contributor unable to self-assign posts the claim request and waits for a steward
to set the assignee and label. An incomplete claim grants no file ownership.

Caps remain one engine lane globally, at most five claimed language lanes and four heavy test or
oracle processes per machine (`docs/design/parallel-lanes.md`, section 4). Machine process counts
are operator attestations until a trusted input is available. Unknown capacity blocks dispatch.

A steward releases a claim after 48 hours without a branch push. Start the clock at the claim
when no push exists. Comments and local commits do not reset it. Query branch push evidence,
not commit author dates or PR `updatedAt`. If push history is unavailable, report age unknown
and inspect manually rather than inventing a timestamp. Before release, recheck the branch,
record its last SHA and the evidence, comment `RELEASE <session>: no push for 48 hours`, remove
the assignee and `claimed`, then restore `ready` only after new grounding. Otherwise use `blocked`.
Preserve the branch and work for the next worker. Release automation only drafts the action.

### CT-D4 Branch, review and wave

Use `lane/<issue>-<slug>`. Outside contributors use a fork with upstream main as the base.
One pull request serves one issue. Its body says `Closes #<issue>` when it completes the issue's
rows, or `Part of #<issue>` otherwise. It lists rows, proof results, CI, owned paths, readings and unknowns. A draft PR may exist while work proceeds.

Before merge the integrator requires green CI on the current reviewed head, an independent critic
read, and a clean path check. Until CTR-3 and CTR-4 ship, the integrator manually compares both
sides of every rename and every added, changed or deleted path against the issue's frozen owned
paths. A PR cannot approve its own wider paths. Missing evidence stops integration.
This manual review does not claim the proposed H0 automated path gate exists.

Integrate in batches, order overlaps, regenerate derived outputs under integrator ownership,
rebase the integration batch to preserve linear history, run the mandatory full gate and plan
verification, then move main. Watch main's CI. Red main
stops merges. A wave is one integration batch, green CI, a tag and a consolidation pass
(`docs/design/parallel-lanes.md`, sections 8 and 9). Consolidate row status, stale claims,
queue readiness, readings and dependent contracts. Never waive a gate for throughput.

### CT-D5 Sync and ownership

Fetch upstream main and rebase at session start and before the PR. A lane resolves its own
conflicts and edits only its owned files. Never rebase or reset another lane's branch.
If rebasing a previously pushed branch would require a history rewrite, stop and return it to
the integrator for an authorized recovery. Do not force-push as an automatic sync step.
Prefer completing the final rebase before the first push. Only the integrator moves main.
Generated shared files need an explicit integrator handoff rather than ownership by every lane.

### CT-D6 Architect check-in

At every wave boundary and weekly, read a fixed checklist of computed evidence:

| Input | Evidence |
|---|---|
| Plan status | `fathom plans status --service fathom`, with indexed root and freshness |
| Queue | Counts by priority and ready, blocked, claimed, needs-ruling, plus overlapping claims |
| CI | Branch heads and main run conclusions, absent runs shown unknown |
| Readings | Filed readings with verdict, input SHA and preregistered bar |
| Claim age | Last branch push evidence and elapsed hours, or unknown |

CTR-7 specifies the read-only collector. Until it ships, collect those sources manually and attach
source timestamps. The architect posts coded rulings on #141. A steward incorporates accepted
rulings into issue bodies and the integrator records governing changes. Comments alone do not
change the handed contract (issue #141, opening paragraph). Unknown inputs remain unknown.
Do not substitute activity prose or completed-commit counts for these inputs.

### CT-D7 Contributor ladder and machines

Reuse `docs/design/parallel-lanes.md`, section 5, with one additional step before step 2:

| Step | Task | Proof |
|---|---|---|
| 0 | Read AGENTS.md, intent router, plan traps, wave guide and queue rules | Name the owned files and proof in a sample issue |
| 1 | Install, wire a sample and read a verify receipt | Doctor green on the sample |
| 1a | Write one issue, independently ground another | Second steward confirms rows, premise, red proof and no overlap |
| 2 | Re-run a committed reading | Compare the registered numbers |
| 3 | Run an open reading | File its verdict against the registered bar |
| 4 | Complete one data lane | Existing recipe's harness reading |
| 5 | Add hazard fixtures | Every required mutant killed |
| 6 | Implement a leaf package | Handed contract tests and critic review |

Stewarding needs git, gh and an agent. Build and test evidence can be supplied by an eligible
worker and independently inspected by the steward. An unrun red proof means the issue stays draft.
Windows lane execution uses WSL2, with each issue's machine requirement still binding. See
[Windows setup](windows-setup.md). A `macos` lane does not become eligible through WSL2.

### CT-D8 Public and fork boundary

Nothing private enters an issue, comment, reading or PR. Use role names for machines and
contributors in narrative. Never paste credentials, private paths or host identifiers.
Fork pull requests receive no secrets. Review and test the untrusted head without privileged
credentials. Never use a privileged workflow to execute fork code. Branch protection and required
checks are unknown here, so an instruction is not a claim that forge enforcement exists.
The current workflow uses `pull_request`, read-only contents permission and Linux runners
(`.github/workflows/ci.yml:19-23,62-69`). This spec does not change that workflow.

## Risks

- CT-R1 Readiness can race with a claim. Recheck overlap after the claim and before merge.
- CT-R2 A rebase of published work can require rewriting history. Return for integrator recovery.
- CT-R3 A document can masquerade as automation. CTR rows stay open and manual receipts name gaps.
- CT-R4 Public issue text can leak private context. The steward checks redaction before publication.

## Sequence

Adopt ADR-111 and amend the autonomy boundary and issues #11 and #139 first. Land CTR-1 and
CTR-2, then the H0 path check and wiring in CTR-3 and CTR-4 before automated fan-out. Build the
expiry proposal and check-in collector next. Run the Windows witness when an eligible worker
exists. This is operator tooling, not a scheduler inside fathom, preserving PL-D4 and
`docs/intent/decisions/README.md`, ADR-098's read-only work-plane boundary.

## Decision models

None applies. Queue validation, expiry and path comparison are deterministic operator bookkeeping.
Provider `none` is the complete behavior. Agents draft issues but never establish a code fact by
judgment. Opt-out is manual stewardship. Measurements are the fixture bars in CTR and the Windows
witness, with no provider call or new decision-port site.
