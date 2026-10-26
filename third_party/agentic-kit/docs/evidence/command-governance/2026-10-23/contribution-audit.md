# Unreachable contribution audit, 2026-10-23

Baseline: `0c38e199b8a3a48a1d91717f67278c06fb2492c2`. Read-only Git inventory: 117 unreachable commits, 46 actual tips and 13 roots. The tips below are the maximal commits in that unreachable graph; roots are not substituted for tips. Full per-tip first-parent, index/worktree and blob evidence: `/Volumes/m2ext/tmp/agentic-kit-tip-dispositions-20261023.json`. Raw inventory: `/Volumes/m2ext/tmp/agentic-kit-contribution-fsck-20261023.log`.

Disposition: 12 integrated, 33 superseded, 1 empty, 0 review-needed. No material behavior absent from current HEAD was identified. This is source/tree comparison, not a test or runtime qualification.

## Method and findings

- Compared every tip to its first parent and HEAD. For merge-shaped WIP commits, compared first parent to index parent and index parent to worktree tip separately. `ade08f0b89` has no tree delta. Recent stashes `01d24ac3b2` and `04cc540378` have no index delta; their worktree files exactly match HEAD.
- The `b424a95e24` staged index differs from its worktree tip in constitution-lint and read-manifest source. The worktree restores read-manifest, and HEAD further hardens the linter; the staged draft is not a missing source contribution. Other overwritten index paths are changelog, tracking and intent-index documentation.
- Historical September WIP tips commonly include deletion of old `.agents/skills/intent-*`, a `CLAUDE.md` shim, and stale `TRACKING.md`/plan edits. These are not safe policy contributions to replay. Current instruction layout and later source changes govern.
- Material divergent source was checked against current implementation: HITL typed-nil store rejection, sourced prose UTF-8/freshness, benchmark timestamp JSON round-trip, reuse plans that keep historical calculations separate from required reruns, and native-host/CLI evolution. These are subsequent hardenings, not missing old behavior.
- Git tree equality or source blob equality was used only for exact matches. Divergent docs, code and skills were separately compared and marked superseded where current behavior or guidance is newer. No tip was cherry-picked.

## Actual tips

| Tip | Disposition | Basis |
|---|---|---|
| `01d24ac3b2` | integrated | Every changed tip blob matches HEAD. |
| `04cc540378` | integrated | Every changed tip blob matches HEAD. |
| `086c40c253` | superseded | workflow help remains; current CLI includes later commands and usage rules. |
| `08c67ad7ef` | superseded | kit-new init flow changed; legacy skill layout and CLAUDE shim retired. |
| `0ec6833722` | superseded | prompt screen gained opt-in structured outputs with local checks. |
| `14021f884a` | superseded | sourced prose JSON bounds evolved. |
| `19c559404c` | superseded | Historical WIP source/docs later revised; legacy instruction, plan and tracking edits obsolete. |
| `1ab02d3788` | superseded | Historical WIP source/docs later revised; legacy instruction, plan and tracking edits obsolete. |
| `23d6ce6a67` | superseded | commit protocol remains; constructor now rejects typed-nil transaction stores. |
| `2406857ed5` | superseded | prose CLI and schema remain; command surface and input validation evolved. |
| `26392123d1` | integrated | Changed source matches HEAD; documentation received later edits. |
| `2aec2dc1ba` | integrated | Changed source matches HEAD; documentation received later edits. |
| `2e67c9d83e` | superseded | Historical WIP source/docs later revised; legacy instruction, plan and tracking edits obsolete. |
| `3b15c4b8fe` | integrated | Changed source matches HEAD; documentation received later edits. |
| `3b9d9c4183` | superseded | lesson CLI and transfer behavior evolved. |
| `3f5ddcf91d` | superseded | Historical WIP source/docs later revised; legacy instruction, plan and tracking edits obsolete. |
| `47cdaa8cf9` | superseded | Fathom skills remain but instructions were later revised. |
| `4901fa7073` | superseded | kit-new source now uses later init behavior; historical snapshot retains obsolete instruction layout. |
| `5256950985` | integrated | Every changed tip blob matches HEAD. |
| `5c42bcff85` | superseded | kit-new source now uses later init behavior; historical snapshot retains obsolete instruction layout. |
| `6bd018dba2` | superseded | reuse planner now permits historical regrade/reprice alongside required rerun. |
| `75223ede2c` | superseded | lesson receipt checks remain; CLI usage and tests evolved. |
| `786bed6316` | superseded | sourced prose schema and pipeline remain with later UTF-8 and freshness hardening; kit-new evolved. |
| `8700372576` | superseded | runner validation behavior remains; producer mismatch contract comment was clarified. |
| `89fbe33acc` | superseded | catalog kit and CLI subsequently expanded. |
| `91f35f5871` | superseded | Historical WIP source/docs later revised; legacy instruction, plan and tracking edits obsolete. |
| `9d1782c28e` | superseded | eval-runner package documentation now describes newer benchmark behavior. |
| `9ea3c77ec2` | superseded | sourced prose and HITL gained stricter schema, UTF-8, freshness and typed-nil checks. |
| `a6946dbab3` | superseded | Historical WIP source/docs later revised; legacy instruction, plan and tracking edits obsolete. |
| `ade08f0b89` | empty | Tip tree equals first parent. |
| `b1ce097f02` | integrated | Changed source matches HEAD; documentation received later edits. |
| `b424a95e24` | superseded | staged read-manifest/linter draft was replaced by the worktree; HEAD retains read-manifest and later linter, stats and eval-runner hardening. |
| `b9e8b6c8c1` | superseded | Historical WIP source/docs later revised; legacy instruction, plan and tracking edits obsolete. |
| `c177f0d336` | superseded | native host implementation evolved after this tip. |
| `cedebaec2d` | integrated | Changed source matches HEAD; documentation received later edits. |
| `d4f2e7aeb5` | integrated | Changed source matches HEAD; documentation received later edits. |
| `da686f8152` | superseded | workflow CLI later expanded; historical WIP instruction layout is obsolete. |
| `db2085da35` | superseded | engineering strict controls remain; config now copies the required default to avoid pointer mutation. |
| `e25261cfeb` | superseded | native host implementation and tests evolved. |
| `e8b82ab592` | superseded | workflow and kit-new CLIs expanded; historical WIP instruction layout is obsolete. |
| `e9330a7a51` | integrated | Every changed tip blob matches HEAD. |
| `f866c2d139` | superseded | sourced prose remains; kit-new and llmclient docs evolved. |
| `f8a7e6f630` | superseded | Historical WIP source/docs later revised; legacy instruction, plan and tracking edits obsolete. |
| `f8ec57feb6` | integrated | Every changed tip blob matches HEAD. |
| `fbcef44fa7` | integrated | Every changed tip blob matches HEAD. |
| `fe9918916d` | superseded | benchmark timestamps now must round-trip through JSON. |

## Limits

- This inventory covers the 46 maximal unreachable commits and their first-parent deltas; the parent graph contains the remaining unreachable commits. It does not assert that a historical commit is desirable merely because its files exist in HEAD.
- No builds or tests were run for this read-only audit. All 117 unreachable commits and unattached objects must remain preserved by the separate backup before any cleanup.

## Independent coverage and preservation

The first inventory incorrectly called 13 graph roots "tips". Root verification
computed **46 tips = unreachable commits minus all parent IDs**, then independently
asserted every one of the 117 commits is reachable from those tips. The incorrect
13-count assertion stopped before creating recovery refs or changing source. A fresh
review produced exactly one disposition for each of the independently computed 46 IDs.
The reviewed closed/unmerged PRs #3/#4/#5/#11 correspond to retained quality/catalog
work above; seven other closed PRs are marked merged. No open PR or additional remote
branch exists. No old snapshot was cherry-picked or treated as current authority.

All **767** unreachable objects are now preserved, including objects without a commit:

- Verified Git bundle covering all 46 tips and the 117-commit graph.
- Verified object pack/index containing every unreachable commit/tree/blob/tag OID.
- SHA-256 checksums, exact tip inventory, original fsck output and per-tip dispositions.

Permanent private backup directory:
`/Volumes/m2ext/backups/agentic-kit/contributions/2026-10-23-command-assignment/`.
Directory mode is 0700; recovery files are 0600. Bundle, pack and checksums were
reverified after relocation. Temporary recovery refs were removed only after verified
preservation. No Git objects were pruned and no original source workspace was reset.

The 78 objects outside HEAD/recovery-commit closure contain 24 trees and 54 blobs.
All 54 blobs were inspected within a 1 MiB/blob and 8 MiB total budget: actual total
1,019,157 bytes, largest 121,025. Closed classifications: 23 source, 7 test,
23 documentation, 1 configuration. Twelve root trees attribute four blobs to older
CHANGELOG/TRACKING versions; 50 blobs have no recovered path. All 59 extracted exported
Go/test identifiers resolve in the current index, and operative document headings have
current homes. This identifies no missing behavior/test lead; it does not prove body
equivalence. The unmatched historical review and configuration remain unknown and
preserved. No raw historical configuration was copied into this repository or installed.
