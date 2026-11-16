# CTR-4, CTR-6, CTR-8 and CTR-9: wiring and honest witnesses

Artifact: `contributor-tooling-wiring-readings-w15-fix1`, branch `lane/contract-contributor-tooling-wiring-readings`.
Original source-only extraction from `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`.
W15-4 revises U1/U2 in source draft `7410870f9d1299ad64e134c8c02ac280c81b0490`.
FIX1 addresses critic F1 on exact `25eb5eafa4ecdec63c3e1bda935655f86badfeef`.
This branch adds only `docs/contracts/contributor-tooling-wiring-readings.md`.
Companion citations resolve to that preserved source until individually
landed. Only the W15-4 authority, acceptance and activation clauses are amended;
companion ports, remaining assertions and source/native-proof prerequisites stay fixed.

Status: W15-4 FIX1 applied; fresh secondary re-review required. No implementation.
Source base: `83226aece9b0f421381373f8dafc4a912d3888e5`.
Fetched main before FIX1: `17d930d2f1b96b707b03ec84d5f87a3a5333c24d`.
W15-4 resolves design choices; it grants no required-check activation or fan-out.
Exact ownership and serial order: `docs/contracts/contributor-tooling-split.md` (preserved source `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`).
Handed interfaces: `docs/contracts/contributor-tooling-trust-paths.md` (preserved source `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`),
`docs/contracts/contributor-tooling-queue.md` (preserved source `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`). Each is prospective.
No plan boxes, forge writes, test runs, native readings or gate runs here.

## Intent, existing seams and settling text

P2/P3/P15/P17/P22 and I8/I19/I63 apply. The dispatch non-goal and F92 prohibit
a scheduler or machine monitor in fathom. ADR-111 is proposed, CT-D1–8 owner
direction governs these drafts through W8-2b. Autonomy's contributor/CI revisit
has already fired as ADR-111 records; architect-owned reconciliation remains
pending. No new trigger evidence or future implementation. W2-1 door 80 /
consumer 81 and no package headroom are unchanged. No Go migration.

## W15-4 rulings and activation boundary

U1 is resolved by W15-4: the trusted operator/integrator owns admission and
snapshot capture. The check source, driver, schema and tool selection come
from the pinned trusted **base branch**, never the candidate they judge.
The operator binds that base, exact head, admitted issue/lane/session and
complete snapshot bytes/digests independently of the candidate. A candidate
workflow, environment, comment, manifest, artifact or success message grants
no authority. No acquisition service or forge API is invented by this contract.

U2 is resolved by W15-4: retain the current **full gate plus REC-1 ownership**
until REC-7 is implemented, integrated and proven. `gate record-ids` is not
`rec vet` and does not claim its shape/link/cycle checks. REC-7's later gate
turn remains governed by ctr-11.md and records-integration.md. Its absence
is not an extra CTR-4 implementation dependency or a reason to weaken the
current gate; all existing REC-1/CTR-2/CTR-3/native-proof prerequisites remain.

This revision authorizes design and named tests only. Enabling a required
check or automatic fan-out waits on the owner's protected-main answer,
OQ9, and a separately authorized activation turn. A contract LAND, test PASS,
borrowed reviewer approval or green gate is not activation permission.
No settings, branch protection, workflow, script, source or plan changes here.

Source-base facts below remain pinned to the original source revision;
they are source citations rather than current deployment/capability claims:

| Source | Existing seam and limit |
|---|---|
| `Taskfile.yml:319-323`, `.gitattributes:1-3` | merge.fathom-regen is configured; generated files retain current side, then regenerate. No union rule for assessments index is present |
| `Taskfile.yml:346-378,387-409` | scripts-check runs shell tests; validate wraps gate-log and lease, then full validate:unleased; record-ids-check is included |
| `Taskfile.yml:425-433` | tenancy-lint and gate record-ids exist; no `rec vet` CLI on this base |
| `.github/workflows/ci.yml:17-23,62-104` | push/pull_request, contents:read, Linux full validate:unleased; commit-message policy is limited to main pushes |
| `scripts/gate-log.sh:13-43` | full START/PASS/FAIL UTC stamps, tree/clean/commit and FAIL exit; START wraps the lease too |
| `scripts/gate-lease.sh:58-71` | wait message has no timestamp and acquisition has no timestamp |
| `docs/guides/agent-waves.md:84-104` | existing merge driver/epoch and exact stacked-work ancestor check |
| `.github/pull_request_template.md`, CT-D4/5/8 | independent critic, reviewed current head, rebased owned paths, public/fork boundary |
| `docs/guides/windows-setup.md`, `scripts/cloud-tools.env`, `go.mod` | prescribed WSL2 route and current pins, not a witnessed pass |

[#140](https://github.com/GeoffMillerAZ/fathom/issues/140) is the reused issue,
not a duplicate queue project. Preserve already implemented merge-driver work.
Its proposed assessment union requires an independent ordering check in a
separate granted turn; CTR-4 owns neither .gitattributes nor agent-waves.md.
Its conditional G1 pre-push relaxation is still unanswered in
[#141](https://github.com/GeoffMillerAZ/fathom/issues/141). This track changes
no pre-push hook, protected control or gate condition. CTR-4 explicitly says
H0, not a lighter lane gate; PL-D6 and W4-2 require full integration/CI.

## CTR-4: proposed local Taskfile boundary (design/tests only)

The later implementation turn owns only Taskfile.yml, ci.yml and wiring.md.
This authoring turn owns only this contract document. No implicit grant to
lefthook.yml, AGENTS.md, autonomy, guide, other scripts or workflow files.
The integrator owns this L-S turn. CTR-2/3 **implementations** and their source
contracts must be on actual main, not just in an author branch or receipt.

Every `task lane:ready` reference below means the **base-owned** entrypoint
operating on the candidate data root. The trusted operator invokes Task with
argv equivalent to `task --taskfile CONTROL/Taskfile.yml --dir DATA lane:ready`.
CONTROL is checked out at the pinned trusted base; DATA at the exact head.
Never use DATA/Taskfile.yml, its includes, modules or env files as authority.
Resolve authority-check scripts/schema/tools from CONTROL, not the current
working directory. Candidate execution remains an unprivileged proof input,
not the owner of the verdict or required-check policy.

The prospective entrypoint requires these operator environment values, no defaults:

| Variable | Exact meaning |
|---|---|
| `FATHOM_LANE_TRUSTED_BASE` | Full immutable trusted base-branch SHA supplied by the operator, shared with REC-1; never selected by candidate |
| `FATHOM_LANE_ID` | Trusted REC-1 slug, shared with REC-1 |
| `FATHOM_LANE_HEAD` | Exact clean candidate commit SHA |
| `FATHOM_LANE_SNAPSHOT` | Operator-owned, byte/digest-bound complete capture under ignored workspace; candidate data cannot replace it |
| `FATHOM_LANE_NOW` | Explicit RFC3339 UTC clock for queue recommendation |
| `FATHOM_LANE_ISSUE` | Positive issue number from trusted steward binding |

The snapshot's base, issue→lane mapping, manifest blob and grounding must match
these independent authority values. Issue body or candidate env files cannot
supply them. Both path checker and queue checker run before any candidate proof
command. This local task assumes an honest operator running the reviewed tools;
the same W15-4 base-source and operator-admission rules apply locally and in
fork CI. Missing complete source or admission evidence still blocks the
prospective verdict. Required-check activation and automatic fan-out remain
HELD for OQ9 regardless of a local result.

Proposed execution order, no `continue-on-error`, swallowed status or optional
check when an input is absent:

1. Freeze clean root/head/tree and trust values; run
   `python3 CONTROL/scripts/lane_paths.py --root DATA --head "$FATHOM_LANE_HEAD"` and
   `python3 CONTROL/scripts/lane_queue.py ready --snapshot "$FATHOM_LANE_SNAPSHOT"
   --now "$FATHOM_LANE_NOW" --issue "$FATHOM_LANE_ISSUE"`. Any 1/2 blocks.
   A ready label alone cannot satisfy this check. For an already claimed lane,
   use the **same** ready evaluator's claimed-owner continuation described
   below, never require a worker to release/reclaim merely to run its check.
2. Independently ground #140's commit checks over immutable trusted-base..head,
   including every nonmerge commit, with `bash CONTROL/scripts/commit-msg-check.sh`
   reading a workspace temp file containing the exact message. Use git argv
   arrays / quoted shell variables. Never interpolate issue proof text into
   shell. Freeze every authority-check driver and script from the trusted base
   for both local and CI checks. Candidate copies are data, never the judge.
3. Stacked-work check uses each **other** open PR's current head from the
   complete trusted snapshot extension below: `git merge-base --is-ancestor
   OTHER_HEAD HEAD`. If true and OTHER_HEAD is not already an ancestor of
   trusted base, refuse. Missing object/history is unknown and blocks. Same
   issue's own PR is excluded by trusted binding, not title substring.
4. Retain #140 checks: real `command gofmt -l` over tracked candidate Go files
   using the trusted pinned tool (not a shell function; reject output), the
   release-notes checker built from CONTROL's trusted vendored source with
   its existing `-root DATA -check` inputs, and
   `bash CONTROL/scripts/check-merge-debris.sh` with DATA as working directory.
   The base-owned Taskfile also invokes `tenancy-lint` and `test:affected`
   against the candidate. Existing full gate already tests isolation; retain
   that coverage without inventing the X-R10 selector that #140 doesn't name.
   Candidate test binaries and the handed proof run only in the disposable
   unprivileged context. They remain proof inputs, not check-source authority.
   The trusted base owns the mandatory check list, authority-check executables
   and result evaluation; the candidate cannot redefine or skip them through
   its Taskfile/scripts. Preserve the current full gate and REC-1 ownership
   until REC-7; neither passing record-ids nor the absence of rec vet changes
   that list. No claim that record-ids supplies vet's later checks.
   Run the exact REC-1 `proof_command` loaded from the trusted manifest only,
   in the unprivileged candidate execution context. Its bytes are approved
   command text, invoked as argv `["bash", "-c", trusted_proof_command]`.
   Do not substitute proof text from issue comments/body or candidate CUE.
   This is never executed in the trusted data-only preflight job.
5. Retain the existing full `task validate` and REC-1 ownership checks under
   the existing machine lease until REC-7. The authoritative ingest **and**
   strict plan verification use the evaluator built from CONTROL's pinned
   trusted base and dependencies, never DATA/bin/fathom or candidate verdicts.
   Inspect the exact DATA head and every active top-level plan except README.md,
   with `--since` the immutable base, into a fresh protected scratch store.
   DATA is inspected input only for this evaluation. The existing fresh-store,
   complete-plan and exact-head requirements remain; the candidate-binary
   integrator recipe does not supply this independent authority.
   Missing, unproven or incompatible trusted evaluator/source/dependencies
   yields unknown/nonzero and blocks the readiness claim. Never fall back to
   a candidate-built evaluator or relabel candidate success as trusted proof.
   Required candidate runtime/regression proof still runs in its advisory
   disposable job; it cannot self-certify the authoritative decision.
   `fathom verify` remains static, lexical receipts do not prove execution,
   and existing global verify fail-open semantics are unchanged.
6. Verify head/tree/clean state again and attach command, exit, source hashes
   and current head to the receipt. Include the trusted base/check-source blob
   ids, trusted evaluator source/dependency/binary identity and its independent
   ingest/verify results, operator admission/snapshot digest and the separately
   inspected REC-1 ownership result. Keep candidate runtime/regression results
   separately identified as advisory proof, never authoritative verification.
   Record `required-check activation: HELD OQ9` and
   `automatic fan-out: HELD OQ9`; no prospective local PASS alters those holds.
   These are wiring receipt assertions, not new queue output fields or a
   lifecycle format. Any changed head/source/snapshot invalidates results.

Exact authoritative scratch command sequence, UNRUN here. CONTROL/bin/fathom
is built and identity-checked from CONTROL's trusted base/module/dependency
pins, outside DATA, using the existing toolchain without pin changes. The
trusted driver owns the resolved absolute CONTROL/DATA roots and launches
this block with **DATA as working directory**, so local git-aware plan checks
inspect DATA rather than CONTROL. Its executable selection is still CONTROL:

```sh
lane_ready_tmp=$(mktemp -d "$CONTROL/workspace/tmp/lane-ready.XXXXXX")
"$CONTROL/bin/fathom" ingest --store "$lane_ready_tmp/verify.sqlite" \
  --service fathom --path "$DATA"
lane_ready_args=()
for lane_ready_plan in "$DATA"/docs/plans/*.md; do
  [ "$lane_ready_plan" = "$DATA/docs/plans/README.md" ] && continue
  lane_ready_args+=(--plan "$lane_ready_plan")
done
"$CONTROL/bin/fathom" verify --strict --since "$FATHOM_LANE_TRUSTED_BASE" \
  --store "$lane_ready_tmp/verify.sqlite" --service fathom "${lane_ready_args[@]}"
```

The base-owned Taskfile executes this sequence with Bash and immediate error
propagation. CONTROL and DATA are the existing trusted driver root bindings,
not new configuration fields. The evaluator, its dependencies and decision
policy are fixed by CONTROL; candidate executable/module/plugin/env/config
content cannot replace them or be executed as authority. The candidate job
cannot write the trusted store, evaluator or output. Do not reuse its ingest
store, receipts or successful verdict as the authoritative evaluation.
Missing/incompatible trusted evaluator or inability to inspect the complete
DATA head is unknown/nonzero, never a candidate fallback or vacuous pass.
The unique scratch directory stays under protected CONTROL/workspace for
logs/critic inspection, with no shared daemon store. Freeze DATA head/tree
before and after, as well as CONTROL evaluator/source and capture identity.
No shell expansion of filenames into proof command text. Gofmt input uses
NUL-aware tracked Go path enumeration and argument arrays, not a whitespace
split. Empty active-plan set remains a missing-input failure.

For CI a runner is exclusive: keep existing `task validate:unleased` as today.
Run the extra queue/path/commit/stack preflight in a separate preceding job,
not a nested full `task lane:ready` that repeats the full suite. Both the
preflight, independent CONTROL-built ingest/verify decision, and the existing
full CI plus REC-1 ownership are required on the exact candidate head for the
combined **fixture/proof** lane-ready result. The required disposable candidate
runtime/regression job is advisory evidence to the trusted decision, never a
replacement for it. A candidate-reported success cannot make a failing or
unknown authoritative evaluation pass.
No result from this design enables a required check or automatic fan-out.
Those activations remain held for OQ9 and a separately authorized turn.
Push jobs retain today's full suite even
when a lane has no admitted manifest. The lane preflight then reports
not-admitted/unknown, not a passing lane. Main/batch integration remains
integrator owned; no per-lane exemption is used to skip its full gate.

### Queue consumer assertions frozen in CTR-2

Queue v1 already includes complete `pull_requests` and claimed-owner
`--session` continuation in its first turn, specified in the queue contract.
CTR-4 receives those interfaces without editing queue files. Taskfile passes
`FATHOM_LANE_SESSION` only from trusted operator/controller binding when
checking a claimed lane. It performs actual git ancestry checks against the
complete PR projection. Missing/ambiguous issue-to-PR mapping is unknown.
The claimed continuation test also requires independent post-claim grounding.

## Fork-safe CI: data, executable and credentials are separate

CT-D8 and CTR-4 settle the security properties, not live forge enforcement.
Retain `pull_request` and `push`, Linux runners, contents:read. Never use
pull_request_target, a write token, secrets, environment approval that releases
credentials to fork code, or a privileged workflow_run that executes fork
artifacts. Do not take shell commands or environment assignment text from an
issue, PR, manifest or checked-out head. No paid API or provider credentials.
Separate jobs, filesystem roots and caches for trusted validation and candidate
execution; no candidate cache/artifact is executable authority in the trusted
job. Checkout credentials are not persisted.

Trusted preflight checks out the immutable event **base** into a clean control
directory with scripts, Taskfile, pinned tools and REC-1 schema from that base.
It obtains the candidate Git objects as untrusted **data** for path/ancestry
checks, never runs candidate code in this job. `FATHOM_LANE_TRUSTED_BASE`
equals the event's full base SHA, never origin/main's later moving tip.
The trusted operator/integrator owns the admission and complete snapshot;
the base-owned controller injects the admitted issue/lane/session mapping
and byte/digest-bound capture. PR body/branch/env cannot choose or replace them.
Check-source blobs must resolve through the trusted base branch; a candidate-
provided ref or workflow cannot name an alternate executable authority.
Missing admission/snapshot/schema/object fails closed for lane eligibility.
Exact head and base are recorded. Pin checkout and setup actions as current
ci.yml does. Do not make a new unpinned action dependency.

The data checkout's HEAD is the candidate SHA; the control checkout's HEAD
is the trusted base. Invoke the control script with the **data** root, so the
path command's HEAD identity check remains literal:

```text
python3 CONTROL/scripts/lane_paths.py --root DATA --head FULL_CANDIDATE_SHA
python3 CONTROL/scripts/lane_queue.py ready --snapshot TRUSTED_CAPTURE --now UTC --issue N --session TOKEN
```

CONTROL/DATA are separately created checkout directories under job workspace
scratch, not manifest fields. Resolve them from the trusted workflow. No
candidate Taskfile, Python module or workflow-generated executable is loaded
to run this preflight. Snapshot and environment authority remain controller
inputs settled by W15-4/U1; these command templates claim no existing
acquisition API or required-check activation.

Candidate build/test job uses that exact PR head, persist-credentials:false,
no secrets or token environment, and the ordinary read-only job permissions.
It still runs full existing validation and the required candidate runtime/
regression proof, including candidate-built ingest/verify when that proof
requires them, in its advisory disposable unprivileged runner. Candidate
success, even with matching head/source identity, is not an independent
verdict and cannot replace or certify CONTROL's authoritative ingest/verify.
The trusted decision does not consume candidate-created stores or executable
outputs as authoritative evidence. Do not interpolate untrusted branch names
into shell.
Public artifacts carry relative source spans and role ids only.

An arbitrary fork can edit its workflow. A check printed by candidate code
does not establish trusted enforcement. ADR-111:7277 explicitly leaves required
checks/branch protection unknown. W15-4 permits the base-source/controller
**design and tests**, not turning it on. Required-check activation and automatic
fan-out remain HELD for the owner's protected-main answer OQ9 and separate
activation authority. The contract cannot claim those settings are enforced.
Report source/controller evidence and missing runtime inputs explicitly.
`bypasses=0` counts the registered fixture bypass attempts only; YAML or a
candidate's success message cannot establish that proof. No invented admission
service, privileged workaround or reviewer approval supplies activation rights.

## Prospective CTR-4 integration assertions

Tests are named tasks inside the owned Taskfile (no new script/test-file grant):
`lane:contract-test` builds local git fixtures under workspace/tmp from the
frozen real base branch with REC-1 and CTR-2/3 actually landed. Pin the
reviewed check/driver source in that trusted fixture base before creating the
candidate branches; never load it from the branch under judgment. The trusted
operator supplies complete admission/snapshot captures, including provenance
and digests. This fixture setup is not a deployment or a claim that current
main already serves the proposed checks. Freeze all fixture
source/manifest/blob/head hashes **before** replay. Embedded setup commands
are fixture-only and never run forge actions. The test creates valid,
outside-add/delete/rename, manifest-self-broaden, missing-trust, claimed-
continuation, missing-capacity, stale-grounding and stacked-PR branches.
The actual named step must fail on each bad branch. The valid clean branch
must pass the real task with all mandatory checks, after the shared gate is
clear and under the lease. No stubbed full gate can count as that proof.

`lane:contract-test` cases and assertions:

| Case | Required witness |
|---|---|
| `valid` | Base-owned `task lane:ready` exit 0 on frozen clean head, complete source/admission capture, current full gate plus REC-1 ownership logs and independent CONTROL-built ingest/strict-verify result; candidate runtime/regression proof remains required and separately advisory; no omitted step; activation/fan-out still HELD OQ9 |
| `outside` | Outside addition/deletion and either rename endpoint fail paths before candidate proof; owner/path in output |
| `self-broaden` | Candidate manifest/schema/script substitution cannot authorize the outside file; missing trusted manifest fails |
| `claimed` | Winning session with independent refreshed grounding passes preflight; wrong/incomplete/raced token fails |
| `stale` | Changed body/base/head/contracts/capacity/claims or absent source prevents pass; older green CI head cannot satisfy |
| `stacked` | Another open unlanded PR head ancestor refuses; already-on-base ancestor doesn't; unavailable object fails |
| `fork` | Trusted operator owns admission/snapshot; check and driver blobs come from base branch; changed candidate check/Taskfile/schema/capture cannot judge itself; no persistent credential/secrets/write permission; fixture PASS never claims required-check activation |
| `gates` | Current full validate dependency set, REC-1 ownership, perfgate/test/race/symlink temp canary, pre-push and message checks remain until REC-7; record-ids is never counted as rec vet; each seeded step failure is named and nonzero |
| `trusted-source` | `TestContributorCheckSourceFromTrustedBase`: inspect check/driver/schema/evaluator source/dependency/binary identity against CONTROL trusted base; substitute candidate copies/ref/capture and assert they never become authority. In an independently failing fixture, substitute candidate-built ingester, verifier and both together returning false success; CONTROL ingest/verify must still reject the fixture despite those successes. Candidate-only stores/receipts cannot satisfy the decision. Missing or incompatible trusted evaluator, source/dependencies or complete admission yields unknown/nonzero; never candidate fallback |
| `activation-held` | `TestContributorActivationHeldForOQ9`: local PASS, contract LAND and prior reviewer approval each leave required-check activation and automatic fan-out HELD; no forge/settings mutation, token or self-granted permission |
| `rec1-until-rec7` | `TestContributorFullGateAndREC1UntilREC7`: without REC-7, demand the complete current full gate plus shared REC-1 ownership; absent/failing ownership or omitted full-gate step blocks; passing record-ids never supplies record shape/link/cycle vet proof |

Proof commands, all UNRUN here. CONTROL and DATA retain the pinned base/head
meaning above; the harness and authority-check tests are loaded from CONTROL:

```sh
python3 -m unittest discover -s CONTROL/scripts -p lane_queue_test.py
python3 -m unittest discover -s CONTROL/scripts -p lane_paths_test.py
task --taskfile CONTROL/Taskfile.yml --dir DATA lane:contract-test
task --taskfile CONTROL/Taskfile.yml --dir DATA lane:ready
```

CTR-4's exact PASS line needs actual local replay **and** fork/controller
fixture evidence satisfying every assertion, plus fresh independent critic
inspection of logs. The exact receipt line stays
`CTR-4 verdict: PASS bypasses=0 privileged-fork-runs=0`; its registered scope
is the witnessed fixtures, not required-check/settings deployment. Report
activation/fan-out HELD OQ9 separately. No receipt presence, contract ruling,
prior borrowed approval or configuration-only inspection substitutes for
integration proof or grants activation. Candidate runtime/regression success
cannot replace the independent CONTROL evaluation. This FIX1 runs none of
those tests and claims no actual RED.

## CTR-6: independent handoff replay

Only `docs/bench/readings/contributor-model/handoff.md`; CTR-4 prerequisite.
Use the same fixture heads and snapshots, template `.github/pull_request_template.md`
and CT-D1–5. Local fixture forge, never live claim/comment/merge. Record each
step's input digest, base/head, command/exit, role ids and independent critic.
For each omitted claim-triple member, one-session check, independent grounding,
current-base rebase, intended RED/GREEN pair, current-head CI, path check,
independent critic and release recheck, merge eligibility must be false.
Author=steward and worker=critic fail even if one person may hold other roles.
Published-branch rebase requiring a rewrite returns `recovery-required`, never
force-push. No real git/forge mutation outside local fixture repository.

Run actual `task lane:ready` for handed branches and the plan verify on the
filed receipt. Log the unsafe case's exact refusing check. All assertions must
hold before `CTR-6 verdict: PASS unsafe-merges=0`. A lexical match only proves
filing. Missing admission/gate/critic proof yields incomplete/unknown, not PASS.
This is not WSL2 evidence and cannot close CTR-8. Use the W15-4 base-owned
entrypoint, independent CONTROL-built ingest/verify and trusted operator
captures throughout the replay, retain full
gate plus REC-1 ownership until REC-7, and show required-check activation and
automatic fan-out HELD for OQ9. Fixture merge eligibility is not real merge or
fan-out permission.

## CTR-8: native WSL2 witness, unavailable here

Only `docs/bench/readings/contributor-model/windows.md`; wait for CTR-6 and an
actual WSL2 worker. CT-D7 and windows-setup.md define route. On the same exact
commit: record WSL2/Linux/kernel/tool versions, clone in Linux filesystem,
machine eligibility, fixed input fixture/head ids, setup commands/exits,
claim snapshot, actual intended RED then GREEN, `task lane:ready` and full
`task validate` logs. A named unsupported oracle or skipped check prevents
`CTR-8 verdict: PASS skipped=0`. Keep native Windows library/build/runtime/
hooks unknown unless separately witnessed. Linux CI, macOS gate, drafted
command or source portability is not a WSL2 witness. No run is claimed here.
A full gate this row needs belongs to that authorized worker after the gate
lease is available, not to this author and not to CTR-9. The witness must use
the trusted base-owned check, independent CONTROL-built ingest/verify and
complete operator admission/snapshot, with
full gate plus REC-1 ownership until REC-7. A native PASS does not enable a
required check or automatic fan-out before OQ9 and a separate activation turn.

## CTR-9: completed logs only, no new full gate

Only `docs/bench/readings/realignment/gate-time.md`; no needs. Acquire existing
completed gate logs with base/head/tree, run identity and timestamp coverage.
`rg -n 'fathom gate:|waiting for gate lease' workspace/logs` is the proposed
locator, not a reading by itself. No log set is handed to this author.

Parse exact current gate-log grammar from `scripts/gate-log.sh:27,38,40`:
`fathom gate: TASK START UTC tree=HEX clean=N commit=HEX`, paired with
`fathom gate: TASK PASS UTC ...` or `FAIL exit=N UTC ...` in the same run.
Source filename plus framed span identifies a run; don't pair across logs or
tasks, even with identical tree. Report duplicate frames, orphan starts/ends,
changed commit/tree/clean, truncations and negative durations explicitly.
Include FAIL as completed, never only successful trials. Publish every run's
source digest, line spans, status and raw wall seconds, then coverage and
summary. Repeated copies of one run are not new trials. Missing run identity
or contradictory copies remain unknown. Base association must be proved,
never inferred from today's HEAD.

Wall seconds = terminal UTC minus START UTC. It includes lease wait because
START precedes the command taking the lease. Report execution-only duration
separately **only** with a timestamped acquisition event. Current lease code
prints an untimestamped waiting message and no acquisition timestamp, so
wait is UNKNOWN for these logs, even if no waiting line was printed. A real
external acquisition log can supply wait=request→acquired with cited clock,
run mapping and spans. No acquisition evidence means neither zero wait nor
wall-minus-zero execution. Do not change gate instrumentation in this lane.

Frozen reading sanity cases: START `2026-11-13T00:00:00Z`, PASS
`2026-11-13T00:10:00Z` gives wall 600 s / wait unknown. FAIL at 00:03:00Z
gives wall 180 s / wait unknown. A separately proved request 00:00:00Z and
acquisition 00:02:00Z gives wait 120 s / execution 480 s for the first run.
An orphan START is incomplete, not a 0-second completed run. These are
synthetic arithmetic examples, not measurements of this helper.

File `CTR-9 verdict: BASELINE` only with at least one completed actual run,
exact base association, disclosed missing/truncated coverage and separately
reported wall/wait. Unknown acquisition times may remain unknown as CTR-9
explicitly allows. No completed raw logs means incomplete, not a baseline.
No fresh full gate is needed. No performance or lease proof ran in this draft.

## App-kit selection and reasoned divergence

Consulted project adoption `docs/app-kit.md`, app-kit guidance→intent→cli-kit,
and capability `cli.output`, catalog
`3ee2c1f02a6199c2e59361171c7f1fdb957a7a70f64e875a73969608314d7318`,
capability digest `6ffa60edf292958c683cc43a85e08ebfaebd61a288462d1b4bf255a0952b22dc`.
Quality-work selection is untracked (empty authored inventory is not proof),
revision `7c2842f733cc32984cce29ceb707aa0c82b72e87b073bc0125e318a60cd0cc7b`.
CLI 0/1/2, plain stable JSON/no ANSI and explicit unknown evidence are adopted
as interface conventions. Reasoned divergence: W8-2b explicitly requires
scripts. No Go terminal runtime or dependency migration for these Python
operator tools. Their exits never alter global verify semantics. No visual
reference or browser UI applies. Contribution-loop disposition is recorded
here because project adoption edits are outside this lane's grant. No
reusable kit implementation changed and no upstream write is authorized.

## W15-4 disposition and dispatch conditions

* U1 resolved: trusted base-branch check source and operator-owned complete
  admission/snapshot, never candidate authority. FIX1 pins authoritative
  ingest/verify to CONTROL's trusted base evaluator with DATA as inspected
  input; candidate runtime/regression proof stays required but advisory.
  Missing/incompatible trusted evaluation blocks as unknown/nonzero.
  Design and named tests only;
  required-check activation and automatic fan-out wait on owner OQ9 plus a
  separately authorized activation turn. No unresolved acquisition API is
  invented and no operational channel/enforcement is claimed to exist.
* U2 resolved: the current full gate plus REC-1 ownership stand until REC-7.
  Do not equate record-ids with rec vet, add an artificial REC-7-before-CTR-4
  dependency, waive an existing gate or change REC-7's approved gate-owner turn.

ADR-111/CT-D Sequence's autonomy/#11/#139 amendments, actual main/F5 and all
complete source/native-proof prerequisites remain. REC-1 and CTR-2/3 must
actually land before implementation dispatch; CTR-6/8 retain their stated
row needs. Missing evidence remains unknown. CTR-9 uses completed logs only,
with no new full gate and unknown acquisition times left unknown.

Only this contract is edited. The three companion contracts cleared for bundle
6, source/scripts/CI/settings/plans and all activation controls stay untouched.
W15-4 settles the two choices, not this revision's review verdict. The exact
new commit requires a fresh foreman-routed secondary critic; existing borrowed
reviewer approval is not authority. No self-approval, local official critic,
new escalation, subagent or actual RED/GREEN/native/gate claim follows here.

## Decision models

None. Provider `none` is complete. Manual stewardship is the opt-out.
Measurements are the exact fixture integration, WSL2 route and source-bound
completed gate logs, never a judgment or receipt-presence shortcut.
