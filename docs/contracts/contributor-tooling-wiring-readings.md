# CTR-4, CTR-6, CTR-8 and CTR-9: wiring and honest witnesses

Artifact: `contributor-tooling-wiring-readings`, branch `lane/contract-contributor-tooling-wiring-readings`.
Source-only extraction from `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`.
This branch adds only `docs/contracts/contributor-tooling-wiring-readings.md`.
Companion citations resolve to that preserved source until individually
landed. Ports, assertions, prerequisites and authority choices are unchanged.

Status: draft for independent criticism, 2026-11-15. No implementation.
Base: `83226aece9b0f421381373f8dafc4a912d3888e5` (refreshed origin/main).
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

Current base facts, source citations rather than capability claims:

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

## CTR-4: exact local Taskfile boundary

Only Taskfile.yml, ci.yml and wiring.md may change. No implicit grant to
lefthook.yml, AGENTS.md, autonomy, guide, other scripts or workflow files.
The integrator owns this L-S turn. CTR-2/3 **implementations** and their source
contracts must be on actual main, not just in an author branch or receipt.

`task lane:ready` requires these operator environment values, no defaults:

| Variable | Exact meaning |
|---|---|
| `FATHOM_LANE_TRUSTED_BASE` | Full immutable base SHA, shared with REC-1 |
| `FATHOM_LANE_ID` | Trusted REC-1 slug, shared with REC-1 |
| `FATHOM_LANE_HEAD` | Exact clean candidate commit SHA |
| `FATHOM_LANE_SNAPSHOT` | Inspected queue snapshot path, under ignored workspace |
| `FATHOM_LANE_NOW` | Explicit RFC3339 UTC clock for queue recommendation |
| `FATHOM_LANE_ISSUE` | Positive issue number from trusted steward binding |

The snapshot's base, issue→lane mapping, manifest blob and grounding must match
these independent authority values. Issue body or candidate env files cannot
supply them. Both path checker and queue checker run before any candidate proof
command. This local task assumes an honest operator running the reviewed tools;
the fork CI authority boundary below is a separate obligation.

Proposed execution order, no `continue-on-error`, swallowed status or optional
check when an input is absent:

1. Freeze clean root/head/tree and trust values; run
   `python3 scripts/lane_paths.py --root . --head "$FATHOM_LANE_HEAD"` and
   `python3 scripts/lane_queue.py ready --snapshot "$FATHOM_LANE_SNAPSHOT"
   --now "$FATHOM_LANE_NOW" --issue "$FATHOM_LANE_ISSUE"`. Any 1/2 blocks.
   A ready label alone cannot satisfy this check. For an already claimed lane,
   use the **same** ready evaluator's claimed-owner continuation described
   below, never require a worker to release/reclaim merely to run its check.
2. Independently ground #140's commit checks over immutable trusted-base..head,
   including every nonmerge commit, with `bash scripts/commit-msg-check.sh`
   reading a workspace temp file containing the exact message. Use git argv
   arrays / quoted shell variables. Never interpolate issue proof text into
   shell. Freeze the script from the reviewed base for CI authority checks.
3. Stacked-work check uses each **other** open PR's current head from the
   complete trusted snapshot extension below: `git merge-base --is-ancestor
   OTHER_HEAD HEAD`. If true and OTHER_HEAD is not already an ancestor of
   trusted base, refuse. Missing object/history is unknown and blocks. Same
   issue's own PR is excluded by trusted binding, not title substring.
4. Retain #140 checks: real `command gofmt -l` over tracked Go files (not a
   shell function; reject output), `go run
   ./third_party/agentic-kit/cmd/release-notes -check`,
   `bash scripts/check-merge-debris.sh`, `task tenancy-lint`, and
   `task test:affected`. Existing full gate already tests isolation; retain
   that coverage without inventing the X-R10 selector that #140 doesn't name.
   Run the exact REC-1 `proof_command` loaded from the trusted manifest only,
   in the unprivileged candidate execution context. Its bytes are approved
   command text, invoked as argv `["bash", "-c", trusted_proof_command]`.
   Do not substitute proof text from issue comments/body or candidate CUE.
   This is never executed in the trusted data-only preflight job.
5. Run the existing full `task validate` under its existing machine lease,
   then static strict plan verification on a fresh scratch ingest of this
   exact head, using the built candidate binary, all active top-level plans
   except README.md, and `--since` the immutable base. This follows the
   current integrator proof in agent-waves.md:92-95; no shared daemon store.
   The exact proposed commands are below. `fathom verify` is not a test runner and lexical receipts cannot prove
   execution. Unknown/operational proof is reported as unknown and prevents
   **this readiness claim**; existing global verify fail-open stays unchanged.
6. Verify head/tree/clean state again and attach command, exit, source hashes
   and current head to the receipt. Any changed head invalidates all results.

Exact scratch verification command sequence, UNRUN here, after the full gate
has produced the candidate `bin/fathom` (the task checks binary source identity):

```sh
lane_ready_tmp=$(mktemp -d workspace/tmp/lane-ready.XXXXXX)
bin/fathom ingest --store "$lane_ready_tmp/verify.sqlite" --service fathom --path .
lane_ready_args=()
for lane_ready_plan in docs/plans/*.md; do
  [ "$lane_ready_plan" = docs/plans/README.md ] && continue
  lane_ready_args+=(--plan "$lane_ready_plan")
done
bin/fathom verify --strict --since "$FATHOM_LANE_TRUSTED_BASE" \
  --store "$lane_ready_tmp/verify.sqlite" --service fathom "${lane_ready_args[@]}"
```

Taskfile executes this sequence with Bash and immediate error propagation.
The unique scratch directory stays under workspace for logs/critic inspection.
No shell expansion of filenames into proof command text. Gofmt input uses
NUL-aware tracked Go path enumeration and argument arrays, not a whitespace
split. Empty active-plan set is a missing-input failure, never a vacuous pass.

For CI a runner is exclusive: keep existing `task validate:unleased` as today.
Run the extra queue/path/commit/stack preflight in a separate preceding job,
not a nested full `task lane:ready` that repeats the full suite. Both the
preflight and the existing full CI must pass on the exact candidate head to
form the combined lane-ready result. Push jobs retain today's full suite even
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
The controller independently injects the admitted issue/lane/session mapping
and byte/digest-bound queue snapshot; PR body/branch/env cannot choose them.
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
inputs under U1; these command templates claim no existing acquisition API.

Candidate build/test job uses that exact PR head, persist-credentials:false,
no secrets or token environment, and the ordinary read-only job permissions.
It runs full existing validation, including untrusted tests, in its disposable
unprivileged runner. Do not interpolate untrusted branch names into shell.
Public artifacts carry relative source spans and role ids only.

An arbitrary fork can edit its workflow. A check printed by candidate code
does not establish trusted enforcement. ADR-111:7277 explicitly leaves required
checks/branch protection unknown. CTR-4 must witness the controller and required
check configuration or report it missing. It cannot certify bypasses=0 by
examining proposed YAML alone. U1 below prevents an invented trusted admission
service or privileged workaround from being smuggled into this contract.

## Prospective CTR-4 integration assertions

Tests are named tasks inside the owned Taskfile (no new script/test-file grant):
`lane:contract-test` builds local git fixtures under workspace/tmp from the
frozen real base with REC-1 and CTR-2/3 actually landed. Freeze all fixture
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
| `valid` | Actual `task lane:ready` exit 0 on frozen clean head, full logs and strict verify receipt; no omitted step |
| `outside` | Outside addition/deletion and either rename endpoint fail paths before candidate proof; owner/path in output |
| `self-broaden` | Candidate manifest/schema/script substitution cannot authorize the outside file; missing trusted manifest fails |
| `claimed` | Winning session with independent refreshed grounding passes preflight; wrong/incomplete/raced token fails |
| `stale` | Changed body/base/head/contracts/capacity/claims or absent source prevents pass; older green CI head cannot satisfy |
| `stacked` | Another open unlanded PR head ancestor refuses; already-on-base ancestor doesn't; unavailable object fails |
| `fork` | Trusted script runs from control base; candidate script containing bypass code not invoked there; no persistent credential/secrets/write permission; modified workflow cannot establish trusted required-check proof |
| `gates` | Existing validate dependency set, perfgate/test/race/symlink temp canary, pre-push and message checks remain; branch CI still runs full gate; each seeded step failure is named and nonzero |

Proof commands, all UNRUN here:

```sh
python3 -m unittest discover -s scripts -p lane_queue_test.py
python3 -m unittest discover -s scripts -p lane_paths_test.py
task lane:contract-test
task lane:ready
```

CTR-4's exact PASS line needs actual local replay **and** fork/controller
evidence satisfying every assertion, plus critic inspection of logs. No receipt
presence or configuration-only inspection substitutes for integration proof.

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
This is not WSL2 evidence and cannot close CTR-8.

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
lease is available, not to this author and not to CTR-9.

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

## Remaining authority questions, queued for foreman

Each is <300 characters. They are not an escalation file, approval or dispatch.
Other exact contracts/readings can proceed independently of these choices.

* U1: Trusted CI lane/session/snapshot admission and required-check enforcement
  have no current source. Recommend immutable integrator admission plus a
  base-controlled required check. Name the supplying channel/owner.
  Alternative: manual H0 hold, no automated fan-out.
* U2: PL-D6 requires record vet, but only gate record-ids exists; REC-7 follows
  CTR-12 which needs CTR-4. Recommend explicit H0 ruling retaining current full
  gate+REC-1 ownership checks until REC-7, without calling that rec vet.
  Alternative: hold automatic fan-out for REC-7.

ADR-111/CT-D Sequence's autonomy/#11/#139 amendments are known prerequisites,
not additional design questions. The author cannot make them in this prefix. Reuse approved ctr-11.md and
records-integration.md for REC-7 ownership/CLI; do not replace their vet law.
Independent contract LAND still required under W2-3/W3-4. Foreman batches any
unsettled points under W8-4 and checks actual F5/main prerequisites after LAND.

## Decision models

None. Provider `none` is complete. Manual stewardship is the opt-out.
Measurements are the exact fixture integration, WSL2 route and source-bound
completed gate logs, never a judgment or receipt-presence shortcut.
