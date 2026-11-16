# CTR-2, CTR-5 and CTR-7: one read-only queue interface

Artifact: `contributor-tooling-queue`, branch `lane/contract-contributor-tooling-queue`.
Source-only extraction from `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`.
This branch adds only `docs/contracts/contributor-tooling-queue.md`.
Companion citations resolve to that preserved source until individually
landed. Ports, assertions, prerequisites and authority choices are unchanged.

Status: draft for independent criticism, 2026-11-15. Contract only.
Base: `83226aece9b0f421381373f8dafc4a912d3888e5`, refreshed origin/main.
No script below exists on this base. No real RED, snapshot acquisition,
claim, expiry, dispatch or completion is asserted. Ownership and prerequisites:
`docs/contracts/contributor-tooling-split.md` (preserved source `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`). Trust is the REC-1 boundary in
`docs/contracts/contributor-tooling-trust-paths.md` (preserved source `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`), never candidate declaration.

## Settled decisions and current consumers

CTR-2/5/7 (`docs/plans/contributor-model.md:27,30,32,44-47`), CT-D2/3/6
(`docs/guides/contributor-lanes.md:37-92,123-140`), PL-D2/4/8/10/11 and
W8-2b settle read-only scripts and serial ownership. P2/P3/P17/P22 and I8/I19/I63
apply as in the trust contract. No provider, network client, credentials,
daemon API, scheduler or new Go package. No time-based hidden state.

`scripts/lane_queue.py` and `lane_queue_test.py` belong to CTR-2, then CTR-5.
CTR-7 imports their read-only interface from new `lane_checkin.py`; it cannot
edit queue files in its turn. The other consumers are CTR-4's Taskfile/CI
entrypoint and CTR-6's local handoff replay. No product CLI/MCP API is renewed.

Source reuse: [#139](https://github.com/GeoffMillerAZ/fathom/issues/139) §§1–2
defines priority/table order and first CLAIM after last RELEASE, §§3/7 caps
and stacked-work checks. [#140](https://github.com/GeoffMillerAZ/fathom/issues/140)
requests lane:ready, and its path-check proposal remains a comment, not an
amended issue. Current source has no lane_queue.py or lane:ready. The current
[#11](https://github.com/GeoffMillerAZ/fathom/issues/11) and #139 bodies still
prescribe merge/early-push/24-hour rules that conflict with CT-D3/5. CT-D3's
48-hour push rule is the proposed implementation contract, not a claim those
forge instructions have changed. ADR-111:7277 and CT guide Sequence require
the governing amendments before new-model dispatch. Comments never overrule
the body. No forge writes are part of this authoring lane.

## Exact proposed commands and pure calls

```text
python3 scripts/lane_queue.py ready --snapshot FILE --now UTC [--issue N [--session TOKEN]]
python3 scripts/lane_queue.py expiry --snapshot FILE --recheck FILE --now UTC
python3 scripts/lane_checkin.py --snapshot FILE --evidence FILE --now UTC
```

UTC is required RFC3339 UTC with `Z`; no current-time fallback. These commands
read files only and print a JSON object plus LF. No gh/git subprocess or forge
adapter inside queue/check-in. Trusted collectors are operator/integrator
work, outside the scripts' grant; a future collector is not implemented here.
CTR-4 passes the exact inspected snapshot bytes, not an issue-body executable.

Proposed Python interfaces, no I/O or input mutation in these functions:

```python
def evaluate_ready(snapshot: dict, now: datetime, issue: int | None = None,
                   session: str | None = None) -> dict: ...
def evaluate_expiry(snapshot: dict, recheck: dict, now: datetime) -> dict: ...
def collect_checkin(snapshot: dict, evidence: dict, now: datetime) -> dict: ...
def main(argv: list[str] | None = None) -> int: ...
```

The first two and queue main live in lane_queue.py; check-in and its main in
lane_checkin.py. Python stdlib only, required Python >=3.10. No implicit package
installer. Functions validate their complete input before returning a verdict. Every Source with complete=false or
error non-null yields named unknowns for its dependent decisions, never empty
healthy evidence. Complete=true still requires the interval/page checks below.
Malformed/duplicate keys, unknown closed-vocabulary members, NaN, negative
counts, bad dates/SHAs/refs and unexpected fields are input errors (exit 2),
never successful empty queues. Runtime-dependent missing evidence stays unknown.

## Snapshot protocol (ephemeral projection, never a second lane record)

All fields below are required unless explicitly nullable. Arrays never null.
`SHA` is lowercase 40 hex; `Digest` lowercase SHA-256. JSON bytes are UTF-8.
`Source` = `{id: string, observed_at: UTC, digest: Digest, complete: bool,
error: string|null}`. `id` is a public-safe opaque source id, not a local path
or token. Complete means all pages for the **specified population and interval**
were inspected, not a boolean inferred from receiving one API response.

Top-level keys:

| Key | Exact content |
|---|---|
| `format` | `fathom.lane_queue/v1` |
| `base_commit` | Trusted main SHA matching REC-1 authority |
| `source` | Source for the complete capture |
| `issue_pages` | Array of `{cursor: string, next_cursor: string|null, source: Source, issues: [Issue]}` |
| `comment_pages` | Array of `{issue: int, cursor: string, next_cursor: string|null, source: Source, comments: [Comment]}` |
| `queue_order` | Ordered unique issue numbers from the inspected #139 body, with body Source; object `{issues: [int], source: Source}` |
| `rows` | Array of `{plan: relative_path, id: string, base_commit: SHA, state: proven|open|deferred|unknown, source: Source}`; verified proof evidence, never checkbox inference |
| `lanes` | Array of `{manifest: REC1_lane_object, manifest_blob: git_blob_SHA, validation: valid|invalid|unknown, source: Source}` from the trusted REC-1 export, byte-bound to that base |
| `groundings` | Array of Grounding below |
| `push_histories` | Array of PushHistory below |
| `pull_requests` | `{source: Source, records: [{number: positive_int, issue: positive_int or null, head_commit: SHA, base_commit: SHA, state: open or closed}]}`; complete open-PR coverage for CTR-4 |
| `operator` | `{session: opaque_string, actor_role: worker|integrator, machine_role: opaque_string, os: linux|macos|wsl2, heavy_in_use: int|null, heavy_limit: int, spend_permitted: bool|null, source: Source}` |

`Issue` = `{number: positive_int, author: opaque_account, state: open|closed,
body_digest: Digest, updated_at: UTC, labels: [string], assignees: [opaque_account],
lane_id: slug|null, needs: [{plan: relative_path, id: string}],
machine: any|macos|linux|windows-ok, size: size-s|size-m|size-l}`.
The lane_id binding and parsed needs/machine/size are trusted steward projection
of the exact inspected issue body, not permission from worker text. Rows and
class/owned_paths/contracts/proof/budget/harness/id ranges come from REC-1,
not copied issue fields with a second schema. Extra unrelated forge labels are
retained but do not grant eligibility. Duplicate/conflicting priority or size
labels refuse eligibility instead of picking one arbitrarily.

`Comment` = `{id: positive_int, created_at: UTC, author: opaque_account, body: string}`.
Comments are sorted by `(created_at, id)` after validating complete cursor
chains. Deduplicate identical repeated ids at page boundaries. Same id with
different bytes is UNKNOWN. Every page chain has one first cursor `first`, no
cycles/gaps, and exactly one null terminal. Missing issue/comment pages prevent
global cap, session, overlap and winner proof; withhold eligibility rather than
assuming unseen claims do not exist. Deleted/edited claim history without an
authoritative complete source is unknown, never reconstructed from prose.

`Grounding` = `{issue: int, author: opaque_account, reviewer: opaque_account,
base_commit: SHA, body_digest: Digest, manifest_blob: git_blob_SHA,
contracts: [{path: relative_path, digest: Digest}],
claims_digest: Digest, checked_at: UTC, proof: {command: string, exit: int,
kind: assertion_red|environment_error|green|unknown, evidence: Source},
checks: {rows: bool, needs: bool, paths: bool, premise: bool,
machine: bool, size: bool}, source: Source}`.
The author equals the issue's author, reviewer differs. `claims_digest` is
SHA-256 of compact sorted-key JSON over current issues' number/labels/assignees
and ordered CLAIM/RELEASE comments (UTF-8, separators comma/colon, LF absent).
Recompute, never accept a worker-supplied validity bit. REC-1 validation belongs
to the shared checker and is cited by source, not reimplemented here.

Fresh means base/body/manifest/contracts/active-claims match the fresh trusted
capture and every evidence timestamp is <=now, not an invented TTL. Grounding
older than a cited changed source, unknown source, or mismatching digest is
stale. Snapshot is a recommendation at observed_at, never a live lock. CTR-4
must independently acquire/recheck current evidence before integration. No
arbitrary freshness window is introduced without governing policy.

`PushHistory` = `{issue: int, session: opaque_string, repository: opaque_repo,
branch: relative_ref, head_commit: SHA, observed_at: UTC,
coverage_start: UTC, coverage_end: UTC, source: Source,
pushes: [{id: opaque_string, at: UTC, head_commit: SHA}]}`.
Repository/ref/session bind to the winning claim, including fork identity.
Complete history covers claim→observation, including force updates and branch
creation. A provider's truncated events listing is not complete history.
Zero pushes with complete interval proves none; an empty inaccessible listing
does not. No `updatedAt`, commit-author/committer dates or comments are used.

Synthetic inputs in the tests use role tokens, never real account fixtures or
credentials. Actual source bodies and pagination may be stored under ignored
workspace, with digests in the reading. This author has read #11/#139/#140/#141;
it has **not** acquired complete queue, push, capacity or plan proof snapshots.
Do not label these proposed fields a currently available forge API response.

## CTR-2: readiness and claim mechanics

Evaluate all open issues for global claims before filtering `--issue`. An
eligible recommendation requires lane+ready, no assignee, and none of claimed,
blocked, needs-ruling, needs-grounding. Resolve every needs row to proven at the
same trusted base (closed issues and checked boxes do not substitute). Require
valid REC-1 manifest, independent matching intended assertion RED, inspected
contract files, <=one lane-day, compatible machine and no owned-path overlap
with any claimed/incomplete/raced claim. Absent dependencies, row identity,
grounding or capacity gives unknown/refusal as appropriate, never ready.

Take p0 then p1 then p2; within priority use queue_order. A lane absent from that
table is unknown-order and not auto-recommended. No date/issue-number tie-break
replaces the queue. Labels unsupported by this checker are informational.
`spend` requires explicit true; unknown/false blocks. macos requires macos.
linux permits Linux/WSL2, never infers a native Windows witness. L-S requires
trusted operator actor_role=integrator, never a general worker recommendation.
CT-D1 role assignment is operator configuration, not a new forge label.

CLAIM grammar is exactly CT-D3's full single line:
`CLAIM machine-role=ROLE branch=lane/N-SLUG base=SHA session=TOKEN`.
ROLE/TOKEN/SLUG are nonempty ASCII alphanumeric/hyphen tokens (first character
alphanumeric). N equals this issue. Release grammar is
`RELEASE TOKEN: REASON`, a nonempty single line. Interpret only these first-line
records; arbitrary mentions or fenced copies are not events. A legacy `CLAIM
TOKEN` or malformed record is a named ambiguous claim, never silently ignored
to make a lane eligible. A release token must refer to a claim in that issue's
current epoch, otherwise history is ambiguous. Following the queue's rule,
the first valid CLAIM after the last valid RELEASE is the tentative winner.
Full ownership additionally requires exactly one matching assignee (comment
author), claimed present, ready absent and fresh reread. An incomplete triple
or competing claim blocks work; the helper proposes stewardship, not a lock.

Same session winning two issues blocks both and names them. All claimed engine
lanes count before priority filtering. A two-engine race proposes release of
the higher issue number; neither gets work authorization from this output.
At five claimed language lanes, no sixth. Incomplete claims conservatively
block overlapping/global-cap proof until resolved. Unknown heavy_in_use or
capacity blocks. The general cap is four heavy processes per machine; the
operator may supply a stricter limit, never a larger one through this script.
Current wave W8 permits three concurrent package runs outside an exclusive
full gate; the operator supplies that stricter capacity. No process sampling.

Claimed continuation is frozen for CTR-4 now, in CTR-2's first turn:
`--session TOKEN` requires --issue N, exactly one complete winning triple for
that session/comment author, and no second winning issue. Require claimed
instead of ready, all normal dependencies/contracts/grounding/machine checks,
and fresh independent post-claim grounding bound to the new claims_digest.
Exclude only this same lane from its own overlap and engine/language count.
Other blocked/ruling/grounding labels still refuse. Wrong token, incomplete or
raced claim refuses. No author self-refresh and no release/reclaim requirement.
The current snapshot's issue/lane/session binding remains trusted input, never
permission minted by the worker. This proof is a recommendation, not a lock.

Output keys: `format`=`fathom.lane_ready/v1`, `base_commit`, `observed_at`,
`recommendations`, `claims`, `unknown_sources`.
Recommendation keys: `issue`, `lane_id`, `status`=`eligible|ineligible|unknown`,
`reasons` (sorted strings), `evidence_ids` (sorted source ids).
Claim keys: `issue`, `session`, `status`=`complete|incomplete|raced|unknown`,
`winner_comment_id` (integer or null), `reasons`. Recommendations stay in queue
order, unknown-order issues follow sorted by number. Empty arrays are `[]`.
Exit 0 for all requested issues eligible (or a complete unfiltered report), 1
for a checked requested ineligible issue, 2 for an unknown requested issue,
incomplete global report, invalid inputs or usage. Unfiltered complete report
may contain ineligible issues; its 0 is not permission for any one issue.

## CTR-5: expiry proposals and recheck

Use the same validated complete winner/triple. Set age origin to max(claim
created_at, last push since claim), or claim created_at with proved no pushes.
Exact threshold is 48*3600 seconds, inclusive. Below it retain; at it propose
release. Future events/negative age invalidate input. Missing push history,
missing branch/head, wrong repo/ref/session, retention gaps or incomplete
coverage yield age unknown and no proposal. A missing branch is not proof no
push happened.

Recheck FILE must be a new complete same-schema snapshot, observed no earlier
than the initial one, covering pushes through its observation and the same
claim epoch. Changed session, labels, assignee, claim winner, branch head or
new push vetoes the old proposal and names the changed evidence. Recompute
age at the supplied now against the recheck, never release from initial data.
The actual steward must recheck again immediately before any manual action.
This is still vulnerable to events after observation; it claims no atomicity.

Output keys: `format`=`fathom.lane_expiry/v1`, `observed_at`, `proposals`,
`retained`, `unknown_sources`. Proposal keys: `issue`, `session`, `repository`,
`branch`, `head_commit`, `last_push_at` (UTC or null only for proved no push),
`age_seconds`, `evidence_ids`, `comment`, `next_labels`.
Comment bytes: `RELEASE TOKEN: no push for 48 hours`. next_labels is
`["needs-grounding"]`; restore ready only after **new** independent grounding.
The output also directs removal of claimed/assignee, preservation of branch
and continuation SHA as advisory text. No command runs and no action is posted.
Exit 0 for a complete report, 2 for missing/unknown required history or misuse.
Retained is an array of `{issue, reason, age_seconds: int|null}`.

## CTR-7: fixed check-in, not a ruling engine

`--evidence` closed object has `plan_status`, `ci`, `readings`, `events`.
Each is `{source: Source, base_commit: SHA, records: [...]}`; sources and
SHA binding are mandatory. Plan_status additionally carries `indexed_root` (public-safe opaque root id),
`indexed_commit: SHA`, `index_fresh: bool|null`. False/null freshness or root/
commit mismatch makes the plan counts unknown. CI additionally carries
`expected_heads: [{branch, head_commit: SHA}]`, independently supplied by the
collector, to reject a success on an older head. Plan records = `{plan, row, state:
proven|open|deferred|unknown}`, never raw local root paths. CI records = `{branch, head_commit, run_id,
conclusion: success|failure|cancelled|pending|unknown}`. Reading records =
`{path, input_commit, verdict, preregistration_path, preregistration_digest,
bar, evidence_id}`; these are attestations, never promoted to verified plan
rows. Event records = `{id, at, kind: wave_boundary|checkin, evidence_id}`.
All referenced inputs must match the base/head they describe; stale/missing
sources are unknown. No free activity prose or commit count becomes completed.

Return keys: `format`=`fathom.lane_checkin/v1`, `base_commit`, `observed_at`,
`plan_status`, `queue_counts`, `overlaps`, `ci`, `readings`, `claim_ages`,
`reminders`, `unknown_sources`, `source_digests`.
Counts are by p0/p1/p2/unknown-priority and ready/blocked/claimed/needs-ruling/
needs-grounding, preserving label overlaps, not summing them into a total.
Plan_status counts the four states, with unknown source yielding unknown
counts, not zero. Each CI head needs an exact run or explicit unknown. Claim
ages reuse CTR-5's push rule; no recheck means no release proposal. Every
section carries its source ids/digests/base or relevant head.

Weekly due = latest checkin+7*24h <=now. A wave_boundary after the latest
checkin makes wave due independently. No checkin event means unknown weekly
due; no complete events means unknown wave due. No guessed last checkin or
cron. Architect inspects and posts on #141, outside these commands. Exit 0
for a complete report; 2 when any required source/section is unknown. Known
non-green CI appears as failure in the report, not a fabricated completeness.

## Frozen synthetic clock and prospective assertions

Implementing tests create fixtures in their owned test files, no additional
fixture directory grant. One valid base snapshot has a single open issue 140,
lane+ready+p0+size-s, no assignee, no comment events, trusted lane `demo-queue`,
one independently witnessed intended RED, empty needs and disjoint exact path
grant. operator=session `candidate`, actor_role worker, machine_role `helper`, os linux,
heavy_in_use 0, heavy_limit 3, spend_permitted false. Complete single issue and
comment pages use cursor first→null. queue_order=[140]; pull_requests is a
complete empty records list. Grounding and every
source observed at `2026-11-13T00:00:00Z`; all body/contract/claim digests are
computed from the frozen synthetic bytes. Freeze their actual hashes in the
fixture reading. Use the real contract base SHA for source-binding tests,
never label this synthetic case a real grounded issue #140.

For expiry variant remove ready, add claimed, sole assignee `worker-a` and
winning full CLAIM by worker-a, session `claim-a`, branch `lane/140-demo`, at
`2026-11-13T00:00:00Z`. Push source explicitly covers that instant through
`2026-11-15T00:00:00Z`. Proved no-push case has pushes=[] and an existing
branch head. Its exact boundary ages are 172799 and 172800 seconds, using
now `2026-11-14T23:59:59Z` and `2026-11-15T00:00:00Z`. A recheck push at
`2026-11-14T23:59:59Z` vetoes release. Git fixture SHAs are generated and
recorded from actual fixed-content fixture objects, not invented real heads.

All tests below are prospective and each assertion is mandatory (W3-2).

| File::test | Assertions |
|---|---|
| lane_queue_test.py::`test_ready_and_grounding` | Valid case eligible; missing row, absent contract, unsatisfied/deferred need, green proof, environmental proof, self-grounding, changed base/body/contract/claims and size-l each withhold; evidence id/reason names failure |
| `test_pages_and_order` | Two-page issue/comment snapshots produce same result as one page; boundary duplicate id dedupes; conflicting duplicate/cycle/gap/missing terminal yields unknown; p0→p1→p2 then frozen table order; absent queue member is unknown-order |
| `test_claim_epoch_and_triples` | First CLAIM after last RELEASE wins by timestamp/id, not page order; stale previous epoch does not win; missing assignee/label/comment or ready retained blocks; wrong comment author/session/branch/issue/legacy grammar is ambiguous; second claimant forces stewardship |
| `test_session_engine_language_caps` | Same session on two issues blocks both; raced engine pair proposes higher-number release; fifth language active blocks sixth; filtering --issue cannot hide global claims or incomplete pages |
| `test_machine_spend_overlap` | Unknown/full machine capacity and spend unknown/false block; WSL2 never permits macos; L-S not recommended; overlaps include exact new fragment/plan/epoch grant; missing lane validation is unknown |
| `test_claimed_continuation` | Correct winning session passes; wrong token, double winning issue, self-refresh, missing post-claim independent grounding and raced/incomplete triple refuse; own overlap/cap excluded exactly once |
| `test_pull_request_projection` | Complete ordered PR projection retains exact issue/head bindings; incomplete pagination/multiple issue PRs/missing object source remain unknown; no substring binding from PR title |
| `test_read_only_and_schema` | Frozen bytes unchanged; no network/forge/subprocess permitted; credentials absent; invalid JSON keys/enums/dates/future timestamps/NaN fail 2; permutation preserves defined order and sorted diagnostics; arrays are [] and no ANSI under pipes/NO_COLOR |
| `test_expiry_boundaries` (CTR-5 turn) | 172799 retains, 172800 proposes; no-push origin is claim; fresh push resets; comments/commit dates/PR update do not reset; incomplete/absent/wrong-ref/missing-branch history unknown; no mutation |
| `test_expiry_recheck` (CTR-5 turn) | Racing push/new head/changed winner/session/labels/assignee veto old proposal; stale recheck unknown; exact proposed comment and grounding-required labels; branch/head preserved; no action executes |
| lane_checkin_test.py::`test_checkin_sources_and_counts` | Known counts match exact fixture population and source SHAs/digests; overlapping labels counted independently; absent/stale source unknown, never zero or complete; CI run on prior head does not satisfy current head; reading verdict does not prove row |
| `test_checkin_claim_age_and_reminders` | Same push/no-push/unknown age rules; comments cannot reset; weekly at exactly seven days due, one second below not due; unacknowledged wave boundary due; missing events/checkin unknown |
| `test_checkin_read_only` | Missing required sections exits 2 with partial report; no prose-as-proof, credential, ANSI, posting, provider, git mutation or input write; arrays/determinism preserved |

```sh
python3 -m unittest discover -s scripts -p lane_queue_test.py
python3 -m unittest discover -s scripts -p lane_checkin_test.py
```

Implementers capture actual RED after tests exist and GREEN on their exact
head. Critic checks every assertion. Only those observations permit the
plan's exact CTR-2/5/7 PASS lines. This draft supplies neither real inputs nor
proof. Independent critic and governing-amendment prerequisites remain open.

## Decision models

None applies. Provider `none` is the full deterministic behavior. Opt-out is
manual stewardship. Fixtures measure false-ready, false-release and false-
complete rates; no judgment replaces a missing source.
