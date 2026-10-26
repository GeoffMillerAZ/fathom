---
name: add-language
description: Use for any work on how deeply fathom understands a programming language or file format. That covers adding a dialect (tree-sitter tags.scm or a doc engine), pinning or re-pinning a grammar, building support for a format such as justfiles or systemd units, measuring or regrading a reading against an oracle (ctags, a SCIP indexer, the tool's own evaluation), explaining why a language sits below its target level (L1 to L4) or was refused (parse errors over the 5% line), choosing the next item from the language-breadth queue, and flipping an LB-Q plan row. Trigger whenever someone in the fathom repository asks to support, add, finish, fix, re-pin or measure a language or format, or asks why one is below target or refused, even if they never say "dialect" or "skill". Not for using fathom on another repository, fathom's MCP tools, or the daemon's memory.
---

# Adding or finishing a language

A level is proven, never typed: a committed reading under `docs/bench/readings/lang/<id>.json` measures fathom against an independent oracle, and the generated tracker in `docs/languages.md` follows from it. Your job is to move one queue item's reading to its target, or to record honestly why it cannot get there.

The full worker recipe is `docs/design/language-breadth.md` section 13. Read the step you are on, not the whole design. This skill tells you which tool to reach for at each step.

## 1. Pick the work

- `go run ./cmd/indexbench lang-next` names the next ready queue item (its `depends_on` rows are done).
- `go run ./cmd/indexbench lang-next --near-miss` lists refused or below-target items whose blocking metric sits just past its bar (for example a parse-error rate of 5.6% against the 5% line). Try these first: a newer grammar pin can flip them.
- `go run ./cmd/indexbench lang-gap -id <id>` explains a below-target item: the miss classes keeping it down, with counts, examples where the reading recorded locations, and the recipe step to apply. Pass `-fresh` to measure again instead of reading the committed file.

The queue is `docs/plans/language-breadth.queue.json`; each item's plan row is `LB-Q-<id>` in `docs/plans/language-breadth.md`.

## 2. The levels and what proves them

| Level | Means | Typical proof |
|---|---|---|
| L1 | every definition, right name and range | ctags (tier B) or the language's indexer |
| L2 | call edges by syntax | a tier A or C oracle |
| L3 | imports and file-resolved edges | the language's own resolution or an import dump |
| L4 | typed edges | a SCIP index from the maintained indexer (Go and CUE are native) |

Oracle tiers: A is the language's own tools (SCIP indexer, type checker, the tool's own evaluation); B is universal-ctags or a compiler syntax dump; C is an independent parser or a grammar's tag queries; D is a grep proxy and proves nothing beyond "tolerated". Every level also needs at most 0.1% faulting files, at least 1 MB/s per core and no file over 2 s. A field the oracle cannot report caps the level at good.

## 3. The loop

1. Grammar (tags-engine items): `go run ./cmd/indexbench lang-pin -id <id>` drafts the lock row; check the licence at the pinned commit; build with `internal/extractor/tsengine/grammars/build.sh`.
2. Dialect: manifest, 3-6 fixtures under 200 lines each, `tags.scm` (and `imports.scm` for L3). Iterate with `go run ./cmd/indexbench lang-probe -dialect <id> <fixture>` until every required kind is captured. Write the failing test first.
3. Measure: `task lang:loop -- <id>` runs probe, oracle, regrade and tracker for the item and prints one bounded summary: level met with grade, the delta against the committed reading, and the top mismatch classes. It never writes the committed reading unless you pass `WRITE=1`, and it exits non-zero when any level regressed. It never installs or fetches an oracle: when one is missing it says which. Declare every non-ctags runner with `ORACLE_CONFIG=<runners.json>` per recipe §13.4.1; skipped runners are incomplete evidence, not a complete calibration.
   If cloud throughput falls below 1 MB/CPU-s, run `indexbench lang-retime -reading <reading.json> -subject <name>=<dir> [-subject ...] [-runs 3] [-write]` on Apple silicon before citing a throughput refusal (design §1.9 T2).
4. Iterate on the top mismatch class, re-running the loop, until the reading meets the target or a written ceiling explains why it cannot.
5. Commit the reading with `WRITE=1 task lang:loop -- <id>`, then regenerate the tracker: `go run ./cmd/indexbench lang-tracker > docs/languages.md`. Never hand-edit the generated sections of `docs/languages.md`; `TestLanguagesTrackerIsGenerated` fails when you do.

## 3.1 Evidence checklist before claiming support or READY

Follow recipe §13.3.1, §13.4.0 and §13.5.1 for the full checks.

- [ ] Run recipe §13.3.1's exact before/after tracked-set manifest command. Record digests,
  counts, exclusions and reading totals in provenance/assessment. It approximates discovery;
  reconcile differences and inspect generated untracked/ignored inputs after builds/oracles.
- [ ] Keep holdout pins/readings outside canonical `calibration/corpus.d/` shards and
  aggregation. Freeze criteria before measurement; audit membership as well as pin timing.
- [ ] Probe truth independence: perturb extractor output and confirm the raw oracle-truth
  digest is unchanged, then change a known input fact and observe oracle truth change.
  Record commands/results.
- [ ] Show per-subject numerators, denominators, ratios and tier caps beside pooled figures.
- [ ] Compare the same metric definition, grammar blob, tool and measured set. Distinguish
  ERROR-only, MISSING-only and ERROR-or-MISSING file counts before claiming non-reproduction.
- [ ] Mutate changed oracle tier/scope/pairing logic and show its focused tests fail, then
  restore and show green. Report `Mutation:` with file, mutant, test command, FAIL line and
  green rerun; preserve these in provenance/assessment per §13.4.0. Reuse owning-lane tests.
- [ ] Exercise adversarial counts/fan-out/paths/nesting/compression where applicable; prove
  whole-unit refusal at cap + 1 per cap. Record `Caps:` with cap/input count, test file/name,
  command and refusal result in provenance/assessment. If touched tests skip under `-race`,
  name/run them once without it too; otherwise run the named tests with `-race` (§13.4.0).
- [ ] Check Reading-schema files in top-level `lang/`, `holdout/`, `tuned/`, `tuned/lf3/` per
  §13.5.1. Exclude different-schema `parity/`; frozen `baseline/` needs an owner E2-R1/D20
  scope ruling before regrading. Run the named top-level strict value round-trip and capture
  the specified sorted-JSON before/after diff for each touched nested reading. Preserve
  `required_kinds`; absent metrics are not thin evidence. Once #7 E2 tests land, use their
  recursive coverage and require E2-R2 canonical byte equality as well as value preservation.
- [ ] Before READY run `go run ./third_party/agentic-kit/cmd/release-notes -check`,
  `bash scripts/check-merge-debris.sh` and, for staged content, the same script `--staged`.
- [ ] Give actual runnable commands and results in `Verified:`, with needed flags/inputs
  and repo-relative paths to committed files, not scratch helpers. A prose list of checks
  is not verification; disclose skips.

## 4. Refusing honestly

Before a ceiling, complete the named-candidate checklist in recipe §13.0.1 (X-R1): record existing tools/forks/sibling features, pinned version, source, license, output, scope and exact reason each cannot grade this item. Distinguish untried, unavailable, out-of-scope and measured failure. Existing nonstandard resolvers may need a neighbor item, not a changed population. Keep the search receipt in an indexed assessment and give a trigger that is not already satisfied.

When no allowed grammar meets the at-most-5% ERROR-or-MISSING file gate, the item is refused, not forced: its `LB-Q-<id>` row takes a `[-]` disposition with the measurement and a trigger to retry (for example a newer grammar release). A refusal claims no level and stays disclosed. Do not lower a bar, trim a calibration corpus or hand-label results to pass. An unavailable oracle may instead defer the target or leave a measured partial level; follow recipe §14.2.

## 5. Proving the row

Require an actual successful `fathom verify --service fathom --plan docs/plans/language-breadth.md` result before a permitted controller claim. Verify does not flip the box itself; a brief forbidding worker box flips leaves it to the controller. Run the gate's language tests with `-race` and the tracker test before the handoff. Add one unique change-note fragment under `docs/changelog/unreleased/` per recipe step 24, not a direct `CHANGELOG.md` edit.

## 6. Many items at once

Each queue item is independent, so breadth work parallelizes well: one helper lane per item, each in its own checkout, following the `fathom-offload` skill for checkouts, checksums and returning results. Keep heavy oracle runs to one per machine at a time, and keep review and the gate on the controller.
