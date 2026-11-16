# CTR-1 and CTR-3: trusted grants and path check

Artifact: `contributor-tooling-trust-paths`, branch `lane/contract-contributor-tooling-trust-paths`.
Source-only extraction from `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`.
This branch adds only `docs/contracts/contributor-tooling-trust-paths.md`.
Companion citations resolve to that preserved source until individually
landed. Ports, assertions, prerequisites and authority choices are unchanged.

Status: draft for independent criticism, 2026-11-15. Contract only.
Base: `83226aece9b0f421381373f8dafc4a912d3888e5` (refreshed origin/main).
Ownership/turns: `docs/contracts/contributor-tooling-split.md` (preserved source `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`). All proposed scripts,
fixtures and tests below are absent on this base. No actual RED or reservation.

## Intent and authority

P2/P3/P15/P17/P20/P22 require deterministic, explicit, owned, computed evidence.
I8 preserves static verify and its 0/1/2 exits. I19/ADR-011 prohibit private
operator identifiers. I63 and the dispatch non-goal keep this outside fathom's
runtime: no assignment, forge write, branch mutation or scheduler. These are
existing constraints. CT-D1–8 and ADR-111 remain proposed records of the owner
direction that W8-2b explicitly asks this tooling to serve.

Trigger already identified in ADR-111: autonomy's CI/contributor revisit needs
architect reconciliation. This contract does not amend autonomy or issues.
No new evidence fires a future promotion. F92 forbids host scheduling/load
sampling. The free option is standalone scripts and exact grants, without a
new package, persistent lane format, id family or language-door import.
W2-1 remains door 80, consumer 81 with no headroom or accounting change.

## CTR-1: the steward's handed checklist

CT-D2 (`docs/guides/contributor-lanes.md:37-59`), the lane form
`.github/ISSUE_TEMPLATE/lane.yml` and CTR-1 (`contributor-model.md:26`) settle
the checklist. Its only product is the assigned reading. A label is not proof.
For each example record issue-body digest, author and independent steward as
public-safe opaque role ids, trusted base, plan path/id, resolved needs,
contract paths/digests, exact owned paths, actual proof command/exit/assertion,
claimed-lane inventory/digest, machine/size and source spans. A missing source
is UNKNOWN, never ready. Size-l must split. A new claimed overlap invalidates
the earlier grounding. Freeze examples before the independent steward reads.

| Frozen example | Required decision and evidence |
|---|---|
| valid | Different author/steward; existing rows/contracts at base; satisfied needs; independently witnessed intended RED; exact disjoint grants; permitted machine and <=one day. READY |
| missing-row | The named plan exists but requested row does not. UNREADY with path/id |
| absent-contract | Named contract absent at base. UNREADY with git-object lookup failure |
| green-proof | Actual proof exits 0 at the stated base. UNREADY, repair or close |
| environmental-error | Proof fails for absent executable, credentials or dependency. UNKNOWN/UNREADY, never intended RED |
| overlap | Claimed trusted lane owns one requested file, including an allowed addition. UNREADY naming lane/path |
| self-grounding | Author and steward identity equal. UNREADY even when every other fact passes |
| stale-grounding | Base/body/contracts/claims differ since inspection. UNREADY pending fresh independent inspection |

The implementing/reading lane supplies real evidence, not this draft. Record
each example's decision and evidence, count false-ready decisions, and only
then emit `CTR-1 verdict: PASS false-ready=0`. Run the plan's verify command
after filing; its lexical match proves receipt presence only. The critic checks
every example. CTR-1's no-needs row can proceed before REC-1 implementation
using the approved contract; CTR-2/3 still wait for actual REC-1 on main.

## Trusted manifest interface shared with REC-1

REC-1 D1/D4 and `docs/contracts/rec-1.md:177-203,246-258` settle the source:

* Required operator environment: `FATHOM_LANE_TRUSTED_BASE` is a full lowercase
  40-hex immutable commit, `FATHOM_LANE_ID` is the REC-1 slug. No trust-ref flag,
  default to HEAD, candidate base field or candidate branch-name derivation.
* Read only direct regular `.cue` blobs under `docs/lanes/` at that commit.
  Filename equals `lane.id`. Decode `lane: #Lane & {...}` against the schema
  at the same revision. Keep REC-1 field names and closed shape verbatim.
  `base_commit` is the lane's grounding base, not authority to choose trust.
* Freeze `cue.mod/module.cue`, `schema/records/schema.cue`,
  `schema/records/lane.cue` and direct lane blobs by git-object reads into an
  isolated temporary module under `workspace/tmp/`. Run the pinned `cue export
  --out json docs/lanes/<slug>.cue -e lane` there, never from candidate CUE imports.
  Only these trusted local imports are allowed. No fetch or registry download.
  Missing schema/import/tool is UNKNOWN, not a hand-written CUE parser fallback.
  This is an ephemeral REC-1 projection, not another authored record format.
* Compare every trusted lane/schema/module blob to the candidate HEAD bytes.
  A missing, changed, symlinked or added direct lane manifest fails ordinary
  lane checking. An integrator grant update is a separate prior base commit,
  never an exception the current lane can approve. Report both blob ids.
* Require the trusted base and the selected lane's `base_commit` to be ancestors
  of HEAD. Failure/ambiguous merge base is a refusal. A fresh integrator grant
  commit becomes part of the worker's base before work starts.
* Exact normalized `owned_paths` only, including fragment/sibling-plan/epoch
  allowances explicitly enumerated there in advance. No new schema fields,
  pattern grants or automatic expansion of `docs/changelog/unreleased/`,
  `docs/plans/` or `internal/fusion/epochs/`. Read-only and contract paths never
  grant writes. Shared schema, generated outputs and record aggregates require
  their own prior integrator turn. No implicit active reservation.

REC-1's checker retains full inventory, effective trusted policy, grandfather
pairs and opaque `ValidatedInventory`. These scripts neither allocate ids nor
reimplement that checker. Queue ready consumes its cited validation outcome,
not a worker's raw inventory or a newly invented allocator signature. The
path checker proves paths only; the record gate independently proves ids.

PL-D5's three allowances are exact entries, not wildcard permissions. Example
synthetic grant: `docs/changelog/unreleased/demo-paths.md`,
`docs/plans/demo.paths.md`, `internal/fusion/epochs/demo-paths.md`.
The fixture integrator chooses these before the worker starts. A plan fixture
containing an id also needs a REC-1 reserved range validated by REC-1. This
example creates no real grant and no new family.

## Proposed path command and mechanics

```text
python3 scripts/lane_paths.py --root DIR --head FULL_SHA
```

Both environment authority values above are required. Root resolves to one
git worktree. HEAD must resolve exactly to `--head`. Freeze root/base/head/tree
at entry, check them again at exit. Reject a dirty index, tracked worktree
change or nonignored untracked file rather than pretending it was checked.
`workspace/` ignored scratch is permitted. Git subprocesses use argument
arrays, no shell interpolation, `--no-ext-diff` and `--no-textconv`.

Compute the complete trusted-base→HEAD tree delta, not just HEAD's last commit:
`git diff --no-ext-diff --no-textconv --name-status -z --find-renames BASE HEAD --`.
Read status and paths as NUL-delimited bytes. A/D/M/T/U/X/B and R/C with scores
are recognized; U/X/B refuse. R/C consume and check both endpoints, including
the source of a copy. Reject truncated/unknown status records. A rename shown
as D+A without similarity still checks both paths. Sorted diagnostics are by
raw path bytes, then status/reason/lane. Escape controls for display or JSON,
never split paths on newlines, tabs or spaces and never pass paths as options.

A repository path must be relative with slash separators, nonempty components,
no empty/`.`/`..` component, NUL, backslash, absolute/drive/UNC spelling or
trailing slash. Do not normalize an invalid spelling into an allowed one.
Spaces, Unicode and embedded LF/tab remain literal git names, JSON escaped.
Use exact byte identity with UTF-8 manifest strings. Invalid UTF-8 git names
refuse with an escaped path diagnostic rather than disappearing.

Build exact path→lane map from **all** trusted lanes, not merely currently
claimed ones. Duplicate grants and file/descendant grant pairs fail before
matching (a file cannot also be a directory grant). Any outside path refuses,
naming another owning lane where known and `unowned` otherwise. This includes
deletions and both rename endpoints. Inspect changed leaf modes in both trees:
reject symlink/submodule targets and a symlink/submodule ancestor in either
tree. Read git objects, never dereference candidate filesystem links.

No CODEOWNERS match grants a write. `internal/ownership/ownership.go:1-22,134`
is review routing, with last-match precedence and literal directory cascading.
Do not copy or call its wildcard matcher from Python. The compatible subset
is root-anchored literal **file** identity. Freeze parity cases against
`ownership_test.go::TestFor_LeadingSlashAndAnyDepthEndToEnd` and
`TestFor_LastMatchWins`: `/scripts/demo.py` matches that file in both systems,
not `nested/scripts/demo.py`. A child of that literal path, `*.py`, `scripts/`,
`**`, `!`, bracket syntax or an unanchored basename never becomes a grant.
Document intentional differences instead of approximating CODEOWNERS.

## Output, exits and prospective tests

All three scripts use JSON on stdout, no ANSI, no timestamps from an internal
clock, explicit sorted diagnostics and arrays `[]`, never null. Diagnostics
contain public-safe repo-relative paths, not root's absolute spelling.
Path result keys: `base_commit`, `head_commit`, `lane_id`, `status`, `violations`,
`unknown_sources`. Violation keys: `path`, `change`, `owner`, `reason`.
`owner` is a trusted slug or `unowned`; status is `pass|refuse|unknown`.
Reasons are `outside_path|overlap|invalid_path|unsafe_mode|trust_changed|
dirty_tree|unmerged|invalid_diff|base_mismatch`. Unknown sources name the
missing git object, manifest, schema or CUE tool without leaking local paths.
Exit 0 passes, 1 is a witnessed violation, 2 cannot check/usage. Both 1 and 2
block lane eligibility. This does not alter fathom verify's fail-open rule.

All tests below are prospective in the exact owned `lane_paths_test.py`:

| Test name | Required assertions |
|---|---|
| `test_trusted_manifest` | Missing env/base/lane/schema refuses or unknown nonzero; malformed CUE and new fields fail; all blob ids are from trusted base; candidate self-broadening, new lane and schema substitution fail even if newly added file would otherwise match |
| `test_exact_allowances` | Each exact granted fragment/plan/epoch passes; sibling name and every directory-wide expansion fail; no read-only/contract permission; no live reservation is inferred |
| `test_all_change_kinds` | A/M/D/T inside grant pass with safe modes; outside A/M/D/T fail; R/C check both endpoints; similarity-off D+A cannot bypass; U/X/B and malformed/truncated NUL records fail |
| `test_path_bytes` | Space/LF/tab/Unicode/leading-dash names remain one path; traversal/absolute/drive/UNC/backslash/noncanonical names fail; invalid UTF-8 is diagnosed, not dropped |
| `test_modes_and_overlap` | Leaf and ancestor symlinks/submodules in either tree fail; duplicate grant and file/descendant grant fail; cross-lane additions name owner; unowned path names unowned |
| `test_literal_matcher_parity` | Shared root-literal positives/negatives agree with pinned ownership fixtures; wildcard/unanchored/directory CODEOWNERS cases explicitly diverge and never grant a write |
| `test_full_range_and_dirty` | An outside write in an earlier commit remains refused; wrong HEAD, missing ancestry, shallow missing object, dirty tracked/index/untracked state and head changed during check are nonzero |
| `test_deterministic_read_only` | Permute lane and diff order; same sorted JSON; git index/refs/worktree byte-identical; subprocess allowlist rejects forge/network/write-git attempts; empty arrays serialize as [] |

Fixture repository is created by the implementing tests under ignored
`workspace/tmp/`, with fixed author/committer dates and safe synthetic identity.
It contains the exact REC-1 schema/module and demo/other lane grants plus the
three exact allowance paths above. Freeze tree/blob ids and fixture contents
before the implementation; log their actual ids, never substitute made-up
commit hashes. No test writes outside its fixture root. Native fixture inputs
not supplied remain missing. W3-2 requires actual failing assertions after
these named tests exist, then GREEN; absence of a test file is not RED.

```sh
python3 -m unittest discover -s scripts -p lane_paths_test.py
```

Only after all assertions pass file `CTR-3 verdict: PASS outside-writes=0`.
No tests, measurements, proof receipts or integration branches ran here.
Approval requires independent LAND and settling citations (W2-3). Fork authority wiring is separately
listed in `docs/contracts/contributor-tooling-wiring-readings.md` (preserved source `a598d4bbfcecbcc0108164382dc2a22bf6c915eb`).

## Decision models

None. Provider `none` is complete deterministic bookkeeping. Manual stewardship
is the opt-out, without claiming the automated gate passed. Fixture refusals
and actual path-gate replay measure correctness.
